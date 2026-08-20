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

package controllers

import (
	"testing"

	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/object"
)

func TestProviderFromChannel(t *testing.T) {
	channel := &object.Channel{
		Name:        "deepseek",
		DisplayName: "DeepSeek",
		BaseUrl:     "https://api.deepseek.com/anthropic",
		ApiKey:      "sk-test",
		Models:      []string{"deepseek-chat", "deepseek-reasoner"},
	}

	provider := providerFromChannel(channel)
	if provider.ID != "deepseek" || provider.Name != "DeepSeek" {
		t.Errorf("id/name = %q/%q", provider.ID, provider.Name)
	}
	if provider.BaseURL != channel.BaseUrl || provider.APIKey != channel.ApiKey {
		t.Errorf("baseURL/apiKey not carried through: %q/%q", provider.BaseURL, provider.APIKey)
	}
	if provider.Models[agentconfig.RoleDefault] != "deepseek-chat" {
		t.Errorf("default model = %q, want the channel's first model", provider.Models[agentconfig.RoleDefault])
	}
}

func TestProviderFromChannelWithoutModels(t *testing.T) {
	provider := providerFromChannel(&object.Channel{Name: "c", BaseUrl: "https://x.example.com"})
	if provider.Models != nil {
		t.Errorf("Models = %v, want nil when the channel has no models", provider.Models)
	}
}
