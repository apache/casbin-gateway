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

	"github.com/apache/casbin-gateway/agenthome"
)

const (
	// droidEntry is the custom model Gateway owns in settings.json, found by
	// its display name. Every other model in the file is left alone.
	droidEntry = "Casbin Gateway"
	// droidGenericProvider is how droid is told to speak plain chat completions
	// to an endpoint it does not ship.
	droidGenericProvider = "generic-chat-completion-api"
	// droidLocalKey stands in for a key the endpoint does not have. droid sends
	// one either way, and the gateway takes any credential from this host.
	droidLocalKey = "casbin-gateway"
	// droidMaxOutputTokens is what a custom model is given when the endpoint
	// says nothing: droid needs a number, and this is the one its own
	// documentation uses.
	droidMaxOutputTokens = 16384
)

// errDroidNoModel rejects a provider droid could not use: a custom model entry
// names exactly one model, and an endpoint it does not know has no catalog to
// pick one from.
var errDroidNoModel = errors.New("droid needs a model name, so bind a provider that lists at least one model")

type droidWriter struct{}

func init() {
	register(droidWriter{})
}

func (droidWriter) AgentId() string { return "droid" }

func (droidWriter) Protocol() string { return "openai" }

func (w droidWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errDroidNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, droidLocalKey))
	data, err := encodeJSON(map[string]any{"customModels": []any{w.model(preview)}})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "json", Preview: string(data)}}, nil
}

func (w droidWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errDroidNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(path)
	if err != nil {
		return nil, err
	}

	models := w.customModels(settings)
	previous := map[string]string{}
	if name := w.builtinOf(models); name != "" {
		previous["model"] = name
	}

	// droid starts on the first custom model, so the gateway entry replaces the
	// one Gateway wrote before it, or goes first.
	entry := w.model(endpoint)
	if index := w.indexOf(models); index >= 0 {
		models[index] = entry
	} else {
		models = append([]any{entry}, models...)
	}
	settings["customModels"] = models

	return previous, w.save(path, settings)
}

func (w droidWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	settings, _, err := readJSON(path)
	if err != nil {
		return err
	}

	models := w.customModels(settings)
	if index := w.indexOf(models); index >= 0 {
		models = append(models[:index], models[index+1:]...)
	}
	if len(models) == 0 {
		delete(settings, "customModels")
	} else {
		settings["customModels"] = models
	}
	if len(settings) == 0 {
		return removeFile(path)
	}
	return w.save(path, settings)
}

// Current is the endpoint behind the model droid starts on.
func (w droidWriter) Current(target Target) (string, error) {
	settings, err := w.settingsOf(target)
	if err != nil {
		return "", err
	}
	first := w.first(w.customModels(settings))
	if first == nil {
		return "", nil
	}
	return stringAt(first, "baseUrl"), nil
}

// Builtin is the model droid starts on without Gateway. droid signs in to
// Factory on its own, so a file that lists no custom model has no name to show.
func (w droidWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return previous["model"]
	}

	settings, err := w.settingsOf(target)
	if err != nil {
		return ""
	}
	return w.builtinOf(w.customModels(settings))
}

// builtinOf is the model of the first entry that is not Gateway's own.
func (w droidWriter) builtinOf(models []any) string {
	for _, item := range models {
		entry, ok := item.(map[string]any)
		if !ok || stringAt(entry, "displayName") == droidEntry {
			continue
		}
		return stringAt(entry, "model")
	}
	return ""
}

// model is the custom model droid loads the endpoint through. droid ships no
// catalog for an endpoint it does not know, so the entry carries the whole of
// it: the endpoint, the key and the one model it serves here.
func (droidWriter) model(endpoint Endpoint) map[string]any {
	return map[string]any{
		"model":           endpoint.Model,
		"displayName":     droidEntry,
		"baseUrl":         endpoint.BaseUrl,
		"apiKey":          emptyAs(endpoint.ApiKey, droidLocalKey),
		"provider":        droidGenericProvider,
		"maxOutputTokens": droidMaxOutputTokens,
	}
}

func (droidWriter) customModels(settings map[string]any) []any {
	models, _ := settings["customModels"].([]any)
	return models
}

// indexOf is the position of Gateway's own entry, -1 when the file has none.
func (droidWriter) indexOf(models []any) int {
	for index, item := range models {
		if entry, ok := item.(map[string]any); ok && stringAt(entry, "displayName") == droidEntry {
			return index
		}
	}
	return -1
}

func (droidWriter) first(models []any) map[string]any {
	for _, item := range models {
		if entry, ok := item.(map[string]any); ok {
			return entry
		}
	}
	return nil
}

func (w droidWriter) settingsOf(target Target) (map[string]any, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(path)
	return settings, err
}

func (droidWriter) save(path string, settings map[string]any) error {
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

// configPath is the settings file droid reads. The older config.json beside it
// spells the same keys in snake case and loses to this one, so a switch writes
// only the file that wins.
func (droidWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".factory", "settings.json"), nil
}
