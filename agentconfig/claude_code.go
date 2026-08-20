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

// claudeCode writes the selected provider into ~/.claude/settings.json. Claude
// Code reads its upstream and credentials from the "env" object there, so a
// switch is a matter of setting a handful of ANTHROPIC_* keys and leaving the
// rest of the file (hooks, permissions, other env) untouched.
type claudeCode struct{}

func init() { register(claudeCode{}) }

func (claudeCode) AgentID() string { return "claude-code" }

func (claudeCode) DefaultMode() os.FileMode { return 0o600 }

func (claudeCode) ConfigPath(install Install) (string, error) {
	home, err := agentpatch.HomeOf(install.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func (claudeCode) Render(existing []byte, p Provider) ([]byte, error) {
	doc, err := loadJSONObject(existing)
	if err != nil {
		return nil, err
	}

	env := ensureObject(doc, "env")
	setOrDelete(env, "ANTHROPIC_BASE_URL", p.BaseURL)
	setOrDelete(env, "ANTHROPIC_AUTH_TOKEN", p.APIKey)
	setOrDelete(env, "ANTHROPIC_DEFAULT_OPUS_MODEL", p.Models[RoleOpus])
	setOrDelete(env, "ANTHROPIC_DEFAULT_SONNET_MODEL", p.Models[RoleSonnet])
	setOrDelete(env, "ANTHROPIC_DEFAULT_HAIKU_MODEL", p.Models[RoleHaiku])

	// TODO(pr1-followup): pass Provider.Extra through into env, refusing to
	// overwrite the owned keys above so a provider preset cannot silently change
	// the base URL or token from a stray Extra entry.

	// An emptied env object is left in place rather than removed: Claude Code
	// treats a missing and an empty "env" the same, and keeping it avoids a
	// spurious diff when every owned key was cleared.
	return marshalJSONObject(doc)
}
