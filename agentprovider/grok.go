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
	// grokAliasPrefix opens the [model.*] tables Gateway writes, which is what
	// tells them apart from the ones Grok ships or someone added by hand.
	grokAliasPrefix = "casbin-gateway/"
	// grokModels is the table naming which model does what.
	grokModels = "models"
	// grokBackend is the wire format each table asks for.
	grokBackend = "chat_completions"
	// grokLocalKey stands in for a key the endpoint does not have. Without one
	// Grok sends its xAI session token instead.
	grokLocalKey = "casbin-gateway"
	// grokContextWindow drives auto-compact. The endpoint carries no size, so
	// this is the smaller one in use, for the reason kimiContextSize is.
	grokContextWindow = 128000
	// grokBuiltin is what Grok talks to with nothing bound: the xAI account.
	grokBuiltin = "Grok"
)

// grokRoles are the [models] keys a switch points at the bound model: the one
// sessions start on, and the one titles are written with, which otherwise asks
// the session's endpoint for an xAI model it does not serve.
var grokRoles = []string{"default", "session_summary"}

var errGrokNoModel = errors.New("Grok Build needs a model name, so bind a provider that lists at least one model")

type grokWriter struct{}

func init() {
	register(grokWriter{})
}

func (grokWriter) AgentId() string { return "grok-build" }

func (grokWriter) Protocol() string { return "openai" }

func (w grokWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errGrokNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, grokLocalKey))
	config := ""
	for _, role := range grokRoles {
		config = tomlSetTableKey(config, grokModels, role, grokAlias(endpoint.Model))
	}
	config = tomlTidy(tomlAppend(config, w.tables(preview)))
	return []File{{Path: path, Format: "toml", Preview: config}}, nil
}

func (w grokWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errGrokNoModel
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
	models := tableOf(document, grokModels)
	for _, role := range grokRoles {
		// A model Gateway already wrote is not what a restore should put back.
		if value := stringOf(models[role]); value != "" && !strings.HasPrefix(value, grokAliasPrefix) {
			previous[role] = value
		}
	}

	text := w.cut(string(data))
	for _, role := range grokRoles {
		text = tomlSetTableKey(text, grokModels, role, grokAlias(endpoint.Model))
	}
	text = tomlAppend(text, w.tables(endpoint))
	return previous, w.save(path, text)
}

func (w grokWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	data, _, _, err := readFile(path)
	if err != nil {
		return err
	}

	text := w.cut(string(data))
	for _, role := range grokRoles {
		if value, ok := previous[role]; ok {
			text = tomlSetTableKey(text, grokModels, role, value)
		} else {
			text = tomlDeleteTableKey(text, grokModels, role)
		}
	}
	return w.save(path, text)
}

// Current is the base URL the starting model is reached at. One of Grok's own
// models names none and goes to xAI, which is no provider of this machine's.
func (w grokWriter) Current(target Target) (string, error) {
	document, err := w.document(target)
	if err != nil {
		return "", err
	}

	selected := stringOf(tableOf(document, grokModels)["default"])
	if baseUrl := stringOf(tableOf(tableOf(document, "model"), selected)["base_url"]); baseUrl != "" {
		return baseUrl, nil
	}
	return stringOf(tableOf(document, "endpoints")["models_base_url"]), nil
}

func (w grokWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous["default"], grokBuiltin)
	}

	document, err := w.document(target)
	if err != nil {
		return grokBuiltin
	}
	selected := stringOf(tableOf(document, grokModels)["default"])
	if strings.HasPrefix(selected, grokAliasPrefix) {
		return grokBuiltin
	}
	return emptyAs(selected, grokBuiltin)
}

// tables are one [model.*] table per model the endpoint serves, so Grok's own
// picker switches between them without Gateway writing the file again. Each
// carries its own key, which keeps the xAI session token from being sent here.
func (grokWriter) tables(endpoint Endpoint) string {
	blocks := []string{}
	for _, model := range endpoint.catalog() {
		blocks = append(blocks, tomlRawTable([]string{"model", grokAlias(model)},
			[]string{"model", "base_url", "api_key", "api_backend", "context_window"},
			map[string]string{
				"model":          strconv.Quote(model),
				"base_url":       strconv.Quote(endpoint.BaseUrl),
				"api_key":        strconv.Quote(emptyAs(endpoint.ApiKey, grokLocalKey)),
				"api_backend":    strconv.Quote(grokBackend),
				"context_window": strconv.Itoa(grokContextWindow),
			}))
	}
	return strings.Join(blocks, "\n")
}

func (grokWriter) cut(text string) string {
	return tomlCutTablesUnder(text, "model", grokAliasPrefix)
}

func grokAlias(model string) string {
	return grokAliasPrefix + model
}

func (grokWriter) save(path string, text string) error {
	changes := &txn{}
	if err := changes.write(path, []byte(tomlTidy(text))); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

func (w grokWriter) document(target Target) (map[string]any, error) {
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

func (grokWriter) decode(path string, data []byte) (map[string]any, error) {
	document := map[string]any{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return document, nil
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, nil
}

func (grokWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".grok", "config.toml"), nil
}
