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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/internal/jsonc"
)

const (
	// openclawProvider is the provider entry Gateway owns in openclaw.json.
	// Every other provider in the file is left alone.
	openclawProvider = "casbin-gateway"
	// openclawApi is the request adapter OpenClaw drives the endpoint with. The
	// gateway answers OpenAI chat completions under the base URL written below.
	openclawApi = "openai-completions"
)

// openclawModelKey is the setting Gateway overwrites, remembered under this
// name. OpenClaw spells it either as a model reference or as an object with a
// primary and its fallbacks, so whatever was there is kept as JSON.
const openclawModelKey = "agents.defaults.model"

// errOpenclawNoKey rejects a provider that forwards the caller's own
// credentials: a provider OpenClaw does not ship holds no sign-in of its own,
// so the key written here is the only one it ever sends.
var errOpenclawNoKey = errors.New("OpenClaw needs an API key, so it cannot use a provider that forwards the credentials of the caller")

// errOpenclawNoModel rejects a provider entry OpenClaw could not use: an
// endpoint it does not ship has no model catalog of its own.
var errOpenclawNoModel = errors.New("OpenClaw needs a model name, so bind a provider that lists at least one model")

type openclawWriter struct{}

func init() {
	register(openclawWriter{})
}

func (openclawWriter) AgentId() string { return "openclaw" }

func (openclawWriter) Protocol() string { return "openai" }

func (w openclawWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(endpoint.ApiKey)
	data, err := encodeJSON(map[string]any{
		"models": map[string]any{"providers": map[string]any{openclawProvider: w.provider(preview)}},
		"agents": map[string]any{"defaults": map[string]any{"model": map[string]any{"primary": w.ref(endpoint.Model)}}},
	})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "json", Preview: string(data)}}, nil
}

func (w openclawWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	config, err := w.load(path)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	if defaults := nestedObject(config, "agents", "defaults"); defaults != nil {
		if value, ok := defaults["model"]; ok {
			text, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			previous[openclawModelKey] = string(text)
		}
	}

	ensureNested(config, "models", "providers")[openclawProvider] = w.provider(endpoint)
	// Writing the primary alone leaves the fallbacks the file names, which are
	// the models OpenClaw drops to when this endpoint fails.
	ensureNested(config, "agents", "defaults", "model")["primary"] = w.ref(endpoint.Model)
	// An allowlist that is set rejects every model outside it, this one
	// included. A file with none allows them all and gets none written.
	if allowed := nestedObject(config, "agents", "defaults", "models"); allowed != nil {
		for _, model := range endpoint.catalog() {
			allowed[w.ref(model)] = map[string]any{}
		}
	}

	return previous, w.save(path, config)
}

func (w openclawWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	config, err := w.load(path)
	if err != nil {
		return err
	}

	if providers := nestedObject(config, "models", "providers"); providers != nil {
		delete(providers, openclawProvider)
	}
	pruneEmpty(config, "models", "providers")

	// The allowlist keeps what it held: only the references Gateway added are
	// taken back out, and one it never had is not created here.
	if allowed := nestedObject(config, "agents", "defaults", "models"); allowed != nil {
		for ref := range allowed {
			if strings.HasPrefix(ref, openclawProvider+"/") {
				delete(allowed, ref)
			}
		}
	}

	if text, ok := previous[openclawModelKey]; ok {
		var value any
		if err := json.Unmarshal([]byte(text), &value); err != nil {
			return fmt.Errorf("restore %s: %w", openclawModelKey, err)
		}
		ensureNested(config, "agents", "defaults")["model"] = value
	} else if defaults := nestedObject(config, "agents", "defaults"); defaults != nil {
		delete(defaults, "model")
	}
	pruneEmpty(config, "agents", "defaults")

	return w.save(path, config)
}

func (w openclawWriter) Current(target Target) (string, error) {
	path, err := w.configPath(target)
	if err != nil {
		return "", err
	}
	config, err := w.load(path)
	if err != nil {
		return "", err
	}

	selected, _, _ := strings.Cut(w.selected(config), "/")
	if selected == "" {
		return "", nil
	}
	if baseUrl := stringAt(nestedObject(config, "models", "providers", selected), "baseUrl"); baseUrl != "" {
		return baseUrl, nil
	}
	// A provider without a base URL is one OpenClaw ships itself, which is
	// still worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// Builtin is the model OpenClaw runs on its own. It ships a catalog but no
// account, so a file that selects nothing has no name to show.
func (w openclawWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		var value any
		if err := json.Unmarshal([]byte(previous[openclawModelKey]), &value); err != nil {
			return ""
		}
		return openclawRef(value)
	}

	path, err := w.configPath(target)
	if err != nil {
		return ""
	}
	config, err := w.load(path)
	if err != nil {
		return ""
	}
	model := w.selected(config)
	if strings.HasPrefix(model, openclawProvider+"/") {
		return ""
	}
	return model
}

// check reports why OpenClaw cannot be pointed at endpoint.
func (openclawWriter) check(endpoint Endpoint) error {
	if endpoint.ApiKey == "" {
		return errOpenclawNoKey
	}
	if endpoint.Model == "" {
		return errOpenclawNoModel
	}
	return nil
}

// provider is the entry OpenClaw reaches the endpoint through. It ships no
// catalog for a provider it does not know, so the entry carries the whole of
// it: the adapter, the endpoint, the key and the models.
func (openclawWriter) provider(endpoint Endpoint) map[string]any {
	models := []any{}
	for _, model := range endpoint.catalog() {
		models = append(models, map[string]any{"id": model, "name": model})
	}
	return map[string]any{
		"baseUrl": endpoint.BaseUrl,
		"apiKey":  endpoint.ApiKey,
		"api":     openclawApi,
		"models":  models,
	}
}

// ref is how OpenClaw names one model: by the provider serving it.
func (openclawWriter) ref(model string) string {
	return openclawProvider + "/" + model
}

// selected is the model reference the file names right now.
func (openclawWriter) selected(config map[string]any) string {
	defaults := nestedObject(config, "agents", "defaults")
	if defaults == nil {
		return ""
	}
	return openclawRef(defaults["model"])
}

// openclawRef reads a model reference out of either spelling of the setting:
// the reference on its own, or the object naming it as the primary.
func openclawRef(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		return stringAt(typed, "primary")
	}
	return ""
}

// load reads the config file. OpenClaw accepts comments and trailing commas in
// it, so a file that has them is read rather than refused; saving writes plain
// JSON, which is what editing the file this way costs.
func (openclawWriter) load(path string) (map[string]any, error) {
	data, _, _, err := readFile(path)
	if err != nil {
		return nil, err
	}

	config := map[string]any{}
	if strings.TrimSpace(string(data)) == "" {
		return config, nil
	}
	if err := json.Unmarshal(jsonc.Strip(data), &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if config == nil {
		return nil, fmt.Errorf("parse %s: the root must be a JSON object", path)
	}
	return config, nil
}

func (openclawWriter) save(path string, config map[string]any) error {
	data, err := encodeJSON(config)
	if err != nil {
		return err
	}

	changes := &txn{}
	if err := changes.write(path, data); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

// configPath is the file OpenClaw reads its settings from, which is the same
// one its MCP servers are listed in.
func (w openclawWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	path, ok := agentconfig.McpConfigPath(w.AgentId(), home)
	if !ok {
		return "", fmt.Errorf("no configuration layout for %s", w.AgentId())
	}
	return path, nil
}
