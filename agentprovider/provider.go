// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package agentprovider writes the selected upstream provider into the
// configuration file each agent CLI reads on startup, in that CLI's own format.
package agentprovider

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/apache/casbin-gateway/agentenv"
)

// ErrNotSupported reports an agent whose configuration format Gateway cannot
// write yet. Those agents are still usable through the environment variables
// the UI shows.
var ErrNotSupported = errors.New("switching the provider of this agent is not supported yet")

// The two ways an agent reaches its provider.
const (
	// ModeGateway points the agent at the local proxy, so changing the bound
	// provider afterwards takes effect without touching a file again.
	ModeGateway = "gateway"
	// ModeDirect writes the provider's own base URL and key into the config,
	// which is what a switcher without a proxy does.
	ModeDirect = "direct"
)

// Target identifies one discovered agent installation.
type Target struct {
	AgentId string `json:"agentId"`
	Path    string `json:"path"`
	Owner   string `json:"owner"`
}

// Endpoint is the upstream one agent is switched to, already resolved: in
// gateway mode BaseUrl is the local proxy, in direct mode the provider's own.
type Endpoint struct {
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
	BaseUrl  string `json:"baseUrl"`
	ApiKey   string `json:"apiKey"`
	// Model is the one the agent starts on, the first the provider lists.
	Model string `json:"model"`
	// Models is every model the provider serves. An agent whose configuration
	// carries the catalog itself is written all of them, so its own picker can
	// switch between them without Gateway writing the file again.
	Models []string `json:"models"`
	Mode   string   `json:"mode"`
	// ServesResponsesApi reports whether BaseUrl answers on the OpenAI Responses
	// API, which is all Codex speaks. The gateway always does, since it
	// translates; a provider's own upstream usually stops at chat completions.
	ServesResponsesApi bool `json:"servesResponsesApi"`
	// ClientAuth reports whether the provider forwards the credentials the
	// agent itself sends, which is why there is no key to write into its
	// configuration: the sign-in it already has is the one being used.
	ClientAuth bool `json:"clientAuth"`
}

// File is one configuration file a switch writes, with the section it will
// contain afterwards.
type File struct {
	Path    string `json:"path"`
	Format  string `json:"format"`
	Preview string `json:"preview"`
}

// Status is what the UI shows beside an installation: whether Gateway owns its
// provider configuration, and which one it points at.
type Status struct {
	Supported bool `json:"supported"`
	// Protocol is the wire format this agent's client speaks, empty for an
	// agent Gateway does not know.
	Protocol string   `json:"protocol"`
	Applied  bool     `json:"applied"`
	Provider string   `json:"provider"`
	Mode     string   `json:"mode"`
	BaseUrl  string   `json:"baseUrl"`
	Time     string   `json:"time"`
	Files    []string `json:"files"`
	Detail   string   `json:"detail"`
	// Builtin is what the agent talks to on its own, with nothing bound: the
	// model its own configuration names, or the account it signs in to when it
	// names none.
	Builtin string `json:"builtin"`
	// Current is the endpoint the agent's files name right now, whichever tool
	// wrote them.
	Current string `json:"current"`
	// EnvConflicts are the variables set in the environment that the agent reads
	// before its configuration file, and so silently override what a switch
	// writes into that file.
	EnvConflicts []agentenv.Conflict `json:"envConflicts"`
}

type writer interface {
	AgentId() string
	// Protocol is the wire format this agent's client speaks.
	Protocol() string
	// Plan renders what Apply would write, without touching a file.
	Plan(Target, Endpoint) ([]File, error)
	// Apply writes the endpoint and returns the previous values of the keys it
	// owns, which is what Restore puts back.
	Apply(Target, Endpoint) (map[string]string, error)
	Restore(Target, map[string]string) error
	// Current is the base URL the files point at right now, empty when the
	// agent has no provider configured.
	Current(Target) (string, error)
	// Builtin names the model the agent uses without Gateway. previous is what
	// the files held before Gateway first wrote them, nil when it never did and
	// the files themselves are read instead.
	Builtin(Target, map[string]string) string
}

var (
	writers     = map[string]writer{}
	writerMutex sync.Mutex
)

func register(value writer) {
	writers[value.AgentId()] = value
}

// Plan is the preview shown before a switch is confirmed.
func Plan(target Target, endpoint Endpoint) ([]File, error) {
	value, err := writerFor(target, endpoint)
	if err != nil {
		return nil, err
	}
	return value.Plan(target, endpoint)
}

// Apply switches one installation to endpoint. The previous values of the keys
// it overwrites are saved first, so Restore can put the agent back on its own
// provider.
func Apply(target Target, endpoint Endpoint) error {
	value, err := writerFor(target, endpoint)
	if err != nil {
		return err
	}

	writerMutex.Lock()
	defer writerMutex.Unlock()

	files, err := value.Plan(target, endpoint)
	if err != nil {
		return err
	}

	// Only the first switch records what the agent looked like before Gateway
	// touched it; a later one would otherwise save Gateway's own values.
	saved, err := loadState(target)
	if err != nil {
		return err
	}

	previous, err := value.Apply(target, endpoint)
	if err != nil {
		return err
	}
	if saved != nil {
		previous = saved.Previous
	}

	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	return saveState(target, &state{
		AgentId:  target.AgentId,
		Provider: endpoint.Provider,
		Mode:     endpoint.Mode,
		BaseUrl:  endpoint.BaseUrl,
		Time:     nowString(),
		Files:    paths,
		Previous: previous,
	})
}

