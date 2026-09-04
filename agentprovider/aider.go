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
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

// aiderOpenaiPrefix is how aider is told to reach a model over the OpenAI API
// rather than look the name up among the services it knows.
const aiderOpenaiPrefix = "openai/"

// aiderLocalKey stands in for a key the endpoint does not have. aider sends one
// either way, and the gateway takes any credential from this host.
const aiderLocalKey = "casbin-gateway"

// The settings Gateway owns in ~/.aider.conf.yml.
var aiderKeys = []string{"openai-api-base", "openai-api-key", "model"}

// errAiderNoModel rejects a provider aider could not use: it starts on one
// model named on the command line or in this file, and an endpoint it does not
// know has no catalog to pick one from.
var errAiderNoModel = errors.New("aider needs a model name, so bind a provider that lists at least one model")

type aiderWriter struct{}

func init() {
	register(aiderWriter{})
}

func (aiderWriter) AgentId() string { return "aider" }

func (aiderWriter) Protocol() string { return "openai" }

func (w aiderWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	if endpoint.Model == "" {
		return nil, errAiderNoModel
	}
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}

	preview := endpoint
	preview.ApiKey = maskSecret(emptyAs(endpoint.ApiKey, aiderLocalKey))
	text, err := yamledit.Render(w.settings(preview))
	if err != nil {
		return nil, err
	}
	return []File{{Path: path, Format: "yaml", Preview: text}}, nil
}

func (w aiderWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errAiderNoModel
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
	for _, key := range aiderKeys {
		if value := yamledit.String(config, key); value != "" {
			previous[key] = value
		}
	}
	for key, value := range w.settings(endpoint) {
		if err := yamledit.Set(config, value, key); err != nil {
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

func (w aiderWriter) Restore(target Target, previous map[string]string) error {
	path, err := w.configPath(target)
	if err != nil {
		return err
	}
	document, config, err := readYAMLMapping(path)
	if err != nil {
		return err
	}

	for _, key := range aiderKeys {
		if value, ok := previous[key]; ok {
			if err := yamledit.Set(config, value, key); err != nil {
				return err
			}
			continue
		}
		yamledit.Delete(config, key)
	}

	changes := &txn{}
	if err := saveYAML(changes, path, document, config); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

func (w aiderWriter) Current(target Target) (string, error) {
	config, err := w.configOf(target)
	if err != nil {
		return "", err
	}
	return yamledit.String(config, "openai-api-base"), nil
}

// Builtin is the model aider starts on without Gateway, with the routing prefix
// dropped: the name a person recognizes is the one behind it.
func (w aiderWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return strings.TrimPrefix(previous["model"], aiderOpenaiPrefix)
	}

	config, err := w.configOf(target)
	if err != nil {
		return ""
	}
	// A model beside an endpoint Gateway wrote is Gateway's, not the one aider
	// would pick for itself.
	if strings.Contains(yamledit.String(config, "openai-api-base"), "/v1/agents/") {
		return ""
	}
	return strings.TrimPrefix(yamledit.String(config, "model"), aiderOpenaiPrefix)
}

// settings is what a switch writes at the root of the file. The prefix on the
// model is what sends aider to the endpoint beside it rather than to the
// service whose catalog holds that name.
func (aiderWriter) settings(endpoint Endpoint) map[string]string {
	return map[string]string{
		"openai-api-base": endpoint.BaseUrl,
		"openai-api-key":  emptyAs(endpoint.ApiKey, aiderLocalKey),
		"model":           aiderOpenaiPrefix + strings.TrimPrefix(endpoint.Model, aiderOpenaiPrefix),
	}
}

func (w aiderWriter) configOf(target Target) (*yaml.Node, error) {
	path, err := w.configPath(target)
	if err != nil {
		return nil, err
	}
	_, config, err := readYAMLMapping(path)
	return config, err
}

// configPath is the file aider reads for every session, which sits in the home
// directory rather than in a directory of its own.
func (aiderWriter) configPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".aider.conf.yml"), nil
}
