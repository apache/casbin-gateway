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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/agenthome"
)

const (
	// clineProvider is the provider entry Gateway owns in providers.json. Every
	// other provider in the file is left alone.
	clineProvider = "casbin-gateway"
	// clineProviderName is what Cline's own picker shows the entry as.
	clineProviderName = "Casbin Gateway"
	// clineClient and clineProtocol pick the OpenAI-compatible path through
	// Cline's runtime, which is what an endpoint it does not ship needs.
	clineClient   = "openai-compatible"
	clineProtocol = "openai-chat"
	// clineSelectedKey is the root key naming the provider Cline starts on, and
	// how its previous value is remembered.
	clineSelectedKey = "lastUsedProvider"
	// clineLocalKey stands in for a key the endpoint does not have. The
	// OpenAI-compatible client sends one either way, and the gateway takes any
	// credential from this host.
	clineLocalKey = "casbin-gateway"
	// clineFileVersion is the schema version both settings files carry. Cline
	// refuses a file whose version it does not know.
	clineFileVersion = 1
)

// errClineNoModel rejects a provider entry Cline could not use: an endpoint it
// does not know has no catalog of its own, and it needs a name to start on.
var errClineNoModel = errors.New("Cline needs a model name, so bind a provider that lists at least one model")

type clineWriter struct{}

func init() {
	register(clineWriter{})
}

func (clineWriter) AgentId() string { return "cline" }

func (clineWriter) Protocol() string { return "openai" }

func (w clineWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errClineNoModel
	}
	providersPath, modelsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, clineLocalKey))
	providers, err := encodeJSON(map[string]any{
		clineSelectedKey: clineProvider,
		"providers":      map[string]any{clineProvider: w.entry(preview)},
	})
	if err != nil {
		return nil, err
	}
	models, err := encodeJSON(map[string]any{
		"providers": map[string]any{clineProvider: w.catalog(endpoint)},
	})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: providersPath, Format: "json", Preview: string(providers)},
		{Path: modelsPath, Format: "json", Preview: string(models)},
	}, nil
}

func (w clineWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errClineNoModel
	}
	providersPath, modelsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(providersPath)
	if err != nil {
		return nil, err
	}
	models, _, err := readJSON(modelsPath)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	if value := stringAt(settings, clineSelectedKey); value != "" {
		previous[clineSelectedKey] = value
	}

	ensureNested(settings, "providers")[clineProvider] = w.entry(endpoint)
	settings[clineSelectedKey] = clineProvider
	ensureNested(models, "providers")[clineProvider] = w.catalog(endpoint)

	changes := &txn{}
	if err := w.stage(changes, providersPath, settings); err != nil {
		changes.abort()
		return nil, err
	}
	if err := w.stage(changes, modelsPath, models); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w clineWriter) Restore(target Target, previous map[string]string) error {
	providersPath, modelsPath, err := w.paths(target)
	if err != nil {
		return err
	}
	settings, _, err := readJSON(providersPath)
	if err != nil {
		return err
	}
	models, _, err := readJSON(modelsPath)
	if err != nil {
		return err
	}

	w.dropProvider(settings)
	w.dropProvider(models)
	if value, ok := previous[clineSelectedKey]; ok {
		settings[clineSelectedKey] = value
	} else {
		delete(settings, clineSelectedKey)
	}

	changes := &txn{}
	if err := w.stage(changes, providersPath, settings); err != nil {
		changes.abort()
		return err
	}
	if err := w.stage(changes, modelsPath, models); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

// Current is the endpoint behind the provider Cline starts on. A provider
// without a base URL is one Cline knows itself, which is still worth naming: it
// is not the endpoint Gateway wrote.
func (w clineWriter) Current(target Target) (string, error) {
	settings, err := w.settingsOf(target)
	if err != nil {
		return "", err
	}
	selected := stringAt(settings, clineSelectedKey)
	if selected == "" {
		return "", nil
	}
	if baseUrl := stringAt(w.settingsFor(settings, selected), "baseUrl"); baseUrl != "" {
		return baseUrl, nil
	}
	return selected, nil
}

// Builtin is the model Cline starts on without Gateway. Cline ships no provider
// of its own, so a file that selects none has no name to show.
func (w clineWriter) Builtin(target Target, previous map[string]string) string {
	settings, err := w.settingsOf(target)
	if err != nil {
		return ""
	}

	selected := stringAt(settings, clineSelectedKey)
	if previous != nil {
		selected = previous[clineSelectedKey]
	} else if selected == clineProvider {
		return ""
	}
	if selected == "" {
		return ""
	}
	return stringAt(w.settingsFor(settings, selected), "model")
}

// entry is one provider as providers.json stores it: the settings Cline hands
// its runtime, wrapped in the bookkeeping the file keeps beside them.
func (clineWriter) entry(endpoint Endpoint) map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"provider": clineProvider,
			"apiKey":   emptyAs(endpoint.ApiKey, clineLocalKey),
			"baseUrl":  endpoint.BaseUrl,
			"model":    endpoint.Model,
			"client":   clineClient,
			"protocol": clineProtocol,
		},
		"updatedAt":   time.Now().UTC().Format(time.RFC3339),
		"tokenSource": "manual",
	}
}

// catalog is the models.json entry. Cline ships no catalog for an endpoint it
// does not know, so the entry carries the whole of it and its own picker can
// switch between the models without Gateway writing the file again.
func (clineWriter) catalog(endpoint Endpoint) map[string]any {
	models := map[string]any{}
	for _, model := range endpoint.catalog() {
		models[model] = map[string]any{"id": model, "name": model}
	}
	return map[string]any{
		"provider": map[string]any{
			"name":           clineProviderName,
			"baseUrl":        endpoint.BaseUrl,
			"defaultModelId": endpoint.Model,
			"client":         clineClient,
			"protocol":       clineProtocol,
		},
		"models": models,
	}
}

func (clineWriter) dropProvider(config map[string]any) {
	if providers := objectAt(config, "providers"); providers != nil {
		delete(providers, clineProvider)
	}
}

// stage queues one file, removing it instead once Gateway's entry was the only
// thing in it: an empty settings file is not what Cline had before.
func (w clineWriter) stage(changes *txn, path string, config map[string]any) error {
	pruneEmpty(config, "providers")
	pruneEmpty(config, "modes")
	if w.isEmpty(config) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	config["version"] = clineFileVersion
	// providers.json carries a modes block, which Cline reads before it reaches
	// the providers beside it.
	if strings.HasSuffix(path, "providers.json") && objectAt(config, "modes") == nil {
		config["modes"] = map[string]any{}
	}

	data, err := encodeJSON(config)
	if err != nil {
		return err
	}
	return changes.write(path, data)
}

// isEmpty reports a file holding nothing but the schema version, which is the
// one key Gateway writes on its own account.
func (clineWriter) isEmpty(config map[string]any) bool {
	for key := range config {
		if key != "version" {
			return false
		}
	}
	return true
}

func (w clineWriter) settingsOf(target Target) (map[string]any, error) {
	providersPath, _, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(providersPath)
	return settings, err
}

// settingsFor is the settings block of one provider entry.
func (clineWriter) settingsFor(config map[string]any, name string) map[string]any {
	return nestedObject(config, "providers", name, "settings")
}

// paths are the two files a switch writes, both under Cline's own data
// directory: the provider settings and the model catalog beside them.
func (clineWriter) paths(target Target) (string, string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, ".cline", "data", "settings")
	return filepath.Join(dir, "providers.json"), filepath.Join(dir, "models.json"), nil
}
