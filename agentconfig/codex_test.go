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

package agentconfig

import (
	"testing"

	toml "github.com/pelletier/go-toml/v2"
)

func TestCodexRenderSelectsProvider(t *testing.T) {
	out, err := codex{}.Render(nil, Provider{
		ID:      "deepseek",
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com/v1",
		APIKey:  "sk-secret",
		Models:  map[Role]string{RoleDefault: "deepseek-chat"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc := decodeTOML(t, out)
	if doc["model_provider"] != "deepseek" {
		t.Errorf("model_provider = %v, want deepseek", doc["model_provider"])
	}
	if doc["model"] != "deepseek-chat" {
		t.Errorf("model = %v, want deepseek-chat", doc["model"])
	}
	entry := providerEntry(t, doc, "deepseek")
	if entry["base_url"] != "https://api.deepseek.com/v1" {
		t.Errorf("base_url = %v", entry["base_url"])
	}
	if entry["experimental_bearer_token"] != "sk-secret" {
		t.Errorf("experimental_bearer_token = %v, want the provider key", entry["experimental_bearer_token"])
	}
}

func TestCodexRenderReplacesEnvKeyWithBearerToken(t *testing.T) {
	// A config previously written with env_key must not keep it, since Codex
	// treats env_key and experimental_bearer_token as mutually exclusive.
	existing := []byte("model_provider = \"deepseek\"\n\n[model_providers.deepseek]\nname = \"DeepSeek\"\nbase_url = \"https://api.deepseek.com/v1\"\nenv_key = \"CASBIN_DEEPSEEK_API_KEY\"\n")

	out, err := codex{}.Render(existing, Provider{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1", APIKey: "sk-secret"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	entry := providerEntry(t, decodeTOML(t, out), "deepseek")
	if _, ok := entry["env_key"]; ok {
		t.Error("env_key was kept alongside experimental_bearer_token")
	}
	if entry["experimental_bearer_token"] != "sk-secret" {
		t.Errorf("experimental_bearer_token = %v", entry["experimental_bearer_token"])
	}
}

func TestCodexRenderClearsTokenWhenKeyEmpty(t *testing.T) {
	existing := []byte("[model_providers.deepseek]\nexperimental_bearer_token = \"sk-old\"\n")

	out, err := codex{}.Render(existing, Provider{ID: "deepseek", Name: "DeepSeek", BaseURL: "https://api.deepseek.com/v1"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	entry := providerEntry(t, decodeTOML(t, out), "deepseek")
	if _, ok := entry["experimental_bearer_token"]; ok {
		t.Error("stale experimental_bearer_token was not cleared for an empty key")
	}
}

func TestCodexRenderPreservesUnrelatedTables(t *testing.T) {
	existing := []byte("model_provider = \"old\"\n\n[tui]\ntheme = \"dark\"\n\n[model_providers.old]\nbase_url = \"https://old.example.com\"\n")

	out, err := codex{}.Render(existing, Provider{ID: "new", Name: "New", BaseURL: "https://new.example.com"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	doc := decodeTOML(t, out)
	if doc["model_provider"] != "new" {
		t.Errorf("model_provider = %v, want new", doc["model_provider"])
	}
	tui, ok := doc["tui"].(map[string]any)
	if !ok || tui["theme"] != "dark" {
		t.Errorf("unrelated [tui] table not preserved: %v", doc["tui"])
	}
	// The previously selected provider's table is left in place; only the
	// pointer moved.
	if _, ok := providerTable(doc)["old"]; !ok {
		t.Error("previous provider table was dropped")
	}
}

func decodeTOML(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal toml: %v", err)
	}
	return doc
}

func providerTable(doc map[string]any) map[string]any {
	providers, _ := doc["model_providers"].(map[string]any)
	return providers
}

func providerEntry(t *testing.T, doc map[string]any, id string) map[string]any {
	t.Helper()
	entry, ok := providerTable(doc)[id].(map[string]any)
	if !ok {
		t.Fatalf("provider entry %q missing", id)
	}
	return entry
}
