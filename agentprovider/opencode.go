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
	// opencodeProvider is the provider entry Gateway owns in opencode.json.
	// Every other provider in the file is left alone.
	opencodeProvider = "casbin-gateway"
	// opencodeNpm is the AI SDK package opencode loads for an endpoint that is
	// not one of the services it ships, which is what the gateway is.
	opencodeNpm = "@ai-sdk/openai-compatible"
)

// The root keys Gateway owns, remembered under these names.
const (
	opencodeModelKey      = "model"
	opencodeSmallModelKey = "small_model"
)

// errOpencodeNoKey rejects a provider that forwards the caller's own
// credentials: the key written here is the only one this provider entry sends.
var errOpencodeNoKey = errors.New("opencode needs an API key, so it cannot use a provider that forwards the credentials of the caller")

// errOpencodeNoModel rejects a provider entry opencode could not use: an
// endpoint it does not know has no model catalog of its own.
var errOpencodeNoModel = errors.New("opencode needs a model name, so bind a provider that lists at least one model")

type opencodeWriter struct {
	id string
}

func init() {
	register(opencodeWriter{id: "opencode"})
	// The desktop app drives the same agent, and with it the same config file.
	register(opencodeWriter{id: "opencode-desktop"})
}

func (w opencodeWriter) AgentId() string { return w.id }

func (opencodeWriter) Protocol() string { return "openai" }

func (w opencodeWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
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
		"provider": map[string]any{opencodeProvider: w.provider(preview)},
		"model":    w.model(endpoint),
	})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "json", Preview: string(data)}}, nil
}

func (w opencodeWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
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
	for _, key := range []string{opencodeModelKey, opencodeSmallModelKey} {
		if value := stringAt(config, key); value != "" {
			previous[key] = value
		}
	}

	providers := objectAt(config, "provider")
	if providers == nil {
		providers = map[string]any{}
	}
	providers[opencodeProvider] = w.provider(endpoint)
	config["provider"] = providers
	config[opencodeModelKey] = w.model(endpoint)
	// Without one, opencode uses the model above for its own small requests
	// too; a leftover selection would send them to another provider.
	delete(config, opencodeSmallModelKey)

	return previous, w.save(path, config)
}

func (w opencodeWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	config, err := w.load(path)
	if err != nil {
		return err
	}

	if providers := objectAt(config, "provider"); providers != nil {
		delete(providers, opencodeProvider)
		if len(providers) == 0 {
			delete(config, "provider")
		}
	}
	for _, key := range []string{opencodeModelKey, opencodeSmallModelKey} {
		if value, ok := previous[key]; ok {
			config[key] = value
		} else {
			delete(config, key)
		}
	}
	return w.save(path, config)
}

func (w opencodeWriter) Current(target Target) (string, error) {
	path, err := w.configPath(target)
	if err != nil {
		return "", err
	}
	config, err := w.load(path)
	if err != nil {
		return "", err
	}

	selected, _, _ := strings.Cut(stringAt(config, opencodeModelKey), "/")
	if selected == "" {
		return "", nil
	}
	provider := objectAt(objectAt(config, "provider"), selected)
	if baseUrl := stringAt(objectAt(provider, "options"), "baseURL"); baseUrl != "" {
		return baseUrl, nil
	}
	// A provider without a base URL is one opencode knows itself, which is
	// still worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// Builtin is the model opencode selects for itself. opencode ships no provider
// of its own, so a file that selects none has no name to show.
func (w opencodeWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return previous[opencodeModelKey]
	}

	path, err := w.configPath(target)
	if err != nil {
		return ""
	}
	config, err := w.load(path)
	if err != nil {
		return ""
	}
	model := stringAt(config, opencodeModelKey)
	if strings.HasPrefix(model, opencodeProvider+"/") {
		return ""
	}
	return model
}

// check reports why opencode cannot be pointed at endpoint.
func (opencodeWriter) check(endpoint Endpoint) error {
	if endpoint.ApiKey == "" {
		return errOpencodeNoKey
	}
	if endpoint.Model == "" {
		return errOpencodeNoModel
	}
	return nil
}

// provider is the entry opencode loads the endpoint through. opencode ships no
// catalog for a provider it does not know, so the entry carries the whole of
// it: the SDK package, the endpoint, the key and the models.
func (opencodeWriter) provider(endpoint Endpoint) map[string]any {
	return map[string]any{
		"name": "Casbin Gateway",
		"npm":  opencodeNpm,
		"options": map[string]any{
			"baseURL": endpoint.BaseUrl,
			"apiKey":  endpoint.ApiKey,
		},
		"models": map[string]any{endpoint.Model: map[string]any{"name": endpoint.Model}},
	}
}

// model is how opencode names one model: by the provider it is served by.
func (opencodeWriter) model(endpoint Endpoint) string {
	return opencodeProvider + "/" + endpoint.Model
}

// load reads the config file. opencode accepts comments and trailing commas in
// it, so a file that has them is read rather than refused; saving writes plain
// JSON, which is what editing the file this way costs.
func (opencodeWriter) load(path string) (map[string]any, error) {
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

func (opencodeWriter) save(path string, config map[string]any) error {
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

// configPath is the file opencode reads its settings from, which is the same
// one its MCP servers are listed in.
func (w opencodeWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	path, ok := agentconfig.McpConfigPath(w.id, home)
	if !ok {
		return "", fmt.Errorf("no configuration layout for %s", w.id)
	}
	return path, nil
}
