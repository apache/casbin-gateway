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
	"path/filepath"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/internal/hermes"
	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

const (
	// hermesProvider is the provider entry Gateway owns in config.yaml. Every
	// other provider in the file is left alone.
	hermesProvider = "casbin-gateway"
	// hermesApiMode is the wire format the endpoint is called with. The gateway
	// answers OpenAI chat completions under the base URL written below.
	hermesApiMode = "chat_completions"
)

// The settings Gateway overwrites, remembered under these names.
const (
	hermesProviderKey = "model.provider"
	hermesModelKey    = "model.default"
	hermesBaseUrlKey  = "model.base_url"
	hermesApiModeKey  = "model.api_mode"
)

// hermesModelKeys are the fields of the model section Gateway owns, by the name
// each is remembered under.
var hermesModelKeys = map[string]string{
	hermesProviderKey: "provider",
	hermesModelKey:    "default",
	hermesBaseUrlKey:  "base_url",
	hermesApiModeKey:  "api_mode",
}

var hermesProvidersPath = []string{"providers", hermesProvider}

// errHermesNoKey rejects a provider that forwards the caller's own credentials:
// a provider entry Hermes does not ship holds no sign-in of its own, so the key
// written here is the only one it ever sends.
var errHermesNoKey = errors.New("Hermes Agent needs an API key, so it cannot use a provider that forwards the credentials of the caller")

// errHermesNoModel rejects an entry Hermes could not use: the default model is
// what it starts a conversation on.
var errHermesNoModel = errors.New("Hermes Agent needs a model name, so bind a provider that lists at least one model")

// hermesProviderConfig is the provider entry as config.yaml spells it.
type hermesProviderConfig struct {
	Api    string `yaml:"api"`
	ApiKey string `yaml:"api_key"`
}

type hermesWriter struct{}

func init() {
	register(hermesWriter{})
}

func (hermesWriter) AgentId() string { return hermes.AgentID }

func (hermesWriter) Protocol() string { return "openai" }

func (w hermesWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(endpoint.ApiKey)
	config, err := yamledit.Render(map[string]any{
		"model":     w.model(endpoint),
		"providers": map[string]any{hermesProvider: w.provider(preview)},
	})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "yaml", Preview: config}}, nil
}

func (w hermesWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	document, root, err := w.load(path)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	for key, field := range hermesModelKeys {
		if value := yamledit.String(root, "model", field); value != "" {
			previous[key] = value
		}
	}

	if err := yamledit.Set(root, w.provider(endpoint), hermesProvidersPath...); err != nil {
		return nil, err
	}
	if err := yamledit.Set(root, hermesProvider, "model", "provider"); err != nil {
		return nil, err
	}
	if err := yamledit.Set(root, endpoint.Model, "model", "default"); err != nil {
		return nil, err
	}
	if err := yamledit.Set(root, hermesApiMode, "model", "api_mode"); err != nil {
		return nil, err
	}
	// A base URL on the model itself bypasses provider resolution, so one left
	// from another tool would send the requests somewhere else entirely.
	yamledit.Delete(root, "model", "base_url")

	return previous, w.save(path, document)
}

func (w hermesWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	document, root, err := w.load(path)
	if err != nil {
		return err
	}

	yamledit.Delete(root, hermesProvidersPath...)
	if yamledit.IsEmpty(yamledit.Get(root, "providers")) {
		yamledit.Delete(root, "providers")
	}
	for key, field := range hermesModelKeys {
		if value, ok := previous[key]; ok {
			if err := yamledit.Set(root, value, "model", field); err != nil {
				return err
			}
			continue
		}
		yamledit.Delete(root, "model", field)
	}
	if yamledit.IsEmpty(yamledit.Get(root, "model")) {
		yamledit.Delete(root, "model")
	}
	// A config.yaml that never held anything but Gateway's own settings is
	// removed rather than left behind as an empty mapping, which is what the
	// monitoring patch does with the same file.
	if yamledit.IsEmpty(root) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return w.save(path, document)
}

func (w hermesWriter) Current(target Target) (string, error) {
	path, err := w.configPath(target)
	if err != nil {
		return "", err
	}
	_, root, err := w.load(path)
	if err != nil {
		return "", err
	}

	// A base URL on the model wins over the provider it names, so it is the
	// endpoint Hermes actually calls.
	if baseUrl := yamledit.String(root, "model", "base_url"); baseUrl != "" {
		return baseUrl, nil
	}
	selected := yamledit.String(root, "model", "provider")
	if selected == "" {
		return "", nil
	}
	if api := yamledit.String(root, "providers", selected, "api"); api != "" {
		return api, nil
	}
	// A provider without an endpoint is one Hermes resolves itself, which is
	// still worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// Builtin is the model Hermes runs on its own. It resolves a provider from its
// own catalog, so a file naming no model has no name to show.
func (w hermesWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return previous[hermesModelKey]
	}

	path, err := w.configPath(target)
	if err != nil {
		return ""
	}
	_, root, err := w.load(path)
	if err != nil {
		return ""
	}
	if yamledit.String(root, "model", "provider") == hermesProvider {
		return ""
	}
	return yamledit.String(root, "model", "default")
}

// check reports why Hermes cannot be pointed at endpoint.
func (hermesWriter) check(endpoint Endpoint) error {
	if endpoint.ApiKey == "" {
		return errHermesNoKey
	}
	if endpoint.Model == "" {
		return errHermesNoModel
	}
	return nil
}

func (hermesWriter) provider(endpoint Endpoint) hermesProviderConfig {
	return hermesProviderConfig{Api: endpoint.BaseUrl, ApiKey: endpoint.ApiKey}
}

// model is the section that says what Hermes starts on: the provider written
// above, and its first model.
func (hermesWriter) model(endpoint Endpoint) map[string]any {
	return map[string]any{
		"provider": hermesProvider,
		"default":  endpoint.Model,
		"api_mode": hermesApiMode,
	}
}

func (hermesWriter) load(path string) (*yamledit.Document, *yaml.Node, error) {
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

func (hermesWriter) save(path string, document *yamledit.Document) error {
	data, err := document.Bytes()
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

// configPath is the file Hermes reads its settings from, in the home directory
// its own launcher keeps them in.
func (hermesWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".hermes", "config.yaml"), nil
}
