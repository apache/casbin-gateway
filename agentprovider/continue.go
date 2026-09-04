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
	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

const (
	// continueEntry is the model entry Gateway owns in config.yaml, found by its
	// name. Every other model in the file is left alone.
	continueEntry = "Casbin Gateway"
	// continueLocalKey stands in for a key the endpoint does not have. Continue
	// sends one either way, and the gateway takes any credential from this host.
	continueLocalKey = "casbin-gateway"
	// continueSchema is the config format version Continue reads. A file
	// Gateway creates has to name it or Continue refuses to load it.
	continueSchema = "v1"
)

// continueRoles are the jobs the gateway entry is offered for. Autocomplete and
// embeddings are left out: those want a small local model, not a chat endpoint.
var continueRoles = []string{"chat", "edit", "apply"}

// errContinueNoModel rejects a provider entry Continue could not use: a model
// entry names exactly one model, and an endpoint it does not know has no
// catalog to pick one from.
var errContinueNoModel = errors.New("Continue needs a model name, so bind a provider that lists at least one model")

type continueWriter struct{}

func init() {
	register(continueWriter{})
}

func (continueWriter) AgentId() string { return "continue" }

func (continueWriter) Protocol() string { return "openai" }

func (w continueWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errContinueNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, continueLocalKey))
	text, err := yamledit.Render(map[string]any{"models": []any{w.model(preview)}})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "yaml", Preview: text}}, nil
}

func (w continueWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errContinueNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	document, config, err := readYAMLMapping(path)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	if name := w.selected(config); name != "" {
		previous["model"] = name
	}

	entry, err := yamledit.Node(w.model(endpoint))
	if err != nil {
		return nil, err
	}
	models, err := w.models(config)
	if err != nil {
		return nil, err
	}
	// Continue starts on the first model that takes the chat role, so the
	// gateway entry replaces the one Gateway wrote before it, or goes first.
	if index := indexOfEntry(models, "name", continueEntry); index >= 0 {
		models.Content[index] = entry
	} else {
		models.Content = append([]*yaml.Node{entry}, models.Content...)
	}
	// A file Gateway creates carries the version Continue reads it as.
	if yamledit.Get(config, "schema") == nil {
		if err := yamledit.Set(config, continueSchema, "schema"); err != nil {
			return nil, err
		}
	}

	changes := &txn{}
	if err := saveYAML(changes, path, document, config); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w continueWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	document, config, err := readYAMLMapping(path)
	if err != nil {
		return err
	}

	models := sequenceAt(config, "models")
	if index := indexOfEntry(models, "name", continueEntry); index >= 0 {
		models.Content = append(models.Content[:index], models.Content[index+1:]...)
	}
	if yamledit.IsEmpty(models) {
		yamledit.Delete(config, "models")
	}
	// The version is Gateway's only when it wrote the whole file, which is what
	// an otherwise empty mapping says.
	if len(config.Content) == 2 && yamledit.String(config, "schema") == continueSchema {
		yamledit.Delete(config, "schema")
	}

	changes := &txn{}
	if err := saveYAML(changes, path, document, config); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

// Current is the endpoint behind the model Continue starts on. A model without
// an apiBase is one Continue reaches through a service it knows, which is still
// worth naming: it is not the endpoint Gateway wrote.
func (w continueWriter) Current(target Target) (string, error) {
	config, err := w.configOf(target)
	if err != nil {
		return "", err
	}
	first := w.firstChat(config)
	if first == nil {
		return "", nil
	}
	if base := yamledit.String(first, "apiBase"); base != "" {
		return base, nil
	}
	return yamledit.String(first, "provider"), nil
}

// Builtin is the model Continue starts on without Gateway.
func (w continueWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return previous["model"]
	}

	config, err := w.configOf(target)
	if err != nil {
		return ""
	}
	return w.selected(config)
}

// selected is the name of the model Continue starts on, empty when that is
// Gateway's own entry.
func (w continueWriter) selected(config *yaml.Node) string {
	first := w.firstChat(config)
	if first == nil {
		return ""
	}
	name := yamledit.String(first, "name")
	if name == continueEntry {
		return ""
	}
	return name
}

// firstChat is the model entry Continue starts a conversation on: the first one
// that takes the chat role, and the first entry at all when none names roles.
func (continueWriter) firstChat(config *yaml.Node) *yaml.Node {
	models := sequenceAt(config, "models")
	if models == nil {
		return nil
	}
	for _, item := range models.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		roles := sequenceAt(item, "roles")
		if roles == nil {
			return item
		}
		for _, role := range roles.Content {
			if role.Value == "chat" {
				return item
			}
		}
	}
	return nil
}

// model is the entry Continue loads the endpoint through. Continue ships no
// catalog for a provider it does not know, so the entry carries the whole of
// it: the endpoint, the key and the one model it serves here.
func (continueWriter) model(endpoint Endpoint) map[string]any {
	return map[string]any{
		"name":     continueEntry,
		"provider": "openai",
		"model":    endpoint.Model,
		"apiBase":  endpoint.BaseUrl,
		"apiKey":   emptyAs(endpoint.ApiKey, continueLocalKey),
		"roles":    continueRoles,
	}
}

// models is the list a switch writes into, created when the file has none.
func (continueWriter) models(config *yaml.Node) (*yaml.Node, error) {
	if models := sequenceAt(config, "models"); models != nil {
		return models, nil
	}
	if yamledit.Get(config, "models") != nil {
		return nil, errors.New("models must be a YAML list")
	}
	models := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	if err := yamledit.Set(config, models, "models"); err != nil {
		return nil, err
	}
	return models, nil
}

func (w continueWriter) configOf(target Target) (*yaml.Node, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	_, config, err := readYAMLMapping(path)
	return config, err
}

// configPath is the one file the IDE extensions and the cn CLI all read.
func (continueWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".continue", "config.yaml"), nil
}
