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

package agentpatch

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	openclawHookName   = "casbin-gateway-agent-monitor"
	openclawStateDir   = ".openclaw"
	openclawConfigFile = "openclaw.json"
)

//go:embed openclaw_hook.js
var openclawHookHandler string

//go:embed openclaw_hook.md
var openclawHookManifest string

type openclawPatcher struct{}

func init() {
	register(openclawPatcher{})
}

func (openclawPatcher) AgentId() string { return "openclaw" }

func (openclawPatcher) Supported() bool { return true }

func (p openclawPatcher) Patch(target Target) error {
	layout, err := p.layoutOf(target)
	if err != nil {
		return err
	}
	return Apply(target, func(changes *ChangeSet) error {
		if err := changes.MkdirAll(layout.hookDir); err != nil {
			return err
		}
		if err := changes.WriteFile(filepath.Join(layout.hookDir, "HOOK.md"), []byte(openclawHookManifest), 0o644); err != nil {
			return err
		}
		if err := changes.WriteFile(filepath.Join(layout.hookDir, "handler.js"), []byte(renderOpenclawHandler(target)), 0o644); err != nil {
			return err
		}
		return enableOpenclawHook(changes, layout.configPath)
	})
}

func (openclawPatcher) Unpatch(target Target) error {
	return Revert(target)
}

func (p openclawPatcher) Status(target Target) (Status, error) {
	layout, err := p.layoutOf(target)
	if err != nil {
		return Status{}, err
	}
	if _, err := os.Stat(filepath.Join(layout.hookDir, "handler.js")); err != nil {
		if os.IsNotExist(err) {
			return Status{Detail: "OpenClaw hook is not installed"}, nil
		}
		return Status{}, err
	}
	enabled, err := openclawHookEnabled(layout.configPath)
	if err != nil {
		return Status{}, err
	}
	if !enabled {
		return Status{Detail: "OpenClaw hook is installed but disabled"}, nil
	}
	if !IsApplied(target) {
		return Status{Patched: true, Detail: "OpenClaw hook was installed outside Gateway"}, nil
	}
	return Status{Patched: true, Detail: "OpenClaw hook active after gateway restart"}, nil
}

func (openclawPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's audit-only OpenClaw hook.", "Restart the OpenClaw gateway after removing the hook."
	}
	return "Installs an audit-only OpenClaw hook.", "Restart the OpenClaw gateway to load the hook."
}

type openclawLayout struct {
	hookDir    string
	configPath string
}

func (p openclawPatcher) layoutOf(target Target) (openclawLayout, error) {
	home, err := homeOf(target)
	if err != nil {
		return openclawLayout{}, err
	}
	stateDir := filepath.Join(home, openclawStateDir)
	return openclawLayout{
		hookDir:    filepath.Join(stateDir, "hooks", openclawHookName),
		configPath: filepath.Join(stateDir, openclawConfigFile),
	}, nil
}

func renderOpenclawHandler(target Target) string {
	handler := strings.ReplaceAll(openclawHookHandler, "__CASBIN_GATEWAY_RECORDS_URL__", jsonString(recordsURL()))
	handler = strings.ReplaceAll(handler, "__CASBIN_GATEWAY_AGENT_PATH__", jsonString(target.Path))
	return strings.ReplaceAll(handler, "__CASBIN_GATEWAY_OWNER__", jsonString(target.Owner))
}

func enableOpenclawHook(changes *ChangeSet, configPath string) error {
	if err := changes.MkdirAll(filepath.Dir(configPath)); err != nil {
		return err
	}
	config, err := readJSONObject(changes, configPath)
	if err != nil {
		return err
	}
	hooks, err := ensureObject(config, "hooks")
	if err != nil {
		return err
	}
	internal, err := ensureObject(hooks, "internal")
	if err != nil {
		return err
	}
	entries, err := ensureObject(internal, "entries")
	if err != nil {
		return err
	}
	entry, err := ensureObject(entries, openclawHookName)
	if err != nil {
		return err
	}
	internal["enabled"] = true
	entry["enabled"] = true
	updated, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return changes.WriteFile(configPath, append(updated, '\n'), 0o600)
}

func openclawHookEnabled(configPath string) (bool, error) {
	config, _, exists, err := readJSONConfig(configPath)
	if err != nil || !exists {
		return false, err
	}
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return false, nil
	}
	internal, ok := objectAt(hooks["internal"])
	if !ok || internal["enabled"] != true {
		return false, nil
	}
	entries, ok := objectAt(internal["entries"])
	if !ok {
		return false, nil
	}
	entry, ok := objectAt(entries[openclawHookName])
	return ok && entry["enabled"] == true, nil
}

func ensureObject(parent map[string]any, key string) (map[string]any, error) {
	if value, exists := parent[key]; exists {
		object, ok := objectAt(value)
		if !ok {
			return nil, fmt.Errorf("%s must be a JSON object", key)
		}
		return object, nil
	}
	object := map[string]any{}
	parent[key] = object
	return object, nil
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
