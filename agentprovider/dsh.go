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

package agentprovider

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

const (
	// dshRoute is the provider route Gateway owns in settings.yaml. Every other
	// route in the file, and every other section, is left alone.
	dshRoute = "casbin-gateway"
	// dshCredentialRef is the name settings.yaml refers the key by; the key
	// itself lives in the credentials document, which is what dsh reads first.
	dshCredentialRef = "CASBIN_GATEWAY_API_KEY"
	// dshProtocol is the wire format the route serves. The gateway answers
	// OpenAI chat completions under the base URL dsh is pointed at.
	dshProtocol = "openai-completions"
	// dshBuiltin is what dsh talks to with no route of its own: the DeepSeek
	// service it ships with.
	dshBuiltin = "DeepSeek"
)

// The settings Gateway overwrites, remembered under these names.
const (
	dshProviderKey   = "agent-default-model.provider"
	dshModelKey      = "agent-default-model.model"
	dshEffortKey     = "agent-default-model.reasoningEffort"
	dshCredentialKey = "credentials." + dshCredentialRef
)

var (
	dshRoutePath     = []string{"llm-pi-ai", "providers", dshRoute}
	dshProvidersPath = []string{"llm-pi-ai", "providers"}
	dshRefPath       = []string{"refs", dshCredentialRef}
)

// dshDefaultModelKeys are the fields of the default-model section Gateway
// owns, by the name each is remembered under.
var dshDefaultModelKeys = map[string]string{
	dshProviderKey: "provider",
	dshModelKey:    "model",
	dshEffortKey:   "reasoningEffort",
}

// errDshNoKey rejects a provider that forwards the caller's own credentials. A
// route dsh does not ship cannot hold a sign-in of its own, so the key written
// here is the only credential such a route ever sends.
var errDshNoKey = errors.New("DeepSeek Harness needs an API key, so it cannot use a provider that forwards the credentials of the caller")

// errDshNoModel rejects a route dsh could not serve: a provider it does not
// ship must name the models it answers for.
var errDshNoModel = errors.New("DeepSeek Harness needs a model name, so bind a provider that lists at least one model")

// dshRouteConfig is the provider route as settings.yaml spells it. The key is
// named rather than written: dsh resolves the reference on every request.
type dshRouteConfig struct {
	DisplayName string          `yaml:"displayName"`
	ApiKeyEnv   string          `yaml:"apiKeyEnv"`
	Api         string          `yaml:"api"`
	BaseURL     string          `yaml:"baseURL"`
	Models      []dshRouteModel `yaml:"models"`
}

type dshRouteModel struct {
	Id string `yaml:"id"`
}

type dshWriter struct{}

func init() {
	register(dshWriter{})
}

func (dshWriter) AgentId() string { return "dsh" }

func (dshWriter) Protocol() string { return "openai" }

func (w dshWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}
	settingsPath, credentialsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	settings, err := yamledit.Render(map[string]any{
		"llm-pi-ai":           map[string]any{"providers": map[string]any{dshRoute: w.route(endpoint)}},
		"agent-default-model": map[string]any{"provider": dshRoute, "model": endpoint.Model},
	})
	if err != nil {
		return nil, err
	}
	credentials, err := yamledit.Render(map[string]any{
		"refs": map[string]any{dshCredentialRef: maskSecret(endpoint.ApiKey)},
	})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: settingsPath, Format: "yaml", Preview: settings},
		{Path: credentialsPath, Format: "yaml", Preview: credentials},
	}, nil
}

func (w dshWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}
	settingsPath, credentialsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	settings, root, err := w.load(settingsPath)
	if err != nil {
		return nil, err
	}
	previous := map[string]string{}
	for key, field := range dshDefaultModelKeys {
		if value := yamledit.String(root, "agent-default-model", field); value != "" {
			previous[key] = value
		}
	}

	if err := yamledit.Set(root, w.route(endpoint), dshRoutePath...); err != nil {
		return nil, err
	}
	if err := yamledit.Set(root, dshRoute, "agent-default-model", "provider"); err != nil {
		return nil, err
	}
	if err := yamledit.Set(root, endpoint.Model, "agent-default-model", "model"); err != nil {
		return nil, err
	}
	// The route declares no reasoning levels, so an effort left over from
	// another provider would name one this model cannot take.
	yamledit.Delete(root, "agent-default-model", "reasoningEffort")

	credentials, refs, err := w.load(credentialsPath)
	if err != nil {
		return nil, err
	}
	if value := yamledit.String(refs, dshRefPath...); value != "" {
		previous[dshCredentialKey] = value
	}
	// The document declares its own version; only a file Gateway is creating
	// needs one written.
	if yamledit.Get(refs, "version") == nil {
		if err := yamledit.Set(refs, 1, "version"); err != nil {
			return nil, err
		}
	}
	if err := yamledit.Set(refs, endpoint.ApiKey, dshRefPath...); err != nil {
		return nil, err
	}

	if err := w.save(settingsPath, settings, credentialsPath, credentials); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w dshWriter) Restore(target Target, previous map[string]string) error {
	settingsPath, credentialsPath, err := w.paths(target)
	if err != nil {
		return err
	}

	settings, root, err := w.load(settingsPath)
	if err != nil {
		return err
	}
	yamledit.Delete(root, dshRoutePath...)
	if yamledit.IsEmpty(yamledit.Get(root, dshProvidersPath...)) {
		yamledit.Delete(root, dshProvidersPath...)
	}
	if yamledit.IsEmpty(yamledit.Get(root, "llm-pi-ai")) {
		yamledit.Delete(root, "llm-pi-ai")
	}
	for key, field := range dshDefaultModelKeys {
		if value, ok := previous[key]; ok {
			if err := yamledit.Set(root, value, "agent-default-model", field); err != nil {
				return err
			}
			continue
		}
		yamledit.Delete(root, "agent-default-model", field)
	}
	if yamledit.IsEmpty(yamledit.Get(root, "agent-default-model")) {
		yamledit.Delete(root, "agent-default-model")
	}

	credentials, refs, err := w.load(credentialsPath)
	if err != nil {
		return err
	}
	if value, ok := previous[dshCredentialKey]; ok {
		if err := yamledit.Set(refs, value, dshRefPath...); err != nil {
			return err
		}
	} else {
		yamledit.Delete(refs, dshRefPath...)
	}

	return w.save(settingsPath, settings, credentialsPath, credentials)
}

