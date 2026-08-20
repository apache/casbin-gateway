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
	"os"
	"path/filepath"

	"github.com/apache/casbin-gateway/agentpatch"
)

// codex writes the selected provider into ~/.codex/config.toml. Codex selects
// its upstream with a top-level model_provider key pointing at a table under
// [model_providers]. For a custom provider the only file-based way to supply a
// credential is experimental_bearer_token on that table (env_key merely names
// an environment variable Codex reads at runtime, which Gateway cannot set for
// a separately launched process), so the key is written there directly.
type codex struct{}

func init() { register(codex{}) }

func (codex) AgentID() string { return "codex" }

func (codex) DefaultMode() os.FileMode { return 0o600 }

func (codex) ConfigPath(install Install) (string, error) {
	home, err := agentpatch.HomeOf(install.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func (codex) Render(existing []byte, p Provider) ([]byte, error) {
	doc, err := loadTOML(existing)
	if err != nil {
		return nil, err
	}

	doc["model_provider"] = p.ID

	providers := ensureTOMLTable(doc, "model_providers")
	entry := ensureTOMLTable(providers, p.ID)
	entry["name"] = p.Name
	entry["base_url"] = p.BaseURL
	entry["wire_api"] = "chat"
	// experimental_bearer_token is mutually exclusive with env_key, so any
	// env_key a previous write left is removed. An empty key clears the token
	// rather than persisting an empty string.
	setOrDelete(entry, "experimental_bearer_token", p.APIKey)
	delete(entry, "env_key")

	if model := p.Models[RoleDefault]; model != "" {
		doc["model"] = model
	}

	return marshalTOML(doc)
}