// Restore puts back the provider settings the agent had before the first Apply
// and forgets the installation. It is a no-op when Gateway never switched it.
func Restore(target Target) error {
	writerMutex.Lock()
	defer writerMutex.Unlock()

	saved, err := loadState(target)
	if err != nil {
		return err
	}

	value, ok := writers[target.AgentId]
	if !ok {
		// An installation Gateway never switched has nothing to put back,
		// whether or not its format is one Gateway can write.
		if saved == nil {
			return nil
		}
		return fmt.Errorf("%s: %w", target.AgentId, ErrNotSupported)
	}

	var previous map[string]string
	if saved != nil {
		previous = saved.Previous
	} else if !pointsAtGateway(value, target) {
		return nil
	}
	if err := value.Restore(target, previous); err != nil {
		return err
	}
	return clearState(target)
}

// StatusOf reports the provider state of one installation. A configuration file
// that cannot be read is reported as detail rather than as an error, so the
// agent list stays usable.
func StatusOf(target Target) Status {
	status := statusOf(target)
	status.EnvConflicts = []agentenv.Conflict{}
	// An agent Gateway cannot write is pointed at the gateway with these very
	// variables by hand, so only an agent with a writer is checked.
	if status.Supported {
		status.EnvConflicts = agentenv.Check(target.AgentId, target.Owner, status.BaseUrl)
	}
	return status
}

func statusOf(target Target) Status {
	value, ok := writers[target.AgentId]
	if !ok {
		return Status{
			Files:  []string{},
			Detail: "Gateway cannot write this agent's provider configuration yet",
		}
	}

	status := Status{Supported: true, Protocol: value.Protocol(), Files: []string{}}
	saved, err := loadState(target)
	if err != nil {
		status.Detail = err.Error()
		return status
	}

	if saved == nil {
		status.Builtin = value.Builtin(target, nil)
	} else {
		status.Builtin = value.Builtin(target, saved.Previous)
	}

	current, err := value.Current(target)
	if err != nil {
		status.Detail = err.Error()
		return status
	}
	status.Current = current

	if saved == nil {
		if current == "" {
			status.Detail = "This agent uses its own provider configuration"
		} else {
			status.Detail = "Configured outside Gateway: " + current
		}
		return status
	}

	status.Applied = true
	status.Provider = saved.Provider
	status.Mode = saved.Mode
	status.BaseUrl = saved.BaseUrl
	status.Time = saved.Time
	status.Files = saved.Files
	if current != saved.BaseUrl {
		status.Applied = false
		status.Detail = "Changed outside Gateway, the agent now points at " + emptyAs(current, "no provider")
	} else if saved.Mode == ModeGateway {
		status.Detail = "Routed through the local proxy, switching the provider needs no restart"
	} else {
		status.Detail = "Written directly into the agent configuration"
	}
	return status
}

// ProtocolOf is the wire format one agent's client speaks, empty for an agent
// Gateway has no writer for.
func ProtocolOf(agentId string) string {
	value, ok := writers[agentId]
	if !ok {
		return ""
	}
	return value.Protocol()
}

func writerFor(target Target, endpoint Endpoint) (writer, error) {
	if target.AgentId == "" {
		return nil, errors.New("agentId is required")
	}
	if endpoint.Provider == "" {
		return nil, errors.New("no provider is bound to this agent")
	}
	if endpoint.BaseUrl == "" {
		return nil, errors.New("the base URL of the bound provider is empty")
	}

	value, ok := writers[target.AgentId]
	if !ok {
		return nil, fmt.Errorf("%s: %w", target.AgentId, ErrNotSupported)
	}
	// Through the gateway the two need not speak the same API: the proxy
	// translates. Written into the agent config the provider is reached
	// directly, so there they do.
	if endpoint.Mode == ModeDirect && value.Protocol() != endpoint.Protocol {
		return nil, fmt.Errorf("%s speaks the %s API, but provider %s speaks %s: bind a provider that speaks %s, or route this agent through the gateway",
			target.AgentId, value.Protocol(), endpoint.Provider, endpoint.Protocol, value.Protocol())
	}
	return value, nil
}

// pointsAtGateway reports whether the agent configuration still names the local
// proxy. A state file can be lost while the file it wrote stays behind, and
// putting back only what state records would leave the agent calling a proxy
// with no provider behind it.
func pointsAtGateway(value writer, target Target) bool {
	current, err := value.Current(target)
	return err == nil && strings.Contains(current, "/v1/agents/")
}

// catalog is the models to write into an agent that carries its own list. The
// bound default comes first, and a duplicate is dropped so the agent's picker
// does not show one twice.
func (endpoint Endpoint) catalog() []string {
	models := []string{}
	seen := map[string]bool{}
	for _, model := range append([]string{endpoint.Model}, endpoint.Models...) {
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		models = append(models, model)
	}
	return models
}

func emptyAs(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// maskSecret is what a preview shows in place of a key: enough to recognize it,
// not enough to use it.
func maskSecret(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "***" + value[len(value)-4:]
}
