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
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
)

const (
	// codebuddyVendor marks the entries in models.json Gateway owns. CodeBuddy
	// keys its models by id rather than by provider, so this is what tells our
	// entries apart from the ones someone added by hand.
	codebuddyVendor = "Casbin Gateway"
	// codebuddyChatPath completes the endpoint. CodeBuddy sends a custom model
	// to the url as written, so it carries the whole path rather than the root
	// the other agents take.
	codebuddyChatPath = "/chat/completions"
	// codebuddyLocalKey stands in for a key the endpoint does not have. The
	// CLI sends one either way, and the gateway takes any credential from this
	// host.
	codebuddyLocalKey = "casbin-gateway"
)

// The keys Gateway owns, remembered under these names.
const (
	codebuddyModelKey = "settings.model"
	// codebuddyDisplacedKey holds the models.json entries a switch overwrote,
	// as the file stored them: an entry Gateway writes takes the id of a model
	// someone configured themselves, and that one comes back on restore.
	codebuddyDisplacedKey = "models.displaced"
)

// errCodebuddyNoModel rejects a model list CodeBuddy could not use: entries in
// models.json are keyed by the model id, so there has to be one.
var errCodebuddyNoModel = errors.New("CodeBuddy needs a model name, so bind a provider that lists at least one model")

type codebuddyWriter struct{}

func init() {
	register(codebuddyWriter{})
}

func (codebuddyWriter) AgentId() string { return "codebuddy" }

func (codebuddyWriter) Protocol() string { return "openai" }

func (w codebuddyWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errCodebuddyNoModel
	}
	modelsPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, codebuddyLocalKey))
	models, err := encodeJSON(map[string]any{"models": w.entries(preview)})
	if err != nil {
		return nil, err
	}
	settings, err := encodeJSON(map[string]any{"model": endpoint.Model})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: modelsPath, Format: "json", Preview: string(models)},
		{Path: settingsPath, Format: "json", Preview: string(settings)},
	}, nil
}

func (w codebuddyWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errCodebuddyNoModel
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
	if selected := stringAt(settings, "model"); selected != "" && !w.owns(models, selected) {
		previous[codebuddyModelKey] = selected
	}

	entries := w.entries(endpoint)
	ids := w.idsOf(entries)
	kept, displaced := w.split(codebuddyList(models, "models"), ids)
	if len(displaced) > 0 {
		text, err := json.Marshal(displaced)
		if err != nil {
			return nil, err
		}
		previous[codebuddyDisplacedKey] = string(text)
	}

	models["models"] = append(kept, entries...)
	// An absent availableModels shows every model there is, so adding one
	// holding only Gateway's would hide the rest.
	if listed := codebuddyList(models, "availableModels"); listed != nil {
		models["availableModels"] = codebuddyAdd(listed, ids)
	}
	settings["model"] = endpoint.Model

	return previous, w.save(modelsPath, models, settingsPath, settings)
}

func (w codebuddyWriter) Restore(target Target, previous map[string]string) error {
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

	displaced := []any{}
	if text, ok := previous[codebuddyDisplacedKey]; ok {
		if err := json.Unmarshal([]byte(text), &displaced); err != nil {
			return err
		}
	}

	kept, dropped := w.split(codebuddyList(models, "models"), nil)
	models["models"] = append(kept, displaced...)
	// A displaced entry took back the id Gateway's had, so that id stays
	// listed; the rest of Gateway's go.
	if listed := codebuddyList(models, "availableModels"); listed != nil {
		models["availableModels"] = codebuddyDrop(listed, w.idsOf(dropped), w.idsOf(displaced))
	}
	if len(codebuddyList(models, "models")) == 0 {
		delete(models, "models")
		delete(models, "availableModels")
	}

	if value, ok := previous[codebuddyModelKey]; ok {
		settings["model"] = value
	} else {
		delete(settings, "model")
	}
	return w.save(modelsPath, models, settingsPath, settings)
}

func (w codebuddyWriter) Current(target Target) (string, error) {
	models, settings, err := w.read(target)
	if err != nil {
		return "", err
	}

	selected := stringAt(settings, "model")
	if selected == "" {
		return "", nil
	}
	for _, entry := range codebuddyList(models, "models") {
		model, _ := entry.(map[string]any)
		if model != nil && stringAt(model, "id") == selected {
			return strings.TrimSuffix(stringAt(model, "url"), codebuddyChatPath), nil
		}
	}
	// A model with no entry of its own is one CodeBuddy serves itself, which is
	// still worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// Builtin is the model CodeBuddy starts on without Gateway: the one settings
// name, or the account it signs in to when they name none.
func (w codebuddyWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous[codebuddyModelKey], codebuddyBuiltin)
	}

	models, settings, err := w.read(target)
	if err != nil {
		return codebuddyBuiltin
	}
	selected := stringAt(settings, "model")
	if selected == "" || w.owns(models, selected) {
		return codebuddyBuiltin
	}
	return selected
}

