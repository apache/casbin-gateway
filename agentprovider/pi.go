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
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
)

const (
	// piProvider is the provider entry Gateway owns in models.json, and the
	// name settings.json starts pi on. Every other provider is left alone.
	piProvider = "casbin-gateway"
	// piApi is the wire format pi talks to the entry with, which is the one an
	// endpoint pi does not ship needs.
	piApi = "openai-completions"
	// piLocalKey stands in for a key the endpoint does not have. pi hides a
	// model whose provider has no credential, while the gateway takes any
	// credential from this host.
	piLocalKey = "casbin-gateway"
)

// The settings keys Gateway owns. pi starts on the pair, and falls back to its
// own picker when either is missing.
const (
	piProviderKey = "defaultProvider"
	piModelKey    = "defaultModel"
)

var piKeys = []string{piProviderKey, piModelKey}

// errPiNoModel rejects a provider entry pi could not start on: an endpoint it
// does not know has no catalog of its own, and settings.json needs a model id
// beside the provider name.
var errPiNoModel = errors.New("pi needs a model name, so bind a provider that lists at least one model")

type piWriter struct{}

func init() {
	register(piWriter{})
}

func (piWriter) AgentId() string { return "pi" }

func (piWriter) Protocol() string { return "openai" }

func (w piWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errPiNoModel
	}
	modelsPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, piLocalKey))
	models, err := encodeJSON(map[string]any{
		"providers": map[string]any{piProvider: w.entry(preview)},
	})
	if err != nil {
		return nil, err
	}
	settings, err := encodeJSON(w.settings(endpoint))
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: modelsPath, Format: "json", Preview: string(models)},
		{Path: settingsPath, Format: "json", Preview: string(settings)},
	}, nil
}

func (w piWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errPiNoModel
	}
	modelsPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	models, _, err := readJSON(modelsPath)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	// A selection Gateway already made is not what a restore should put back.
	if stringAt(settings, piProviderKey) != piProvider {
		for _, key := range piKeys {
			if value := stringAt(settings, key); value != "" {
				previous[key] = value
			}
		}
	}

	ensureNested(models, "providers")[piProvider] = w.entry(endpoint)
	for key, value := range w.settings(endpoint) {
		settings[key] = value
	}

	changes := &txn{}
	if err := w.stageModels(changes, modelsPath, models); err != nil {
		changes.abort()
		return nil, err
	}
	if err := w.stage(changes, settingsPath, settings); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w piWriter) Restore(target Target, previous map[string]string) error {
	modelsPath, settingsPath, err := w.paths(target)
	if err != nil {
		return err
	}
	models, _, err := readJSON(modelsPath)
	if err != nil {
		return err
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return err
	}

	if providers := objectAt(models, "providers"); providers != nil {
		delete(providers, piProvider)
	}
	for _, key := range piKeys {
		if value, ok := previous[key]; ok {
			settings[key] = value
		} else {
			delete(settings, key)
		}
	}

	changes := &txn{}
	if err := w.stageModels(changes, modelsPath, models); err != nil {
		changes.abort()
		return err
	}
	if err := w.stage(changes, settingsPath, settings); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

func (w piWriter) Current(target Target) (string, error) {
	models, settings, err := w.read(target)
	if err != nil {
		return "", err
	}

	selected := stringAt(settings, piProviderKey)
	if selected == "" {
		return "", nil
	}
	if baseUrl := stringAt(nestedObject(models, "providers", selected), "baseUrl"); baseUrl != "" {
		return baseUrl, nil
	}
	// A provider without a base URL is one pi ships itself, which is still
	// worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// Builtin is the model pi starts on without Gateway. pi signs in to a provider
// rather than shipping one, so settings that name none have no name to show.
func (w piWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return previous[piModelKey]
	}

	_, settings, err := w.read(target)
	if err != nil || stringAt(settings, piProviderKey) == piProvider {
		return ""
	}
	return stringAt(settings, piModelKey)
}

// entry is the provider as models.json stores it, with the whole catalog under
// it so pi's own picker can switch models without Gateway writing the file
// again.
func (piWriter) entry(endpoint Endpoint) map[string]any {
	models := []any{}
	for _, model := range endpoint.catalog() {
		models = append(models, map[string]any{"id": model, "name": model})
	}
	return map[string]any{
		"baseUrl": endpoint.BaseUrl,
		"api":     piApi,
		"apiKey":  piValue(emptyAs(endpoint.ApiKey, piLocalKey)),
		// The gateway fronts providers that reject the developer role, and
		// every one of them takes a system message instead.
		"compat": map[string]any{"supportsDeveloperRole": false},
		"models": models,
	}
}

func (piWriter) settings(endpoint Endpoint) map[string]any {
	return map[string]any{
		piProviderKey: piProvider,
		piModelKey:    endpoint.Model,
	}
}

// piValue escapes a literal for the fields pi resolves before use: a "$" would
// name an environment variable, and a leading "!" would run the whole value as
// a command.
func piValue(value string) string {
	value = strings.ReplaceAll(value, "$", "$$")
	if strings.HasPrefix(value, "!") {
		return "$!" + value[1:]
	}
	return value
}

// stageModels queues models.json, removing it once Gateway's provider was the
// only thing in it: an empty file is not what pi had before.
func (w piWriter) stageModels(changes *txn, path string, models map[string]any) error {
	pruneEmpty(models, "providers")
	return w.stage(changes, path, models)
}

func (piWriter) stage(changes *txn, path string, config map[string]any) error {
	if len(config) == 0 {
		return removeFile(path)
	}
	data, err := encodeJSON(config)
	if err != nil {
		return err
	}
	return changes.write(path, data)
}

func (w piWriter) read(target Target) (map[string]any, map[string]any, error) {
	modelsPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, nil, err
	}
	models, _, err := readJSON(modelsPath)
	if err != nil {
		return nil, nil, err
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return nil, nil, err
	}
	return models, settings, nil
}

// paths are the two files a switch writes, both in pi's own agent directory.
func (piWriter) paths(target Target) (string, string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, ".pi", "agent")
	return filepath.Join(dir, "models.json"), filepath.Join(dir, "settings.json"), nil
}
