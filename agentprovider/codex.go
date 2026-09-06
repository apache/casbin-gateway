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
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/apache/casbin-gateway/agentmonitor"
)

const (
	// codexProviderName is the model provider Gateway owns in config.toml.
	// Everything else in the file, including other providers, is left alone.
	codexProviderName = "casbin-gateway"
	// codexAuthKey is the auth.json entry an older release wrote the key to.
	// Codex reads it only while signed in with an API key: a user signed in
	// through ChatGPT got "Missing environment variable: OPENAI_API_KEY"
	// instead, so the key now travels in the header below.
	codexAuthKey = "OPENAI_API_KEY"
	// codexBuiltin is what Codex talks to with no provider entry of its own: the
	// ChatGPT sign-in, whose model the CLI picks by itself.
	codexBuiltin = "ChatGPT"
	// codexAuthHeader carries the key on every request Codex sends to the
	// provider, which is the one place Codex reads it from without an
	// environment variable and without a sign-in of its own.
	codexAuthHeader = "Authorization"
	// codexOpenAiAuthKey tells Codex to send the ChatGPT sign-in it already has
	// to this provider instead of looking for a key of its own. It is what puts a
	// subscription through the gateway, which forwards it to the Codex backend.
	codexOpenAiAuthKey = "requires_openai_auth"
)

// The keys of the root table Gateway owns, remembered under these names.
const (
	codexModelProviderKey = "model_provider"
	codexModelKey         = "model"
	codexAuthStateKey     = "auth." + codexAuthKey
	// codexHeaderStateKey marks a state Gateway wrote without touching
	// auth.json. A state without it comes from the release that did, and only
	// that one has an auth.json entry to put back.
	codexHeaderStateKey = "auth.inHeader"
)

var (
	codexProviderPath = []string{"model_providers", codexProviderName}
	codexHeaderPath   = []string{"model_providers", codexProviderName, "http_headers"}
)

// errCodexNoKey rejects a provider with nothing to authenticate with. A
// client-auth provider is not one of them: there Codex sends the sign-in it
// already has, and the entry says so instead of carrying a key.
var errCodexNoKey = errors.New("Codex has no key to send this provider, and the provider does not forward the caller's own login either")

// errCodexResponsesApi rejects writing an upstream Codex cannot reach. Codex
// dropped the chat completions wire format, so a provider that serves only that
// is reachable through the gateway, which translates, but not directly.
var errCodexResponsesApi = errors.New("Codex only speaks the OpenAI Responses API, which this provider does not serve, so switch this agent to gateway mode")

type codexWriter struct {
	id string
}

func init() {
	register(codexWriter{id: "codex"})
	register(codexWriter{id: "codex-cli"})
}

func (w codexWriter) AgentId() string { return w.id }

func (codexWriter) Protocol() string { return "openai" }

func (w codexWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}

	home, err := agentmonitor.ResolveCodexHome(target.Path, target.Owner)
	if err != nil {
		return nil, err
	}

	config := tomlSetRootKey("", codexModelProviderKey, codexProviderName)
	if endpoint.Model != "" {
		config = tomlSetRootKey(config, codexModelKey, endpoint.Model)
	}
	preview := endpoint
	preview.ApiKey = maskSecret(endpoint.ApiKey)
	config = tomlTidy(tomlAppend(config, w.providerTable(preview)))

	return []File{
		{Path: filepath.Join(home, "config.toml"), Format: "toml", Preview: config},
	}, nil
}

