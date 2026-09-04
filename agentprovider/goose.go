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
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

const (
	// gooseProvider is the built-in provider goose reaches an OpenAI-compatible
	// endpoint through. It is not the endpoint: OPENAI_HOST and OPENAI_BASE_PATH
	// below are.
	gooseProvider = "openai"
	// gooseChatPath is what goose appends to the host for a provider whose base
	// path it is not told, and so the tail the gateway's own path needs.
	gooseChatPath = "chat/completions"
	// gooseLocalKey stands in for a key the endpoint does not have. goose sends
	// one either way, and the gateway takes any credential from this host.
	gooseLocalKey = "casbin-gateway"
)

// The config.yaml keys Gateway owns. The API key is not among them: goose
// ignores a provider key placed in config.yaml and reads it from the keyring or
// from secrets.yaml, which is why a switch writes that file too.
const (
	gooseActiveKey = "active_provider"
	gooseHostKey   = "OPENAI_HOST"
	goosePathKey   = "OPENAI_BASE_PATH"
	gooseModelKey  = "GOOSE_MODEL"
	gooseSecretKey = "OPENAI_API_KEY"
)

var gooseKeys = []string{gooseActiveKey, gooseHostKey, goosePathKey, gooseModelKey}

// errGooseNoModel rejects a provider goose could not use: it starts on one
// model, and an endpoint it does not know has no catalog to pick one from.
var errGooseNoModel = errors.New("goose needs a model name, so bind a provider that lists at least one model")

type gooseWriter struct{}

func init() {
	register(gooseWriter{})
}

func (gooseWriter) AgentId() string { return "goose" }

func (gooseWriter) Protocol() string { return "openai" }

func (w gooseWriter) Plan(target Target, endpoint Endpoint) ([]File, error) {
	settings, err := w.settings(endpoint)
	if err != nil {
		return nil, err
	}
	configPath, secretsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}

	config, err := yamledit.Render(settings)
	if err != nil {
		return nil, err
	}
	secrets, err := yamledit.Render(map[string]string{
		gooseSecretKey: maskSecret(emptyAs(endpoint.ApiKey, gooseLocalKey)),
	})
	if err != nil {
		return nil, err
	}
	return []File{
		{Path: configPath, Format: "yaml", Preview: config},
		{Path: secretsPath, Format: "yaml", Preview: secrets},
	}, nil
}

func (w gooseWriter) Apply(target Target, endpoint Endpoint) (map[string]string, error) {
	settings, err := w.settings(endpoint)
	if err != nil {
		return nil, err
	}
	configPath, secretsPath, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	document, config, err := readYAMLMapping(configPath)
	if err != nil {
		return nil, err
	}
	secretsDocument, secrets, err := readYAMLMapping(secretsPath)
	if err != nil {
		return nil, err
	}

	previous := map[string]string{}
	for _, key := range gooseKeys {
		if value := yamledit.String(config, key); value != "" {
			previous[key] = value
		}
	}
	// The provider block carries the model goose starts on in newer versions,
	// and the flat GOOSE_MODEL key in the ones before them; both are written so
	// the switch lands whichever the installation reads. What is remembered is
	// the model of the provider that was active, which is not always the one
	// the switch is about to write under.
	if model := yamledit.String(config, "providers", w.activeOf(config), "model"); model != "" {
		previous["providers.model"] = model
	}
	if value := yamledit.String(secrets, gooseSecretKey); value != "" {
		previous[gooseSecretKey] = value
	}

	for key, value := range settings {
		if err := yamledit.Set(config, value, key); err != nil {
			return nil, err
		}
	}
	for key, value := range map[string]any{"enabled": true, "configured": true, "model": endpoint.Model} {
		if err := yamledit.Set(config, value, "providers", gooseProvider, key); err != nil {
			return nil, err
		}
	}
	if err := yamledit.Set(secrets, emptyAs(endpoint.ApiKey, gooseLocalKey), gooseSecretKey); err != nil {
		return nil, err
	}

	changes := &txn{}
	if err := saveYAML(changes, configPath, document, config); err != nil {
		changes.abort()
		return nil, err
	}
	if err := saveYAML(changes, secretsPath, secretsDocument, secrets); err != nil {
		changes.abort()
		return nil, err
	}
	if err := changes.commit(); err != nil {
		return nil, err
	}
	return previous, nil
}

