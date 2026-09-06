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

// qwenBuiltin is what Qwen Code talks to with no endpoint of its own: the Qwen
// sign-in, whose model the CLI picks by itself.
const qwenBuiltin = "Qwen"

// qwenOpenaiAuth is the auth method that reads the OPENAI_* variables. Qwen
// Code stores the method it last signed in with and that stored value wins, so
// a switch has to write it or an account already signed in to Qwen would keep
// going there.
const qwenOpenaiAuth = "openai"

// qwenAuthKey is where that method is stored, and how the previous value is
// kept beside the environment variables a switch owns.
const qwenAuthKey = "security.auth.selectedType"

// qwenLocalKey stands in for a key the endpoint does not have. The CLI refuses
// to start without one, while the gateway takes any credential from this host.
const qwenLocalKey = "casbin-gateway"

// The environment Gateway owns in ~/.qwen/.env.
var qwenKeys = []string{
	"OPENAI_BASE_URL",
	"OPENAI_API_KEY",
	"OPENAI_MODEL",
}

type qwenWriter struct{}

func init() {
	register(qwenWriter{})
}

func (qwenWriter) AgentId() string { return "qwen-code" }

func (qwenWriter) Protocol() string { return "openai" }

func (w qwenWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	envPath, settingsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	preview := &envFile{}
	masked := w.env(endpoint, true)
	for _, key := range qwenKeys {
		if value, ok := masked[key]; ok {
			preview.set(key, value)
		}
	}
	auth, err := encodeJSON(map[string]any{
		"security": map[string]any{"auth": map[string]any{"selectedType": qwenOpenaiAuth}},
	})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: envPath, Format: "env", Preview: string(preview.bytes())},
		{Path: settingsPath, Format: "json", Preview: string(auth)},
	}, nil
}

func (w qwenWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
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
	for _, key := range qwenKeys {
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
		previous[qwenAuthKey] = value
	}
	auth["selectedType"] = qwenOpenaiAuth

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

func (w qwenWriter) Restore(target Target, previous map[string]string) error {
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

	for _, key := range qwenKeys {
		if value, ok := previous[key]; ok {
			env.set(key, value)
		} else {
			env.remove(key)
		}
	}

	if auth := objectAt(objectAt(settings, "security"), "auth"); auth != nil {
		if value, ok := previous[qwenAuthKey]; ok {
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

func (w qwenWriter) Current(target Target) (string, error) {
	env, err := w.envOf(target)
	if err != nil {
		return "", err
	}
	baseUrl, _ := env.get("OPENAI_BASE_URL")
	return baseUrl, nil
}

func (w qwenWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous["OPENAI_MODEL"], qwenBuiltin)
	}

	env, err := w.envOf(target)
	if err != nil {
		return qwenBuiltin
	}
	// A model beside an endpoint Gateway wrote is Gateway's, not the one the
	// CLI would pick for itself.
	if baseUrl, _ := env.get("OPENAI_BASE_URL"); strings.Contains(baseUrl, "/v1/agents/") {
		return qwenBuiltin
	}
	if model, _ := env.get("OPENAI_MODEL"); model != "" {
		return model
	}

	_, settingsPath, err := w.paths(target)
	if err != nil {
		return qwenBuiltin
	}
	settings, _, err := readJSON(settingsPath)
	if err != nil {
		return qwenBuiltin
	}
	return emptyAs(stringAt(objectAt(settings, "model"), "name"), qwenBuiltin)
}

// sessionEnv points Qwen Code at the endpoint for one run. Its .env file never
// replaces a variable already exported, so this beats it.
func (w qwenWriter) sessionEnv(endpoint Endpoint) map[string]string {
	return w.env(endpoint, false)
}

// env is what a switch writes into ~/.qwen/.env. The base URL selects the
// endpoint; the key is only there because the CLI insists on one.
func (qwenWriter) env(endpoint Endpoint, masked bool) map[string]string {
	key := emptyAs(endpoint.ApiKey, qwenLocalKey)
	if masked {
		key = maskSecret(key)
	}

	env := map[string]string{
		"OPENAI_BASE_URL": endpoint.BaseUrl,
		"OPENAI_API_KEY":  key,
	}
	if endpoint.Model != "" {
		env["OPENAI_MODEL"] = endpoint.Model
	}
	return env
}

func (qwenWriter) envOf(target Target) (*envFile, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return nil, err
	}
	return readEnvFile(filepath.Join(home, ".qwen", ".env"))
}

// paths are the two files a switch writes: the environment the CLI loads, and
// the settings that say which sign-in it loads it for.
func (qwenWriter) paths(target Target) (string, string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(home, ".qwen", ".env"), filepath.Join(home, ".qwen", "settings.json"), nil
}