func (w codexWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if err := w.check(endpoint); err != nil {
		return nil, err
	}

	configPath, _, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	data, _, _, err := readFile(configPath)
	if err != nil {
		return nil, err
	}
	document, err := w.decode(configPath, data)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{codexHeaderStateKey: "1"}
	// The CLI and the VS Code integration share one ~/.codex, so what is found
	// here can be Gateway's own selection from the other one, model included,
	// which is not what a restore should put back.
	if selected, _ := document[codexModelProviderKey].(string); selected != codexProviderName {
		if selected != "" {
			previous[codexModelProviderKey] = selected
		}
		if value, ok := document[codexModelKey].(string); ok {
			previous[codexModelKey] = value
		}
	}

	text := tomlCutTable(string(data), codexProviderPath...)
	text = tomlSetRootKey(text, codexModelProviderKey, codexProviderName)
	if endpoint.Model != "" {
		text = tomlSetRootKey(text, codexModelKey, endpoint.Model)
	}
	text = tomlAppend(text, w.providerTable(endpoint))

	changes := &txn{}
	if err := changes.write(configPath, []byte(text)); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w codexWriter) Restore(target Target, previous map[string]string) error {
	configPath, authPath, err := w.paths(target)
	if err != nil {
		return err
	}

	data, _, _, err := readFile(configPath)
	if err != nil {
		return err
	}
	text := tomlCutTable(string(data), codexProviderPath...)
	for _, key := range []string{codexModelProviderKey, codexModelKey} {
		value, ok := previous[key]
		// A state that recorded Gateway's own entry, from the installation
		// sharing this file, would leave the agent switched if it were put back.
		if key == codexModelProviderKey && value == codexProviderName {
			ok = false
		}
		if ok {
			text = tomlSetRootKey(text, key, value)
		} else {
			text = tomlDeleteRootKey(text, key)
		}
	}
	text = strings.TrimLeft(text, "\n")

	changes := &txn{}
	if err := changes.write(configPath, []byte(text)); err != nil {
		changes.abort()
		return err
	}

	// Only a state from the release that put the key in auth.json has one to
	// take back out; a newer one, and a restore with no state at all, never
	// opened the file.
	_, inHeader := previous[codexHeaderStateKey]
	if previous != nil && !inHeader {
		auth, _, err := readJSON(authPath)
		if err != nil {
			changes.abort()
			return err
		}
		if value, ok := previous[codexAuthStateKey]; ok {
			auth[codexAuthKey] = value
		} else {
			delete(auth, codexAuthKey)
		}
		authData, err := encodeJSON(auth)
		if err != nil {
			changes.abort()
			return err
		}
		if err := changes.write(authPath, authData); err != nil {
			changes.abort()
			return err
		}
	}
	return changes.commit()
}

func (w codexWriter) Current(target Target) (string, error) {
	document, err := w.document(target)
	if err != nil {
		return "", err
	}

	selected, _ := document[codexModelProviderKey].(string)
	if selected == "" {
		return "", nil
	}
	providers, _ := document["model_providers"].(map[string]any)
	provider, _ := providers[selected].(map[string]any)
	if baseUrl, ok := provider["base_url"].(string); ok {
		return baseUrl, nil
	}
	// A provider without a base URL is one Codex knows itself, which is still
	// worth naming: it is not the endpoint Gateway wrote.
	return selected, nil
}

func (w codexWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous[codexModelKey], codexBuiltin)
	}

	document, err := w.document(target)
	if err != nil {
		return codexBuiltin
	}
	// A model beside Gateway's own provider entry is Gateway's, not the one
	// Codex would pick for itself.
	if selected, _ := document[codexModelProviderKey].(string); selected == codexProviderName {
		return codexBuiltin
	}
	model, _ := document[codexModelKey].(string)
	return emptyAs(model, codexBuiltin)
}

// document is config.toml parsed, empty when the file is not there yet.
func (w codexWriter) document(target Target) (map[string]any, error) {
	configPath, _, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	data, _, exists, err := readFile(configPath)
	if err != nil || !exists {
		return map[string]any{}, err
	}
	return w.decode(configPath, data)
}

// check reports why Codex cannot be pointed at endpoint.
func (codexWriter) check(endpoint Endpoint) error {
	if endpoint.ApiKey == "" && !endpoint.ClientAuth {
		return errCodexNoKey
	}
	if !endpoint.ServesResponsesApi {
		return errCodexResponsesApi
	}
	return nil
}

// providerTable is the [model_providers.casbin-gateway] block, followed by the
// header sub-table carrying the key. env_key would send Codex looking for an
// environment variable no shell exports, so the key is written here instead.
// A client-auth provider has no key to write: the entry asks Codex for the
// sign-in it already holds, which is the credential forwarded upstream.
func (codexWriter) providerTable(endpoint Endpoint) string {
	table := tomlTable(codexProviderPath,
		[]string{"name", "base_url", "wire_api"},
		map[string]string{
			"name":     "Casbin Gateway",
			"base_url": endpoint.BaseUrl,
			"wire_api": "responses",
		})
	if endpoint.ClientAuth {
		return table + codexOpenAiAuthKey + " = true\n"
	}

	headers := tomlTable(codexHeaderPath,
		[]string{codexAuthHeader},
		map[string]string{codexAuthHeader: "Bearer " + endpoint.ApiKey})
	return table + "\n" + headers
}

func (codexWriter) decode(path string, data []byte) (map[string]any, error) {
	document := map[string]any{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return document, nil
	}
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, nil
}

func (codexWriter) paths(target Target) (string, string, error) {
	home, err := agentmonitor.ResolveCodexHome(target.Path, target.Owner)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, "config.toml"), filepath.Join(home, "auth.json"), nil
}
