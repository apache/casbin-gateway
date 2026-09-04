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

// iflowBuiltin is what iFlow CLI talks to with no endpoint of its own: the
// iFlow account, whose model the CLI picks by itself.
const iflowBuiltin = "iFlow"

// iflowOpenaiAuth is the auth method that reads the baseUrl beside it. The CLI
// stores the method it last signed in with and that stored value wins, so a
// switch has to write it or an account already signed in to iFlow would keep
// going there.
const iflowOpenaiAuth = "openai-compatible"

// iflowLocalKey stands in for a key the endpoint does not have. The CLI refuses
// to start without one, while the gateway takes any credential from this host.
const iflowLocalKey = "casbin-gateway"

// The settings keys Gateway owns in ~/.iflow/settings.json. iFlow keeps them at
// the root of the file, beside everything else it stores there.
const (
	iflowAuthKey  = "selectedAuthType"
	iflowUrlKey   = "baseUrl"
	iflowKeyKey   = "apiKey"
	iflowModelKey = "modelName"
)

var iflowKeys = []string{iflowAuthKey, iflowUrlKey, iflowKeyKey, iflowModelKey}

type iflowWriter struct{}

func init() {
	register(iflowWriter{})
}

func (iflowWriter) AgentId() string { return "iflow-cli" }

func (iflowWriter) Protocol() string { return "openai" }

func (w iflowWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, iflowLocalKey))
	data, err := encodeJSON(w.settings(preview))
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "json", Preview: string(data)}}, nil
}

func (w iflowWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	settings, _, err := readJSON(path)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	for _, key := range iflowKeys {
		if value := stringAt(settings, key); value != "" {
			previous[key] = value
		}
	}
	for key, value := range w.settings(endpoint) {
		settings[key] = value
	}
	// The model is what the CLI starts on, and a bound provider that names none
	// leaves it to pick from the endpoint itself.
	if endpoint.Model == "" {
		delete(settings, iflowModelKey)
	}

	return previous, w.save(path, settings)
}

func (w iflowWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	settings, _, err := readJSON(path)
	if err != nil {
		return err
	}

	for _, key := range iflowKeys {
		if value, ok := previous[key]; ok {
			settings[key] = value
		} else {
			delete(settings, key)
		}
	}
	return w.save(path, settings)
}

func (w iflowWriter) Current(target Target) (string, error) {
	path, err := w.configPath(target)
	if err != nil {
		return "", err
	}
	settings, _, err := readJSON(path)
	if err != nil {
		return "", err
	}
	return stringAt(settings, iflowUrlKey), nil
}

func (w iflowWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous[iflowModelKey], iflowBuiltin)
	}

	path, err := w.configPath(target)
	if err != nil {
		return iflowBuiltin
	}
	settings, _, err := readJSON(path)
	if err != nil {
		return iflowBuiltin
	}
	// A model beside an endpoint Gateway wrote is Gateway's, not the one the
	// CLI would pick for itself.
	if strings.Contains(stringAt(settings, iflowUrlKey), "/v1/agents/") {
		return iflowBuiltin
	}
	return emptyAs(stringAt(settings, iflowModelKey), iflowBuiltin)
}

// settings is what a switch writes at the root of the file. The auth type is
// what makes the CLI read the three keys below it rather than sign in.
func (iflowWriter) settings(endpoint Endpoint) map[string]any {
	settings := map[string]any{
		iflowAuthKey: iflowOpenaiAuth,
		iflowUrlKey:  endpoint.BaseUrl,
		iflowKeyKey:  emptyAs(endpoint.ApiKey, iflowLocalKey),
	}
	if endpoint.Model != "" {
		settings[iflowModelKey] = endpoint.Model
	}
	return settings
}

func (iflowWriter) save(path string, settings map[string]any) error {
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

func (iflowWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".iflow", "settings.json"), nil
}
