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
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/protocol"
)

type NativeProvider struct {
	DisplayName         string `json:"displayName"`
	BaseUrl             string `json:"baseUrl"`
	Protocol            string `json:"protocol"`
	Model               string `json:"model"`
	AnthropicAuthHeader string `json:"anthropicAuthHeader,omitempty"`
	ApiKey              string `json:"-"`
}

// ReadNativeProvider reads only the credential associated with the selected upstream.
func ReadNativeProvider(target Target) (*NativeProvider, error) {
	saved, err := loadState(target)
	if err != nil || saved != nil {
		return nil, errors.New("restore the agent's original configuration before importing it")
	}
	var candidate *NativeProvider
	switch target.AgentId {
	case "claude-code":
		env, err := (claudeCodeWriter{}).envOf(target)
		if err != nil {
			return nil, errors.New("could not read Claude Code settings.json")
		}
		if stringAt(env, "ANTHROPIC_CUSTOM_HEADERS") != "" {
			return nil, errors.New("a Claude provider with custom headers must be added manually")
		}
		candidate = &NativeProvider{
			Protocol:            protocol.Anthropic,
			BaseUrl:             stringAt(env, "ANTHROPIC_BASE_URL"),
			Model:               stringAt(env, "ANTHROPIC_MODEL"),
			ApiKey:              stringAt(env, "ANTHROPIC_API_KEY"),
			AnthropicAuthHeader: "X-Api-Key",
		}
		if token := stringAt(env, "ANTHROPIC_AUTH_TOKEN"); token != "" {
			if candidate.ApiKey != "" {
				return nil, errors.New("choose one Claude authentication field before importing")
			}
			candidate.ApiKey = token
			candidate.AnthropicAuthHeader = "Authorization"
		}
	case "codex-cli":
		candidate, err = readCodexNativeProvider(target)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("this installation does not support native provider import")
	}

	candidate.ApiKey = strings.TrimSpace(candidate.ApiKey)
	if candidate.ApiKey == "" || strings.HasPrefix(candidate.ApiKey, "$") || strings.HasPrefix(candidate.ApiKey, "sk-ant-oat") || strings.ContainsAny(candidate.ApiKey, "\r\n") {
		return nil, errors.New("a literal API key for the selected upstream is required")
	}
	endpoint, err := url.Parse(strings.TrimSpace(candidate.BaseUrl))
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, errors.New("a custom HTTP(S) base URL without embedded credentials or a query is required")
	}
	if strings.Contains(candidate.BaseUrl, candidate.ApiKey) || strings.Contains(endpoint.Path, candidate.ApiKey) || strings.Contains(candidate.Model, candidate.ApiKey) || strings.Contains(candidate.DisplayName, candidate.ApiKey) {
		return nil, errors.New("the provider metadata contains its credential")
	}
	host := endpoint.Hostname()
	ip := net.ParseIP(host)
	if strings.Contains(path.Clean(endpoint.Path)+"/", "/v1/agents/") || candidate.ApiKey == conf.GetRelayToken() ||
		(endpoint.Port() == strconv.Itoa(conf.GetHttpPort()) && (strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") || ip.IsLoopback() || ip.IsUnspecified())) {
		return nil, errors.New("the configuration points to Gateway")
	}
	candidate.BaseUrl = endpoint.String()
	if candidate.DisplayName == "" {
		candidate.DisplayName = endpoint.Host
	}
	return candidate, nil
}

func readCodexNativeProvider(target Target) (*NativeProvider, error) {
	writer := codexWriter{id: target.AgentId}
	doc, err := writer.document(target)
	if err != nil {
		return nil, errors.New("could not read Codex config.toml")
	}
	selected := stringAt(doc, codexModelProviderKey)
	model := stringAt(doc, codexModelKey)
	if name := stringAt(doc, "profile"); name != "" {
		profile := objectAt(objectAt(doc, "profiles"), name)
		if profile == nil {
			return nil, errors.New("could not read the selected Codex profile")
		}
		if value := stringAt(profile, codexModelProviderKey); value != "" {
			selected = value
		}
		if value := stringAt(profile, codexModelKey); value != "" {
			model = value
		}
	}
	if selected == "" || selected == codexProviderName {
		return nil, errors.New("choose a custom upstream in Codex before importing")
	}
	provider := objectAt(objectAt(doc, "model_providers"), selected)
	candidate := &NativeProvider{
		DisplayName: stringAt(provider, "name"),
		BaseUrl:     stringAt(provider, "base_url"),
		Model:       model,
		Protocol:    protocol.Responses,
	}
	switch stringAt(provider, "wire_api") {
	case "", "responses":
	case "chat":
		candidate.Protocol = protocol.OpenAi
	default:
		return nil, errors.New("unsupported Codex wire API")
	}
	if stringAt(provider, "env_key") != "" || len(objectAt(provider, "env_http_headers")) > 0 || len(objectAt(provider, "query_params")) > 0 || provider["auth"] != nil {
		return nil, errors.New("a provider using environment credentials, an auth command or custom query parameters must be added manually")
	}
	candidate.ApiKey = stringAt(provider, "experimental_bearer_token")
	for name, raw := range objectAt(provider, "http_headers") {
		value, _ := raw.(string)
		scheme, token, ok := strings.Cut(value, " ")
		if !strings.EqualFold(name, "Authorization") || !ok || !strings.EqualFold(scheme, "Bearer") || (candidate.ApiKey != "" && candidate.ApiKey != token) {
			return nil, errors.New("this Codex provider uses custom HTTP headers and must be added manually")
		}
		candidate.ApiKey = token
	}
	// auth.json is global to this Codex home. Only an upstream explicitly
	// using OpenAI authentication can take a credential from it.
	usesOpenAiAuth, _ := provider[codexOpenAiAuthKey].(bool)
	if candidate.ApiKey == "" && usesOpenAiAuth {
		_, authPath, err := writer.paths(target)
		if err != nil {
			return nil, errors.New("could not locate Codex auth.json")
		}
		auth, _, err := readJSON(authPath)
		if err != nil {
			return nil, errors.New("could not read Codex auth.json")
		}
		mode := stringAt(auth, "auth_mode")
		if auth["tokens"] == nil && (mode == "" || strings.EqualFold(mode, "apikey")) {
			candidate.ApiKey = stringAt(auth, codexAuthKey)
		}
	}
	return candidate, nil
}
