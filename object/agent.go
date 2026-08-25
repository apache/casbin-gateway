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

package object

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// ErrAgentNoProvider lets the proxy answer with a client error, not a gateway one.
var ErrAgentNoProvider = errors.New("no provider is bound to this agent")

// AgentOwner is the owner every agent row is stored under: an agent belongs to
// the host, and its proxy endpoint is reached without a session.
const AgentOwner = "admin"

// How an agent reaches its provider. The values are the ones agentprovider
// writes into the agent's own configuration file.
const (
	ModeGateway = "gateway"
	ModeDirect  = "direct"
)

// Agent is the Gateway-side configuration of one kind of AI agent. Installations
// are discovered by scanning the host, so this table holds only what is chosen.
type Agent struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	// Provider is the "owner/name" id of the bound provider, empty when unbound.
	Provider string `xorm:"varchar(200)" json:"provider"`
	// Fallbacks are the provider ids tried, in order, when Provider cannot answer.
	// They are JSON-serialized by xorm, hence the text column.
	Fallbacks []string `xorm:"mediumtext" json:"fallbacks"`
	// Mode is how the agent reaches its provider: ModeGateway routes it through
	// the local proxy, ModeDirect writes the provider's own endpoint into the
	// agent's configuration file.
	Mode string `xorm:"varchar(20)" json:"mode"`
}

func (agent *Agent) GetId() string {
	return fmt.Sprintf("%s/%s", agent.Owner, agent.Name)
}

func GetAgent(agentId string) (*Agent, error) {
	agent := &Agent{Owner: AgentOwner, Name: agentId}
	existed, err := ormer.Engine.Get(agent)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return agent, nil
}

// GetAgents returns every configured agent, keyed by agent id. Installations
// are discovered per host while the routing is stored per agent id, so the two
// are merged by the caller.
func GetAgents() (map[string]*Agent, error) {
	agents := []*Agent{}
	if err := ormer.Engine.Where("owner = ?", AgentOwner).Find(&agents); err != nil {
		return nil, err
	}

	result := map[string]*Agent{}
	for _, agent := range agents {
		if agent.Mode == "" {
			agent.Mode = ModeGateway
		}
		result[agent.Name] = agent
	}
	return result, nil
}

// SetAgentRouting stores where one agent's requests go: the bound provider, the
// providers to fall over to when it cannot answer, and how the agent reaches
// them. Every provider is resolved here so a typo fails at the form rather than
// on the next relayed request.
func SetAgentRouting(agentId string, providerId string, fallbacks []string, mode string) error {
	if agentId == "" {
		return errors.New("the agent id is empty")
	}
	if mode == "" {
		mode = ModeGateway
	}
	if mode != ModeGateway && mode != ModeDirect {
		return fmt.Errorf("invalid agent mode: %s", mode)
	}

	fallbacks = normalizeFallbacks(providerId, fallbacks)
	for _, id := range append([]string{providerId}, fallbacks...) {
		if id == "" {
			continue
		}
		provider, err := getProviderById(id)
		if err != nil {
			return err
		}
		if provider == nil {
			return fmt.Errorf("the provider does not exist: %s", id)
		}
	}

	stored, err := GetAgent(agentId)
	if err != nil {
		return err
	}

	now := util.GetCurrentTime()
	if stored == nil {
		_, err = ormer.Engine.Insert(&Agent{
			Owner:       AgentOwner,
			Name:        agentId,
			CreatedTime: now,
			UpdatedTime: now,
			Provider:    providerId,
			Fallbacks:   fallbacks,
			Mode:        mode,
		})
		return err
	}

	// Cols() is what writes an empty provider: xorm skips zero values otherwise.
	_, err = ormer.Engine.ID(core.PK{AgentOwner, agentId}).
		Cols("provider", "fallbacks", "mode", "updated_time").
		Update(&Agent{Provider: providerId, Fallbacks: fallbacks, Mode: mode, UpdatedTime: now})
	return err
}

// SetAgentProvider binds an agent to a provider, or unbinds it when providerId is
// empty, leaving its fallbacks and mode as they are.
func SetAgentProvider(agentId string, providerId string) error {
	stored, err := GetAgent(agentId)
	if err != nil {
		return err
	}
	if stored == nil {
		return SetAgentRouting(agentId, providerId, nil, "")
	}
	return SetAgentRouting(agentId, providerId, stored.Fallbacks, stored.Mode)
}

// normalizeFallbacks drops empty entries, duplicates and the primary provider,
// so the chain never tries the same upstream twice.
func normalizeFallbacks(providerId string, fallbacks []string) []string {
	result := []string{}
	seen := map[string]bool{providerId: true, "": true}
	for _, id := range fallbacks {
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// GetProvidersByAgent is the chain one agent's requests are tried against: the
// bound provider first, then its fallbacks. A missing or disabled entry is
// skipped rather than failing the request, which is what makes the fallbacks
// worth configuring; a chain with nothing left in it reports why.
func GetProvidersByAgent(agentId string) ([]*Provider, error) {
	agent, err := GetAgent(agentId)
	if err != nil {
		return nil, err
	}
	if agent == nil || agent.Provider == "" {
		return nil, errNoProvider(agentId)
	}

	providers := []*Provider{}
	skipped := ""
	for _, id := range append([]string{agent.Provider}, agent.Fallbacks...) {
		provider, err := getProviderById(id)
		if err != nil {
			skipped = err.Error()
			continue
		}
		if provider == nil {
			skipped = fmt.Sprintf("the provider bound to agent %s no longer exists: %s", agentId, id)
			continue
		}
		if provider.Status != "enabled" {
			skipped = fmt.Sprintf("the provider bound to agent %s is disabled: %s", agentId, id)
			continue
		}
		providers = append(providers, provider)
	}

	if len(providers) == 0 {
		if skipped == "" {
			return nil, errNoProvider(agentId)
		}
		return nil, errors.New(skipped)
	}
	return providers, nil
}

// errNoProvider says what an unbound agent should do instead of calling the
// proxy: an agent left on its own built-in model has no reason to reach Gateway
// at all, so its configuration is what needs putting back.
func errNoProvider(agentId string) error {
	return fmt.Errorf("%w: %s. It is set to its own built-in model, so bind a provider, or restore its configuration so it stops calling Gateway",
		ErrAgentNoProvider, agentId)
}

// GetProviderByAgent resolves the first provider of an agent's chain.
func GetProviderByAgent(agentId string) (*Provider, error) {
	providers, err := GetProvidersByAgent(agentId)
	if err != nil {
		return nil, err
	}
	return providers[0], nil
}

// getProviderById is GetProvider() without its panic on a malformed id.
func getProviderById(providerId string) (*Provider, error) {
	tokens := strings.Split(providerId, "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return nil, fmt.Errorf("invalid provider ID: %s", providerId)
	}
	return getProvider(tokens[0], tokens[1])
}
