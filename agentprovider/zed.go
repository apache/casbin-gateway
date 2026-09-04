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
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/internal/jsonc"
)

const (
	// zedProvider is the OpenAI-compatible provider Gateway owns under
	// language_models. Every other provider in the file is left alone.
	zedProvider = "casbin-gateway"
	// zedDefaultContext is the window a model is assumed to have when the
	// endpoint says nothing. Zed requires a number per model.
	zedDefaultContext = 128000
	// zedDefaultOutput is the matching output cap.
	zedDefaultOutput = 16384
)

// zedSelectedKey is the agent's default model, remembered under this name.
const zedSelectedKey = "agent.default_model"

// errZedNoModel rejects a provider Zed could not use: a provider entry carries
// its own model list, and an endpoint Zed does not know has none of its own.
var errZedNoModel = errors.New("Zed needs a model name, so bind a provider that lists at least one model")

// zedWriter owns the endpoint and the model list, not the credential: Zed keeps
// a key in the system keychain, and otherwise reads it from the provider id
// upper-snake-cased — CASBIN_GATEWAY_API_KEY. Neither is a store Gateway
// writes, so the key is entered once in Zed itself. Any value does: the gateway
// takes any credential from this host.
type zedWriter struct{}

func init() {
	register(zedWriter{})
}

func (zedWriter) AgentId() string { return "zed" }

func (zedWriter) Protocol() string { return "openai" }

func (w zedWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errZedNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	data, err := encodeJSON(map[string]any{
		"language_models": map[string]any{
			"openai_compatible": map[string]any{zedProvider: w.provider(endpoint)},
		},
		"agent": map[string]any{"default_model": w.defaultModel(endpoint)},
	})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "json", Preview: string(data)}}, nil
}

func (w zedWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errZedNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	settings, err := w.load(path)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	if selected := w.selectedOf(settings); selected != "" {
		previous[zedSelectedKey] = selected
	}

	ensureNested(settings, "language_models", "openai_compatible")[zedProvider] = w.provider(endpoint)
	ensureNested(settings, "agent")["default_model"] = w.defaultModel(endpoint)

	return previous, w.save(path, settings)
}

func (w zedWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	settings, err := w.load(path)
	if err != nil {
		return err
	}

	if providers := nestedObject(settings, "language_models", "openai_compatible"); providers != nil {
		delete(providers, zedProvider)
	}
	pruneEmpty(settings, "language_models", "openai_compatible")
	pruneEmpty(settings, "language_models")

	if selected, ok := previous[zedSelectedKey]; ok {
		provider, model, _ := strings.Cut(selected, "/")
		ensureNested(settings, "agent")["default_model"] = map[string]any{"provider": provider, "model": model}
	} else if agent := objectAt(settings, "agent"); agent != nil {
		delete(agent, "default_model")
	}
	pruneEmpty(settings, "agent")

	if len(settings) == 0 {
		return removeFile(path)
	}
	return w.save(path, settings)
}

// Current is the endpoint behind the provider Zed's agent starts on.
func (w zedWriter) Current(target Target) (string, error) {
	settings, err := w.settingsOf(target)
	if err != nil {
		return "", err
	}
	provider, _, _ := strings.Cut(w.selectedOf(settings), "/")
	if provider == "" {
		return "", nil
	}
	entry := nestedObject(settings, "language_models", "openai_compatible", provider)
	if url := stringAt(entry, "api_url"); url != "" {
		return url, nil
	}
	// A provider with no endpoint of its own is one Zed ships, which is still
	// worth naming: it is not the endpoint Gateway wrote.
	return provider, nil
}

// Builtin is the model Zed's agent starts on without Gateway.
func (w zedWriter) Builtin(target Target, previous map[string]string) string {
	selected := ""
	if previous != nil {
		selected = previous[zedSelectedKey]
	} else {
		settings, err := w.settingsOf(target)
		if err != nil {
			return ""
		}
		if selected = w.selectedOf(settings); strings.HasPrefix(selected, zedProvider+"/") {
			return ""
		}
	}
	_, model, _ := strings.Cut(selected, "/")
	return model
}

// selectedOf is the agent's default model as "provider/model", empty when the
// file names none.
func (zedWriter) selectedOf(settings map[string]any) string {
	selected := nestedObject(settings, "agent", "default_model")
	provider, model := stringAt(selected, "provider"), stringAt(selected, "model")
	if provider == "" || model == "" {
		return ""
	}
	return provider + "/" + model
}

// provider is the entry Zed loads the endpoint through. Zed ships no catalog
// for an endpoint it does not know, so the entry carries every model the
// gateway serves and its own picker switches between them.
func (zedWriter) provider(endpoint Endpoint) map[string]any {
	models := []any{}
	for _, model := range endpoint.catalog() {
		models = append(models, map[string]any{
			"name":              model,
			"max_tokens":        zedDefaultContext,
			"max_output_tokens": zedDefaultOutput,
		})
	}
	return map[string]any{"api_url": endpoint.BaseUrl, "available_models": models}
}

func (zedWriter) defaultModel(endpoint Endpoint) map[string]any {
	return map[string]any{"provider": zedProvider, "model": endpoint.Model}
}

func (w zedWriter) settingsOf(target Target) (map[string]any, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	return w.load(path)
}

// load reads the settings file. Zed accepts comments and trailing commas in it,
// so a file that has them is read rather than refused; saving writes plain
// JSON, which is what editing the file this way costs.
func (zedWriter) load(path string) (map[string]any, error) {
	data, _, _, err := readFile(path)
	if err != nil {
		return nil, err
	}

	settings := map[string]any{}
	if strings.TrimSpace(string(data)) == "" {
		return settings, nil
	}
	if err := json.Unmarshal(jsonc.Strip(data), &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if settings == nil {
		return nil, fmt.Errorf("parse %s: the root must be a JSON object", path)
	}
	return settings, nil
}

func (zedWriter) save(path string, settings map[string]any) error {
	data, err := encodeJSON(settings)
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

// configPath is the settings file Zed reads. It follows XDG on Unix and sits
// under the roaming profile on Windows.
func (zedWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", "Zed", "settings.json"), nil
	}
	return filepath.Join(home, ".config", "zed", "settings.json"), nil
}
