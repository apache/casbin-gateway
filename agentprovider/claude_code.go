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

// claudeCodeBuiltin is what Claude Code talks to with no endpoint of its own:
// the Anthropic sign-in, whose model the CLI picks by itself.
const claudeCodeBuiltin = "Claude"

// The env keys Gateway owns in settings.json. ANTHROPIC_API_KEY is one of them
// even though a switch never sets it: a key left over from another provider
// would otherwise be sent to the new one.
var claudeCodeKeys = []string{
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_MODEL",
}

type claudeCodeWriter struct{}

func init() {
	register(claudeCodeWriter{})
}

func (claudeCodeWriter) AgentId() string { return "claude-code" }

func (claudeCodeWriter) Protocol() string { return "anthropic" }

func (w claudeCodeWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview, err := encodeJSON(map[string]any{"env": w.env(endpoint, true)})
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "json", Preview: string(preview)}}, nil
}

func (w claudeCodeWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	config, _, err := readJSON(path)
	if err != nil {
		return nil, err
	}

	env := objectAt(config, "env")
	if env == nil {
		env = map[string]any{}
	}
	previous := map[string]string{}
	for _, key := range claudeCodeKeys {
		if value, ok := env[key].(string); ok {
			previous[key] = value
		}
		delete(env, key)
	}
	for key, value := range w.env(endpoint, false) {
		env[key] = value
	}
	config["env"] = env

	data, err := encodeJSON(config)
	if err != nil {
		return nil, err
	}

	changes := &txn{}
	if err := changes.write(path, data); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w claudeCodeWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	config, _, err := readJSON(path)
	if err != nil {
		return err
	}

	env := objectAt(config, "env")
	if env == nil {
		return nil
	}
	for _, key := range claudeCodeKeys {
		if value, ok := previous[key]; ok {
			env[key] = value
		} else {
			delete(env, key)
		}
	}
	if len(env) == 0 {
		delete(config, "env")
	}

	data, err := encodeJSON(config)
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

func (w claudeCodeWriter) Current(target Target) (string, error) {
	env, err := w.envOf(target)
	if err != nil || env == nil {
		return "", err
	}
	baseUrl, _ := env["ANTHROPIC_BASE_URL"].(string)
	return baseUrl, nil
}

func (w claudeCodeWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous["ANTHROPIC_MODEL"], claudeCodeBuiltin)
	}

	env, err := w.envOf(target)
	if err != nil || env == nil {
		return claudeCodeBuiltin
	}
	// A model beside an endpoint Gateway wrote is Gateway's, not the one Claude
	// Code would pick for itself.
	if baseUrl, _ := env["ANTHROPIC_BASE_URL"].(string); strings.Contains(baseUrl, "/v1/agents/") {
		return claudeCodeBuiltin
	}
	model, _ := env["ANTHROPIC_MODEL"].(string)
	return emptyAs(model, claudeCodeBuiltin)
}

// envOf is the env block of settings.json, nil when the file has none.
func (w claudeCodeWriter) envOf(target Target) (map[string]any, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	config, _, err := readJSON(path)
	if err != nil {
		return nil, err
	}
	return objectAt(config, "env"), nil
}

// sessionEnv points Claude Code at the endpoint for one run. It is what a
// switch writes into settings.json, and beats it.
func (w claudeCodeWriter) sessionEnv(endpoint Endpoint) map[string]string {
	env := map[string]string{}
	for key, value := range w.env(endpoint, false) {
		if text, ok := value.(string); ok {
			env[key] = text
		}
	}
	return env
}

// env is the block written into settings.json. Claude Code authenticates with
// ANTHROPIC_AUTH_TOKEN, so the provider key goes there and not into
// ANTHROPIC_API_KEY, which stays cleared.
func (claudeCodeWriter) env(endpoint Endpoint, masked bool) map[string]any {
	token := endpoint.ApiKey
	if masked {
		token = maskSecret(token)
	}

	env := map[string]any{
		"ANTHROPIC_BASE_URL": endpoint.BaseUrl,
	}
	// A provider that forwards the caller's own credentials has no token to
	// write, and writing one would override the sign-in Claude Code already
	// has, which is the login the switch is meant to keep using.
	if endpoint.ApiKey != "" {
		env["ANTHROPIC_AUTH_TOKEN"] = token
	}
	if endpoint.Model != "" {
		env["ANTHROPIC_MODEL"] = endpoint.Model
	}
	return env
}

func (claudeCodeWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}