func (w dshWriter) Current(target Target) (string, error) {
	settingsPath, _, err := w.paths(target)
	if err != nil {
		return "", err
	}
	_, root, err := w.load(settingsPath)
	if err != nil {
		return "", err
	}

	selected := yamledit.String(root, "agent-default-model", "provider")
	if selected == "" {
		return "", nil
	}
	if baseUrl := yamledit.String(root, "llm-pi-ai", "providers", selected, "baseURL"); baseUrl != "" {
		return baseUrl, nil
	}
	// A route without a base URL is one dsh ships itself, which is still worth
	// naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

func (w dshWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous[dshModelKey], dshBuiltin)
	}

	settingsPath, _, err := w.paths(target)
	if err != nil {
		return dshBuiltin
	}
	_, root, err := w.load(settingsPath)
	if err != nil {
		return dshBuiltin
	}
	// A model beside Gateway's own route is Gateway's, not the one dsh would
	// pick for itself.
	if yamledit.String(root, "agent-default-model", "provider") == dshRoute {
		return dshBuiltin
	}
	return emptyAs(yamledit.String(root, "agent-default-model", "model"), dshBuiltin)
}

// check reports why dsh cannot be pointed at endpoint.
func (dshWriter) check(endpoint Endpoint) error {
	if endpoint.ApiKey == "" {
		return errDshNoKey
	}
	if endpoint.Model == "" {
		return errDshNoModel
	}
	return nil
}

// route is the llm-pi-ai profile Gateway writes. dsh ships no catalog for this
// route, so the profile carries the whole provider: protocol, endpoint, models.
func (dshWriter) route(endpoint Endpoint) dshRouteConfig {
	models := []dshRouteModel{}
	for _, model := range endpoint.catalog() {
		models = append(models, dshRouteModel{Id: model})
	}
	return dshRouteConfig{
		DisplayName: "Casbin Gateway",
		ApiKeyEnv:   dshCredentialRef,
		Api:         dshProtocol,
		BaseURL:     endpoint.BaseUrl,
		Models:      models,
	}
}

func (dshWriter) load(path string) (*yamledit.Document, *yaml.Node, error) {
	data, _, _, err := readFile(path)
	if err != nil {
		return nil, nil, err
	}
	document, err := yamledit.Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	root, err := document.Mapping()
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, root, nil
}

func (dshWriter) save(settingsPath string, settings *yamledit.Document,
	credentialsPath string, credentials *yamledit.Document) error {
	settingsData, err := settings.Bytes()
	if err != nil {
		return err
	}
	credentialsData, err := credentials.Bytes()
	if err != nil {
		return err
	}

	changes := &txn{}
	if err := changes.write(settingsPath, settingsData); err != nil {
		changes.abort()
		return err
	}
	if err := changes.write(credentialsPath, credentialsData); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

func (w dshWriter) paths(target Target) (string, string, error) {
	home, err := w.dshHome(target.Owner)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, "settings.yaml"), filepath.Join(home, ".credentials.yaml"), nil
}

// dshHome is the harness home the owner's dsh reads. DSH_HOME is meaningful
// only for the account Gateway runs as.
func (dshWriter) dshHome(owner string) (string, error) {
	if current, err := user.Current(); err == nil && sameAccountName(owner, current.Username) {
		if configured := strings.TrimSpace(os.Getenv("DSH_HOME")); configured != "" {
			if !filepath.IsAbs(configured) {
				return "", errors.New("DSH_HOME must be an absolute path")
			}
			return filepath.Clean(configured), nil
		}
	}
	home, err := agenthome.Resolve(owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dsh"), nil
}

func sameAccountName(left, right string) bool {
	bare := func(value string) string {
		value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
		return strings.ToLower(filepath.Base(value))
	}
	return left != "" && bare(left) == bare(right)
}
