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
	"fmt"
	"net/url"
	"strings"
)

// ImportLinkScheme is CC Switch's. Vendors already publish "add this provider"
// links in this format, so Gateway reads that one rather than inventing a
// second one nobody would be given.
const ImportLinkScheme = "ccswitch"

// providerLinkApps are the values CC Switch accepts in "app". Only the wire
// format matters here, and only Claude speaks Anthropic's.
var providerLinkApps = map[string]string{
	"claude":    "anthropic",
	"codex":     "openai",
	"gemini":    "openai",
	"grokbuild": "openai",
	"opencode":  "openai",
	"openclaw":  "openai",
	"hermes":    "openai",
}

// ParseProviderLink reads a link that has to be a provider, which is what the
// provider pages import. Every other resource is read by ParseImportLink.
func ParseProviderLink(owner string, raw string) (*Provider, error) {
	link, err := ParseImportLink(owner, raw)
	if err != nil {
		return nil, err
	}
	if link.Provider == nil {
		return nil, fmt.Errorf("this link carries a %q, not a provider", link.Resource)
	}
	return link.Provider, nil
}

// providerFromLink fills in what the link says and leaves the rest to the form.
func providerFromLink(owner string, query url.Values) (*Provider, error) {
	app := query.Get("app")
	protocol, known := providerLinkApps[app]
	if !known {
		return nil, fmt.Errorf("unknown app in the link: %q", app)
	}

	name := strings.TrimSpace(query.Get("name"))
	if name == "" {
		return nil, fmt.Errorf("the link does not say what to call this provider")
	}

	provider := &Provider{
		Owner:       owner,
		DisplayName: name,
		Type:        protocol,
		Status:      "enabled",
		AuthMode:    ProviderAuthProvider,
		BaseUrl:     firstEndpoint(query.Get("endpoint")),
		ApiKey:      strings.TrimSpace(query.Get("apiKey")),
		Models:      linkModels(query),
		Notes:       query.Get("notes"),
		Icon:        strings.TrimSpace(query.Get("icon")),
	}
	// The icon field takes a site to read a favicon from, which is what
	// "homepage" is; CC Switch's own "icon" is a name from its icon set.
	if provider.Icon == "" {
		provider.Icon = strings.TrimSpace(query.Get("homepage"))
	}
	return provider, nil
}

// firstEndpoint takes the primary of the comma-separated list the format allows.
func firstEndpoint(endpoint string) string {
	for _, candidate := range strings.Split(endpoint, ",") {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// linkModels collects the model names the link carries. CC Switch names one
// model per Claude tier; here they are all just models this provider serves.
func linkModels(query url.Values) []string {
	models := []string{}
	seen := map[string]bool{}
	for _, key := range []string{"model", "opusModel", "sonnetModel", "haikuModel"} {
		name := strings.TrimSpace(query.Get(key))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		models = append(models, name)
	}
	return models
}

// NewApiChannelType marks the connection info a New API site copies out of its
// channel page. It is JSON rather than a link, and carries only these two
// fields, so it is read where a link is pasted instead of getting a box of its
// own.
const NewApiChannelType = "newapi_channel_conn"

type newApiChannel struct {
	Type string `json:"_type"`
	Key  string `json:"key"`
	Url  string `json:"url"`
}

// providerFromNewApiChannel reads that connection info. The second value says
// whether the text was this format at all, so text that is not falls through to
// being read as a link.
func providerFromNewApiChannel(owner string, raw string) (*Provider, bool, error) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "{") {
		return nil, false, nil
	}

	var channel newApiChannel
	if err := json.Unmarshal([]byte(value), &channel); err != nil {
		return nil, true, fmt.Errorf("this is neither a link nor JSON: %w", err)
	}
	if channel.Type != NewApiChannelType {
		return nil, true, fmt.Errorf("this JSON is not New API connection info: %q", channel.Type)
	}

	site := strings.TrimRight(strings.TrimSpace(channel.Url), "/")
	parsed, err := url.Parse(site)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, true, fmt.Errorf("this connection info does not name a site: %q", channel.Url)
	}
	key := strings.TrimSpace(channel.Key)
	if key == "" {
		return nil, true, fmt.Errorf("this connection info carries no API key")
	}

	return &Provider{
		Owner:       owner,
		DisplayName: strings.TrimPrefix(parsed.Host, "www."),
		// A New API site resells every vendor behind one OpenAI-compatible
		// endpoint, so the models are whatever that one site sells.
		Type:     "custom",
		Status:   "enabled",
		AuthMode: ProviderAuthProvider,
		BaseUrl:  site,
		ApiKey:   key,
		Models:   []string{},
		Icon:     site,
	}, true, nil
}