// codebuddyBuiltin is what CodeBuddy talks to with no model entry of its own:
// the CodeBuddy account, whose model the CLI picks by itself.
const codebuddyBuiltin = "CodeBuddy"

// entries is the catalog as models.json stores it. CodeBuddy carries the list
// itself, so all of it is written and its own picker can switch between the
// models without Gateway writing the file again.
func (codebuddyWriter) entries(endpoint Endpoint) []any {
	entries := []any{}
	for _, model := range endpoint.catalog() {
		entries = append(entries, map[string]any{
			"id":               model,
			"name":             model,
			"vendor":           codebuddyVendor,
			"url":              endpoint.BaseUrl + codebuddyChatPath,
			"apiKey":           emptyAs(endpoint.ApiKey, codebuddyLocalKey),
			"supportsToolCall": true,
		})
	}
	return entries
}

// split separates the entries Gateway is about to own from the ones it leaves
// alone: its own from a previous switch, and any carrying an id it is about to
// write. The second return is what was taken over, which is what comes back on
// restore.
func (codebuddyWriter) split(entries []any, ids []string) ([]any, []any) {
	taken := map[string]bool{}
	for _, id := range ids {
		taken[id] = true
	}

	kept := []any{}
	removed := []any{}
	for _, entry := range entries {
		model, _ := entry.(map[string]any)
		if model != nil && (stringAt(model, "vendor") == codebuddyVendor || taken[stringAt(model, "id")]) {
			removed = append(removed, entry)
			continue
		}
		kept = append(kept, entry)
	}
	return kept, removed
}

func (codebuddyWriter) idsOf(entries []any) []string {
	ids := []string{}
	for _, entry := range entries {
		if model, _ := entry.(map[string]any); model != nil {
			if id := stringAt(model, "id"); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// owns reports a model id that names one of Gateway's own entries.
func (codebuddyWriter) owns(models map[string]any, id string) bool {
	for _, entry := range codebuddyList(models, "models") {
		model, _ := entry.(map[string]any)
		if model != nil && stringAt(model, "id") == id {
			return stringAt(model, "vendor") == codebuddyVendor
		}
	}
	return false
}

func (w codebuddyWriter) save(modelsPath string, models map[string]any, settingsPath string, settings map[string]any) error {
	changes := &txn{}
	if err := w.stage(changes, modelsPath, models); err != nil {
		changes.abort()
		return err
	}
	if err := w.stage(changes, settingsPath, settings); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

// stage queues one file, removing it instead once Gateway's entries were the
// only thing in it: an empty file is not what CodeBuddy had before.
func (codebuddyWriter) stage(changes *txn, path string, config map[string]any) error {
	if len(config) == 0 {
		return removeFile(path)
	}
	data, err := encodeJSON(config)
	if err != nil {
		return err
	}
	return changes.write(path, data)
}

func (w codebuddyWriter) read(target Target) (map[string]any, map[string]any, error) {
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

// paths are the two files a switch writes: the model catalog, and the settings
// naming the one CodeBuddy starts on.
func (codebuddyWriter) paths(target Target) (string, string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, ".codebuddy")
	return filepath.Join(dir, "models.json"), filepath.Join(dir, "settings.json"), nil
}

// codebuddyList is the array at key, nil when the key is missing or holds
// something else. The difference matters for availableModels, where a missing
// list means every model rather than none.
func codebuddyList(config map[string]any, key string) []any {
	value, ok := config[key].([]any)
	if !ok {
		return nil
	}
	return value
}

func codebuddyAdd(listed []any, ids []string) []any {
	seen := map[string]bool{}
	for _, value := range listed {
		if text, ok := value.(string); ok {
			seen[text] = true
		}
	}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			listed = append(listed, id)
		}
	}
	return listed
}

func codebuddyDrop(listed []any, ids []string, keep []string) []any {
	dropping := map[string]bool{}
	for _, id := range ids {
		dropping[id] = true
	}
	for _, id := range keep {
		dropping[id] = false
	}

	kept := []any{}
	for _, value := range listed {
		if text, ok := value.(string); ok && dropping[text] {
			continue
		}
		kept = append(kept, value)
	}
	return kept
}