func (w gooseWriter) Restore(target Target, previous map[string]string) error {
	configPath, secretsPath, err := w.paths(target)
	if err != nil {
		return err
	}
	document, config, err := readYAMLMapping(configPath)
	if err != nil {
		return err
	}
	secretsDocument, secrets, err := readYAMLMapping(secretsPath)
	if err != nil {
		return err
	}

	for _, key := range gooseKeys {
		if value, ok := previous[key]; ok {
			if err := yamledit.Set(config, value, key); err != nil {
				return err
			}
			continue
		}
		yamledit.Delete(config, key)
	}
	// Gateway's own provider block goes away, and the model it displaced goes
	// back under whichever provider was active before the switch - which is not
	// always the one Gateway wrote under.
	active := w.activeOf(config)
	yamledit.Delete(config, "providers", gooseProvider)
	if model, ok := previous["providers.model"]; ok {
		if err := yamledit.Set(config, model, "providers", active, "model"); err != nil {
			return err
		}
	}
	if yamledit.IsEmpty(yamledit.Get(config, "providers")) {
		yamledit.Delete(config, "providers")
	}
	if value, ok := previous[gooseSecretKey]; ok {
		if err := yamledit.Set(secrets, value, gooseSecretKey); err != nil {
			return err
		}
	} else {
		yamledit.Delete(secrets, gooseSecretKey)
	}

	changes := &txn{}
	if err := saveYAML(changes, configPath, document, config); err != nil {
		changes.abort()
		return err
	}
	if err := saveYAML(changes, secretsPath, secretsDocument, secrets); err != nil {
		changes.abort()
		return err
	}
	return changes.commit()
}

// Current is the endpoint the two halves in config.yaml spell out, put back
// together as the one URL the rest of Gateway compares against.
func (w gooseWriter) Current(target Target) (string, error) {
	config, err := w.configOf(target)
	if err != nil {
		return "", err
	}
	host := yamledit.String(config, gooseHostKey)
	if host == "" {
		return "", nil
	}
	path := strings.TrimSuffix(yamledit.String(config, goosePathKey), gooseChatPath)
	return strings.TrimSuffix(host, "/") + "/" + strings.Trim(path, "/"), nil
}

func (w gooseWriter) Builtin(target Target, previous map[string]string) string {
	if previous != nil {
		return emptyAs(previous["providers.model"], previous[gooseModelKey])
	}

	config, err := w.configOf(target)
	if err != nil {
		return ""
	}
	// A model beside an endpoint Gateway wrote is Gateway's, not the one goose
	// would pick for itself.
	if strings.Contains(yamledit.String(config, goosePathKey), "/agents/") {
		return ""
	}
	return emptyAs(yamledit.String(config, "providers", w.activeOf(config), "model"), yamledit.String(config, gooseModelKey))
}

// activeOf is the provider goose starts on, which is the OpenAI one for a file
// that names none: that is goose's own default and the one a switch writes.
func (gooseWriter) activeOf(config *yaml.Node) string {
	return emptyAs(yamledit.String(config, gooseActiveKey), gooseProvider)
}

// settings is what a switch writes at the root of config.yaml. goose reaches an
// endpoint through a host and a path rather than one URL, so the gateway's own
// URL is split into the two it wants.
func (gooseWriter) settings(endpoint Endpoint) (map[string]string, error) {
	if endpoint.Model == "" {
		return nil, errGooseNoModel
	}
	parsed, err := url.Parse(endpoint.BaseUrl)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("goose needs an absolute base URL to split into a host and a path")
	}
	return map[string]string{
		gooseActiveKey: gooseProvider,
		gooseHostKey:   parsed.Scheme + "://" + parsed.Host,
		goosePathKey:   strings.Trim(parsed.Path, "/") + "/" + gooseChatPath,
		gooseModelKey:  endpoint.Model,
	}, nil
}

func (w gooseWriter) configOf(target Target) (*yaml.Node, error) {
	configPath, _, err := w.paths(target)
	if err != nil {
		return nil, err
	}
	_, config, err := readYAMLMapping(configPath)
	return config, err
}

// paths are the two files a switch writes: the settings goose reads, and the
// secrets beside them. A keyring goose can reach answers for the key before
// secrets.yaml does, which the switch leaves alone: the gateway takes whatever
// credential the keyring already holds.
func (gooseWriter) paths(target Target) (string, string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, ".config", "goose")
	if runtime.GOOS == "windows" {
		dir = filepath.Join(home, "AppData", "Roaming", "Block", "goose", "config")
	}
	return filepath.Join(dir, "config.yaml"), filepath.Join(dir, "secrets.yaml"), nil
}
