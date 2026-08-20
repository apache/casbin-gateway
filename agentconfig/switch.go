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

// Package agentconfig writes a selected upstream provider into the native
// configuration file of each supported CLI agent (Claude Code, Codex, ...),
// atomically and idempotently. Each agent's format mapping is a pure Adapter;
// this file owns reading, the one-time backup and the atomic write, so an
// Adapter never touches the filesystem itself.
package agentconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

// Install identifies one discovered CLI agent installation to write into. The
// fields mirror what agentpatch already resolves during discovery; they are
// kept local so this package does not depend on the monitoring code path.
type Install struct {
	AgentID string
	Path    string
	Owner   string
}

// Role selects which model slot a provider model maps to. Adapters that have no
// notion of roles ignore the ones they do not use.
type Role string

const (
	RoleDefault Role = "default"
	RoleOpus    Role = "opus"
	RoleSonnet  Role = "sonnet"
	RoleHaiku   Role = "haiku"
)

// Provider is the upstream selection to write into an agent's own config, in
// whatever format that agent expects. Adapters map these neutral fields onto
// the agent's schema; an empty field is left to the adapter's default handling
// (typically: remove the key it owns).
type Provider struct {
	ID      string            // stable id, e.g. the channel name
	Name    string            // display name
	BaseURL string            // upstream base URL
	APIKey  string            // secret; the adapter decides file vs env placement
	Models  map[Role]string   // optional per-role model overrides
	Extra   map[string]string // optional adapter-specific passthrough
}

// Adapter is one agent's format mapping. Render is a pure transform: given the
// current file bytes it returns the new bytes with only the adapter's own
// fields rewritten and everything else preserved. It performs no I/O.
type Adapter interface {
	AgentID() string
	// ConfigPath returns the agent's own config file for this install. It may
	// read the environment and account home, but must not write.
	ConfigPath(install Install) (string, error)
	// Render returns the updated file contents. existing is nil when the config
	// file does not exist yet.
	Render(existing []byte, p Provider) ([]byte, error)
	// DefaultMode is the permission for a freshly created config file.
	DefaultMode() os.FileMode
}

// Result describes what Switch did so callers can distinguish a real write from
// an idempotent no-op.
type Result struct {
	AgentID    string
	ConfigPath string
	Changed    bool // false when the config already matched the provider
	BackedUp   bool // true when a one-time pre-Gateway backup was created
}

var adapters = map[string]Adapter{}

func register(a Adapter) { adapters[a.AgentID()] = a }

// SupportedAgents reports the agent ids that can be switched.
func SupportedAgents() []string {
	ids := make([]string, 0, len(adapters))
	for id := range adapters {
		ids = append(ids, id)
	}
	return ids
}

// Supports reports whether agentID has a registered adapter, i.e. whether
// Switch can write a provider into that agent's config.
func Supports(agentID string) bool {
	_, ok := adapters[agentID]
	return ok
}

// Switch writes provider p into install's config file, atomically and
// idempotently: switching again to the same provider is a no-op, and switching
// to a different one rewrites only the adapter-owned fields while preserving
// every unrelated user setting.
func Switch(install Install, p Provider) (Result, error) {
	adapter, err := adapterFor(install.AgentID)
	if err != nil {
		return Result{}, err
	}

	path, err := adapter.ConfigPath(install)
	if err != nil {
		return Result{}, err
	}

	existing, err := readFileOrNil(path)
	if err != nil {
		return Result{}, err
	}

	updated, err := adapter.Render(existing, p)
	if err != nil {
		return Result{}, err
	}

	result := Result{AgentID: install.AgentID, ConfigPath: path}
	if existing != nil && bytes.Equal(existing, updated) {
		return result, nil // already selected; nothing to write
	}

	backedUp, err := backupOnce(path, existing)
	if err != nil {
		return Result{}, err
	}
	result.BackedUp = backedUp

	mode := adapter.DefaultMode()
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := writeFileAtomic(path, updated, mode); err != nil {
		return Result{}, err
	}
	result.Changed = true
	return result, nil
}

func adapterFor(id string) (Adapter, error) {
	if id == "" {
		return nil, errors.New("agentId is required")
	}
	adapter, ok := adapters[id]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", id)
	}
	return adapter, nil
}

func readFileOrNil(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// backupSuffix names the sidecar copy of the config as it was before Gateway
// first wrote to it, so a later "stop managing" step can restore the original.
const backupSuffix = ".casbin-orig"

// backupOnce saves the pre-Gateway file exactly once. It is a no-op when
// Gateway is creating the file (nothing to preserve) or when a backup already
// exists (the current content is Gateway's own, not the user's original).
func backupOnce(path string, existing []byte) (bool, error) {
	if existing == nil {
		return false, nil
	}
	backup := path + backupSuffix
	if _, err := os.Stat(backup); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := writeFileAtomic(backup, existing, 0o600); err != nil {
		return false, err
	}
	return true, nil
}
