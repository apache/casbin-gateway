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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/apache/casbin-gateway/agenthome"
)

const (
	// kimiProvider is the provider table Gateway owns in config.toml.
	// Everything else in the file, other providers included, is left alone.
	kimiProvider = "casbin-gateway"
	// kimiType is the provider type that reads the base_url beside it, which
	// is what an endpoint Kimi Code does not ship needs.
	kimiType = "openai"
	// kimiAliasPrefix opens the model aliases Gateway writes. Kimi Code keys
	// models by alias rather than by provider, so this is what tells its own
	// tables apart from the ones someone added by hand.
	kimiAliasPrefix = kimiProvider + "/"
	// kimiModelKey is the root key naming the alias Kimi Code starts on, and
	// how its previous value is remembered.
	kimiModelKey = "default_model"
	// kimiLocalKey stands in for a key the endpoint does not have. Kimi Code
	// refuses to start a provider without one, while the gateway takes any
	// credential from this host.
	kimiLocalKey = "casbin-gateway"
	// kimiContextSize is the context every alias declares. Kimi Code requires
	// one and the endpoint carries none, so it is the smaller of the sizes in
	// use: a model with room to spare compacts early, while one told it has
	// room it does not have fails the request instead.
	kimiContextSize = 128000
	// kimiBuiltin is what Kimi Code talks to with no provider of its own: the
	// Kimi account, whose model the CLI picks by itself.
	kimiBuiltin = "Kimi"
)

var kimiProviderPath = []string{"providers", kimiProvider}

// errKimiNoModel rejects a config Kimi Code could not start on: an alias is
// keyed by the model it names, so there has to be one.
var errKimiNoModel = errors.New("Kimi Code needs a model name, so bind a provider that lists at least one model")

type kimiWriter struct{}

func init() {
	register(kimiWriter{})
}

func (kimiWriter) AgentId() string { return "kimi-code" }

func (kimiWriter) Protocol() string { return "openai" }

func (w kimiWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errKimiNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, kimiLocalKey))
	config := tomlSetRootKey("", kimiModelKey, kimiAlias(endpoint.Model))
	config = tomlTidy(tomlAppend(config, w.tables(preview)))
	return []File{{Path: path, Format: "toml", Preview: config}}, nil
}

func (w kimiWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errKimiNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	data, _, _, err := readFile(path)
	if err != nil {
		return nil, err
	}
	document, err := w.decode(path, data)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	// An alias Gateway already wrote is not what a restore should put back.
	if selected := stringOf(document[kimiModelKey]); !strings.HasPrefix(selected, kimiAliasPrefix) && selected != "" {
		previous[kimiModelKey] = selected
	}

	text := w.cut(string(data))
	text = tomlSetRootKey(text, kimiModelKey, kimiAlias(endpoint.Model))
	text = tomlAppend(text, w.tables(endpoint))

	return previous, w.save(path, text)
}

func (w kimiWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	data, _, _, err := readFile(path)
	if err != nil {
		return err
	}

	text := w.cut(string(data))
	if value, ok := previous[kimiModelKey]; ok {
		text = tomlSetRootKey(text, kimiModelKey, value)
	} else {
		text = tomlDeleteRootKey(text, kimiModelKey)
	}
	return w.save(path, text)
}

func (w kimiWriter) Current(target Target) (string, error) {
	document, err := w.document(target)
	if err != nil {
		return "", err
	}

	selected := stringOf(document[kimiModelKey])
	if selected == "" {
		return "", nil
	}
	alias := tableOf(tableOf(document, "models"), selected)
	provider := tableOf(tableOf(document, "providers"), stringOf(alias["provider"]))
	if baseUrl := stringOf(provider["base_url"]); baseUrl != "" {
		return baseUrl, nil
	}
	// An alias whose provider names no URL is one Kimi Code reaches itself,
	// which is still worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

// Builtin is the model Kimi Code starts on without Gateway: the alias its own
// config names, or the account it signs in to when it names none.
func (w kimiWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous[kimiModelKey], kimiBuiltin)
	}

	document, err := w.document(target)
	if err != nil {
		return kimiBuiltin
	}
	selected := stringOf(document[kimiModelKey])
	if strings.HasPrefix(selected, kimiAliasPrefix) {
		return kimiBuiltin
	}
	return emptyAs(selected, kimiBuiltin)
}

// tables are the blocks a switch appends: the provider, then one alias per
// model the endpoint serves, so Kimi Code's own picker can switch between them
// without Gateway writing the file again.
func (kimiWriter) tables(endpoint Endpoint) string {
	blocks := []string{tomlTable(kimiProviderPath,
		[]string{"type", "base_url", "api_key"},
		map[string]string{
			"type":     kimiType,
			"base_url": endpoint.BaseUrl,
			"api_key":  emptyAs(endpoint.ApiKey, kimiLocalKey),
		})}

	for _, model := range endpoint.catalog() {
		blocks = append(blocks, tomlRawTable([]string{"models", kimiAlias(model)},
			[]string{"provider", "model", "max_context_size"},
			map[string]string{
				"provider":         strconv.Quote(kimiProvider),
				"model":            strconv.Quote(model),
				"max_context_size": strconv.Itoa(kimiContextSize),
			}))
	}
	return strings.Join(blocks, "\n")
}

// cut takes out everything Gateway owns: its provider table and every alias
// written under its own name.
func (kimiWriter) cut(text string) string {
	return tomlCutTablesUnder(tomlCutTable(text, kimiProviderPath...), "models", kimiAliasPrefix)
}

// kimiAlias is the name one model is listed under, kept apart from the aliases
// Kimi Code ships so a switch never overwrites one.
func kimiAlias(model string) string {
	return kimiAliasPrefix + model
}

func (kimiWriter) save(path string, text string) error {
	changes := &txn{}
	if err := changes.write(path, []byte(tomlTidy(text))); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

// document is config.toml parsed, empty when the file is not there yet.
func (w kimiWriter) document(target Target) (map[string]any, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	data, _, exists, err := readFile(path)
	if err != nil || !exists {
		return map[string]any{}, err
	}
	return w.decode(path, data)
}

func (kimiWriter) decode(path string, data []byte) (map[string]any, error) {
	document := map[string]any{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return document, nil
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, nil
}

func (kimiWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kimi-code", "config.toml"), nil
}

func stringOf(value any) string {
	text, _ := value.(string)
	return text
}

func tableOf(document map[string]any, key string) map[string]any {
	if document == nil {
		return nil
	}
	value, _ := document[key].(map[string]any)
	return value
}
