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
	"encoding/json"
	"testing"
)

func TestClaudeCodeRenderCreatesEnv(t *testing.T) {
	out, err := claudeCode{}.Render(nil, Provider{
		BaseURL: "https://api.deepseek.com/anthropic",
		APIKey:  "sk-test",
		Models:  map[Role]string{RoleOpus: "deepseek-v4-pro"},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	env := decodeEnv(t, out)
	if env["ANTHROPIC_BASE_URL"] != "https://api.deepseek.com/anthropic" {
		t.Errorf("ANTHROPIC_BASE_URL = %q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-test" {
		t.Errorf("ANTHROPIC_AUTH_TOKEN = %q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "deepseek-v4-pro" {
		t.Errorf("ANTHROPIC_DEFAULT_OPUS_MODEL = %q", env["ANTHROPIC_DEFAULT_OPUS_MODEL"])
	}
}

func TestClaudeCodeRenderPreservesUnrelatedKeys(t *testing.T) {
	existing := []byte(`{
  "hooks": {"Stop": [{"hooks": []}]},
  "env": {"MY_OWN": "keep", "ANTHROPIC_BASE_URL": "https://old.example.com"}
}`)

	out, err := claudeCode{}.Render(existing, Provider{BaseURL: "https://new.example.com"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := doc["hooks"]; !ok {
		t.Error("hooks was dropped; unrelated keys must be preserved")
	}
	env, _ := doc["env"].(map[string]any)
	if env["MY_OWN"] != "keep" {
		t.Errorf("unrelated env key MY_OWN = %v, want kept", env["MY_OWN"])
	}
	if env["ANTHROPIC_BASE_URL"] != "https://new.example.com" {
		t.Errorf("ANTHROPIC_BASE_URL = %v, want overwritten", env["ANTHROPIC_BASE_URL"])
	}
}

func TestClaudeCodeRenderClearsUnsetFields(t *testing.T) {
	existing := []byte(`{"env": {"ANTHROPIC_AUTH_TOKEN": "sk-stale"}}`)

	out, err := claudeCode{}.Render(existing, Provider{BaseURL: "https://new.example.com"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	env := decodeEnv(t, out)
	if _, ok := env["ANTHROPIC_AUTH_TOKEN"]; ok {
		t.Error("stale ANTHROPIC_AUTH_TOKEN was not cleared for an empty provider APIKey")
	}
}

func decodeEnv(t *testing.T, data []byte) map[string]string {
	t.Helper()
	var doc struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return doc.Env
}
