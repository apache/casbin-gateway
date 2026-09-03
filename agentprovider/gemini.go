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
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
)

// geminiBuiltin is what the Gemini CLI talks to with no endpoint of its own:
// the Google sign-in, whose model the CLI picks by itself.
const geminiBuiltin = "Gemini"

// geminiApiKeyAuth is the auth method that reads GEMINI_API_KEY and honours
// GOOGLE_GEMINI_BASE_URL. The CLI stores the method it last signed in with, and
// that stored value wins over anything the environment says, so a switch has to
// write it or an account already signed in to Google would keep going there.
const geminiApiKeyAuth = "gemini-api-key"

// geminiAuthKey is where that method is stored, and how the previous value is
// kept beside the environment variables a switch owns.
const geminiAuthKey = "security.auth.selectedType"

// geminiLocalKey stands in for a key the endpoint does not have. The CLI
// refuses to start with an empty GEMINI_API_KEY, while the gateway it is being
// pointed at takes any credential from this host.
const geminiLocalKey = "casbin-gateway"

// The environment Gateway owns in ~/.gemini/.env.
var geminiKeys = []string{
	"GOOGLE_GEMINI_BASE_URL",
	"GEMINI_API_KEY",
	"GEMINI_MODEL",
}

type geminiWriter struct{}

func init() {
	register(geminiWriter{})
}

func (geminiWriter) AgentId() string { return "gemini-cli" }

func (geminiWriter) Protocol() string { return "gemini" }

func (w geminiWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	envPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	preview := &envFile{}
	for key, value := range w.env(endpoint, true) {
		preview.set(key, value)
	}
	auth, err := encodeJSON(map[string]any{
		"security": map[string]any{"auth": map[string]any{"selectedType": geminiApiKeyAuth}},
	})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: envPath, Format: "env", Preview: string(preview.bytes())},
		{Path: settingsPath, Format: "json", Preview: string(auth)},
	}, nil
}

func (w geminiWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	envPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	env, err := readEnvFile(envPath)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	for _, key := range geminiKeys {
		if value, ok := env.get(key); ok {
			previous[key] = value
		}
		env.remove(key)
	}
	for key, value := range w.env(endpoint, false) {
		env.set(key, value)
	}

	auth := ensureNested(settings, "security", "auth")
	if value, ok := auth["selectedType"].(string); ok {
		previous[geminiAuthKey] = value
	}
	auth["selectedType"] = geminiApiKeyAuth

	data, err := encodeJSON(settings)
	if err != nil {
		return nil, err
	}

	changes := &txn{}
	if err := changes.write(envPath, env.bytes()); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.write(settingsPath, data); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w geminiWriter) Restore(target Target, previous map[string]string) error {
	envPath, settingsPath, err := w.paths(target)
	if err != nil {
		return err
	}
	env, err := readEnvFile(envPath)
	if err != nil {
		return err
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return err
	}

	for _, key := range geminiKeys {
		if value, ok := previous[key]; ok {
			env.set(key, value)
		} else {
			env.remove(key)
		}
	}

	if auth := objectAt(objectAt(settings, "security"), "auth"); auth != nil {
		if value, ok := previous[geminiAuthKey]; ok {
			auth["selectedType"] = value
		} else {
			delete(auth, "selectedType")
		}
	}

	data, err := encodeJSON(settings)
	if err != nil {
		return err
	}

	changes := &txn{}
	if err := changes.write(envPath, env.bytes()); err != nil {
		changes.abort()
		return err
	}
	if err := changes.write(settingsPath, data); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

func (w geminiWriter) Current(target Target) (string, error) {
	env, err := w.envOf(target)
	if err != nil {
		return "", err
	}
	baseUrl, _ := env.get("GOOGLE_GEMINI_BASE_URL")
	return baseUrl, nil
}

func (w geminiWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous["GEMINI_MODEL"], geminiBuiltin)
	}

	env, err := w.envOf(target)
	if err != nil {
		return geminiBuiltin
	}
	// A model beside an endpoint Gateway wrote is Gateway's, not the one the
	// CLI would pick for itself.
	if baseUrl, _ := env.get("GOOGLE_GEMINI_BASE_URL"); strings.Contains(baseUrl, "/v1/agents/") {
		return geminiBuiltin
	}
	if model, _ := env.get("GEMINI_MODEL"); model != "" {
		return model
	}

	_, settingsPath, err := w.paths(target)
	if err != nil {
		return geminiBuiltin
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return geminiBuiltin
	}
	return emptyAs(stringAt(objectAt(settings, "model"), "name"), geminiBuiltin)
}

// env is what a switch writes into ~/.gemini/.env. The base URL is what selects
// the endpoint; the key is only there because the CLI insists on one.
func (geminiWriter) env(endpoint Endpoint, masked bool) map[string]string {
	key := emptyAs(endpoint.ApiKey, geminiLocalKey)
	if masked {
		key = maskSecret(key)
	}

	env := map[string]string{
		"GOOGLE_GEMINI_BASE_URL": endpoint.BaseUrl,
		"GEMINI_API_KEY":         key,
	}
	if endpoint.Model != "" {
		env["GEMINI_MODEL"] = endpoint.Model
	}
	return env
}

func (geminiWriter) envOf(target Target) (*envFile, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return nil, err
	}
	return readEnvFile(filepath.Join(home, ".gemini", ".env"))
}

// paths are the two files a switch writes: the environment the CLI loads, and
// the settings that say which sign-in it loads it for.
func (geminiWriter) paths(target Target) (string, string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, ".gemini", ".env"), filepath.Join(home, ".gemini", "settings.json"), nil
}
