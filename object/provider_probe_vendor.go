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

// What each vendor documents about itself, which is the only thing a check is
// entitled to hold it to. An endpoint or a model name that matches nothing here
// is not a finding: it is a question this build cannot ask.

package object

import (
	"net/url"
	"strings"
)

type probeVendor struct {
	key string
	// hosts are the vendor's own API hosts. A reseller answers on its own host
	// and so matches none of them, which is the point.
	hosts []string
	// models are the prefixes of the model names this vendor serves.
	models []string
	// aliases are names the vendor documents as pointing at whichever model is
	// current. Asking for one and being answered with a different name is what
	// the vendor says will happen, so it is not evidence of anything.
	aliases []string
	// headers are the response headers this vendor's own API sets. Empty means
	// this build does not document them, which leaves the header case
	// unmeasured rather than failed.
	headers []string
}

// The entries below were each read off the vendor's own API reference. A vendor
// is here for what it documents, not for being trusted: an official endpoint
// scores no better for being official.
var probeVendors = []probeVendor{
	{
		key:     "openai",
		hosts:   []string{"api.openai.com"},
		models:  []string{"gpt-", "chatgpt-", "o1", "o3", "o4", "codex-", "text-embedding-"},
		aliases: []string{"chatgpt-4o-latest"},
		headers: []string{
			"X-Request-Id",
			"Openai-Organization",
			"Openai-Processing-Ms",
			"Openai-Version",
			"X-Ratelimit-Limit-Requests",
		},
	},
	{
		key:    "anthropic",
		hosts:  []string{"api.anthropic.com"},
		models: []string{"claude-"},
		headers: []string{
			"Request-Id",
			"Anthropic-Organization-Id",
			"Anthropic-Ratelimit-Requests-Limit",
			"Anthropic-Ratelimit-Input-Tokens-Limit",
			"X-Should-Retry",
		},
	},
	{
		key:   "deepseek",
		hosts: []string{"api.deepseek.com"},
		// "deepseek-chat" is the non-thinking model of the day and
		// "deepseek-reasoner" the thinking one; which model that is changes
		// with every release, and the answer carries the name it changed to.
		models:  []string{"deepseek"},
		aliases: []string{"deepseek-chat", "deepseek-reasoner"},
	},
	{
		key:     "moonshot",
		hosts:   []string{"api.moonshot.cn", "api.moonshot.ai"},
		models:  []string{"kimi", "moonshot-"},
		aliases: []string{"kimi-latest"},
	},
	{
		key:    "zhipu",
		hosts:  []string{"open.bigmodel.cn"},
		models: []string{"glm-", "charglm-", "codegeex-"},
	},
	{
		key:    "qwen",
		hosts:  []string{"dashscope.aliyuncs.com", "dashscope-intl.aliyuncs.com"},
		models: []string{"qwen", "qwq", "qvq"},
	},
	{
		key:    "xai",
		hosts:  []string{"api.x.ai"},
		models: []string{"grok-"},
	},
	{
		key:    "google",
		hosts:  []string{"generativelanguage.googleapis.com"},
		models: []string{"gemini-", "gemma-"},
	},
	{
		key:   "mistral",
		hosts: []string{"api.mistral.ai"},
		models: []string{
			"mistral-", "ministral-", "magistral-", "codestral-", "devstral-", "pixtral-", "open-mistral-",
		},
	},
	{
		key:    "minimax",
		hosts:  []string{"api.minimaxi.com", "api.minimax.chat", "api.minimaxi.chat"},
		models: []string{"minimax-", "abab"},
	},
	{
		key:    "stepfun",
		hosts:  []string{"api.stepfun.com", "api.stepfun.ai"},
		models: []string{"step-"},
	},
}

// probeVendorOfProvider is the vendor whose own endpoint this provider points
// at. Anything else — a reseller, an aggregator, a private deployment — is nil:
// its answers are its own, not a vendor's to be checked against.
func probeVendorOfProvider(provider *Provider) *probeVendor {
	parsed, err := url.Parse(strings.TrimSpace(provider.BaseUrl))
	if err != nil {
		return nil
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return nil
	}
	for index := range probeVendors {
		for _, candidate := range probeVendors[index].hosts {
			if host == candidate || strings.HasSuffix(host, "."+candidate) {
				return &probeVendors[index]
			}
		}
	}
	return nil
}

// probeVendorOfModel is the vendor a model name belongs to, which is a separate
// question from whose endpoint served it: a reseller passing DeepSeek through
// is still answering with DeepSeek's model names.
func probeVendorOfModel(name string) *probeVendor {
	model := probeModelName(name)
	if model == "" {
		return nil
	}
	for index := range probeVendors {
		for _, prefix := range probeVendors[index].models {
			if strings.HasPrefix(model, prefix) {
				return &probeVendors[index]
			}
		}
	}
	return nil
}

// isProbeModelAlias reports whether this name is one the vendor documents as
// moving. "-latest" says so in the name itself; the rest are listed above.
func isProbeModelAlias(name string) bool {
	model := probeModelName(name)
	if model == "" {
		return false
	}
	if strings.HasSuffix(model, "-latest") {
		return true
	}

	vendor := probeVendorOfModel(model)
	if vendor == nil {
		return false
	}
	for _, alias := range vendor.aliases {
		if model == alias {
			return true
		}
	}
	return false
}

// sameModelVendor reports whether two names come out of the same catalogue: the
// vendor that owns both prefixes where one is known, and the leading segment of
// the name otherwise, which is how the vendors that are not listed above still
// name their own models.
func sameModelVendor(asked string, answered string) bool {
	left, right := probeModelName(asked), probeModelName(answered)
	if left == "" || right == "" {
		return false
	}

	if vendor := probeVendorOfModel(left); vendor != nil {
		return probeVendorOfModel(right) == vendor
	}
	return probeModelSegment(left) == probeModelSegment(right)
}

// probeModelName is the model as the vendor names it, with the vendor prefix an
// aggregator adds ("deepseek/deepseek-chat") taken off.
func probeModelName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}
	return name
}

func probeModelSegment(name string) string {
	if index := strings.Index(name, "-"); index > 0 {
		return name[:index]
	}
	return name
}
