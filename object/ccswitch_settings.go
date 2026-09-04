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

package object

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// CC Switch stores a provider as the block it writes into the agent's own
// configuration file, so one of these is read in that agent's format rather
// than in a format of its own. The keys below are what those formats carry an
// endpoint, a credential and a model name under; the first one an entry sets
// wins, which is why the credentials are ordered with the agent's preferred one
// first.
var (
	ccSwitchBaseUrlKeys = []string{
		"ANTHROPIC_BASE_URL", "ANTHROPIC_API_BASE",
		"OPENAI_BASE_URL", "OPENAI_API_BASE",
		"GOOGLE_GEMINI_BASE_URL", "GEMINI_BASE_URL", "GOOGLE_GEMINI_ENDPOINT",
		"XAI_BASE_URL", "CODE_BASE_URL", "BASE_URL",
	}
	ccSwitchApiKeyKeys = []string{
		"ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY",
		"OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"XAI_API_KEY", "API_KEY", "AUTH_TOKEN",
	}
	ccSwitchModelKeys = []string{
		"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL", "ANTHROPIC_SMALL_FAST_MODEL",
		"OPENAI_MODEL", "GEMINI_MODEL", "MODEL",
	}
)

// ccSwitchEndpoint is what one CC Switch entry says: where its requests go,
// what they carry, and which models it was set up for.
type ccSwitchEndpoint struct {
	baseUrl string
	apiKey  string
	models  []string
	// responses records that Codex was told this upstream answers on the OpenAI
	// Responses API, which is what tells a real OpenAI endpoint from the
	// OpenAI-compatible vendors that stop at chat completions.
	responses bool
}

// readCcSwitchSettings reads one entry's stored configuration block. Every
// shape CC Switch writes is tried against it rather than picked by app, so an
// app it grows support for is read as far as it looks like one of them.
func readCcSwitchSettings(raw string) ccSwitchEndpoint {
	endpoint := ccSwitchEndpoint{}

	document := map[string]any{}
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &document) != nil {
		return endpoint
	}

	// Claude and Gemini keep an env block, Codex an auth block beside the TOML
	// its endpoint is in.
	endpoint.readPairs(pairsOf(document["env"]))
	endpoint.readPairs(pairsOf(document["auth"]))
	endpoint.readObject(document)

	switch config := document["config"].(type) {
	case string:
		endpoint.readConfigText(config)
	case map[string]any:
		endpoint.readPairs(pairsOf(config["env"]))
		endpoint.readObject(config)
	}
	if providers, ok := document["provider"].(map[string]any); ok {
		endpoint.readOpencodeProviders(providers)
	}
	return endpoint
}

// readPairs takes what an environment block sets.
func (endpoint *ccSwitchEndpoint) readPairs(env map[string]string) {
	for _, key := range ccSwitchBaseUrlKeys {
		endpoint.setBaseUrl(env[key])
	}
	for _, key := range ccSwitchApiKeyKeys {
		endpoint.setApiKey(env[key])
	}
	for _, key := range ccSwitchModelKeys {
		endpoint.addModel(env[key])
	}
}

// readObject takes the endpoint an app writes at the top of its own file, which
// is how OpenClaw and the older shapes carry one.
func (endpoint *ccSwitchEndpoint) readObject(document map[string]any) {
	for _, key := range []string{"baseUrl", "baseURL", "base_url", "url"} {
		endpoint.setBaseUrl(textOf(document[key]))
	}
	for _, key := range []string{"apiKey", "api_key", "token", "key"} {
		endpoint.setApiKey(textOf(document[key]))
	}
	if models, ok := document["models"].([]any); ok {
		for _, model := range models {
			switch named := model.(type) {
			case string:
				endpoint.addModel(named)
			case map[string]any:
				endpoint.addModel(textOf(named["id"]))
			}
		}
	}
}

// readConfigText reads a whole configuration file kept as text rather than as a
// block. Codex's is TOML; the apps whose own file is JSON keep it as text too.
func (endpoint *ccSwitchEndpoint) readConfigText(text string) {
	nested := map[string]any{}
	if json.Unmarshal([]byte(strings.TrimSpace(text)), &nested) == nil {
		endpoint.readPairs(pairsOf(nested["env"]))
		endpoint.readObject(nested)
		return
	}
	endpoint.readCodexConfig(text)
}

// readCodexConfig reads the config.toml Codex is switched with. The endpoint is
// in the table the file's model_provider names, which is the one CC Switch
// wrote for this entry.
func (endpoint *ccSwitchEndpoint) readCodexConfig(text string) {
	document := map[string]any{}
	if _, err := toml.Decode(text, &document); err != nil {
		return
	}

	providers, _ := document["model_providers"].(map[string]any)
	selected, _ := document["model_provider"].(string)
	// A file naming no provider but describing exactly one means that one.
	if selected == "" && len(providers) == 1 {
		for name := range providers {
			selected = name
		}
	}
	if provider, ok := providers[selected].(map[string]any); ok {
		endpoint.setBaseUrl(textOf(provider["base_url"]))
		endpoint.responses = endpoint.responses || textOf(provider["wire_api"]) == "responses"
	}
	endpoint.addModel(textOf(document["model"]))
}

// readOpencodeProviders reads opencode.json, where every provider is a block of
// its own with the models it serves under it.
func (endpoint *ccSwitchEndpoint) readOpencodeProviders(providers map[string]any) {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		provider, _ := providers[name].(map[string]any)
		if options, ok := provider["options"].(map[string]any); ok {
			endpoint.readObject(options)
		}
		models, _ := provider["models"].(map[string]any)
		ids := make([]string, 0, len(models))
		for id := range models {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			endpoint.addModel(id)
		}
	}
}

func (endpoint *ccSwitchEndpoint) setBaseUrl(value string) {
	if endpoint.baseUrl == "" {
		endpoint.baseUrl = strings.TrimSpace(value)
	}
}

func (endpoint *ccSwitchEndpoint) setApiKey(value string) {
	if endpoint.apiKey == "" {
		endpoint.apiKey = strings.TrimSpace(value)
	}
}

func (endpoint *ccSwitchEndpoint) addModel(value string) {
	name := strings.TrimSpace(value)
	if name == "" || len(name) > maxProviderModelChars || len(endpoint.models) >= maxProviderModels {
		return
	}
	for _, existing := range endpoint.models {
		if existing == name {
			return
		}
	}
	endpoint.models = append(endpoint.models, name)
}

// pairsOf keeps the settings of a block that are plain strings. The rest is
// whatever else the agent's own file holds — Codex keeps a whole OAuth session
// beside its API key — and none of it is a value Gateway stores.
func pairsOf(value any) map[string]string {
	block, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	pairs := map[string]string{}
	for key, held := range block {
		if text, ok := held.(string); ok {
			pairs[key] = text
		}
	}
	return pairs
}

func textOf(value any) string {
	text, _ := value.(string)
	return text
}
