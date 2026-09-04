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

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agentmonitor"
)

const (
	openclawHookName   = "casbin-gateway-agent-monitor"
	openclawStateDir   = ".openclaw"
	openclawConfigFile = "openclaw.json"
)

// openclawPluginId is the plugin that decides a tool call. It is a second
// artifact rather than a change to the hook beside it: the event stream a hook
// listens on is not something a session waits for, and before_tool_call, the
// one seam that is, is only reachable from a plugin.
const openclawPluginId = "casbin-gateway-agent-permissions"

//go:embed openclaw_hook.js
var openclawHookHandler string

//go:embed openclaw_hook.md
var openclawHookManifest string

//go:embed openclaw_plugin.js
var openclawPluginEntry string

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
	token, err := IssueIngestToken(target)
	if err != nil {
		return err
	}
	handler, err := renderOpenclawHandler(target, token)
	if err != nil {
		return err
	}
	plugin, err := renderOpenclawPlugin(token)
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
		if err := changes.WriteFile(filepath.Join(layout.hookDir, "handler.js"), []byte(handler), 0o644); err != nil {
			return err
		}
		if err := writeOpenclawPlugin(changes, layout.pluginDir, plugin); err != nil {
			return err
		}
		return enableOpenclawEntries(changes, layout.configPath)
	})
}

func (openclawPatcher) Unpatch(target Target) error {
	if err := Revert(target); err != nil {
		return err
	}
	return RevokeIngestToken(target)
}

func (p openclawPatcher) Status(target Target) (Status, error) {
	layout, err := p.layoutOf(target)
	if err != nil {
		return Status{}, err
	}
	handler, err := os.ReadFile(filepath.Join(layout.hookDir, "handler.js"))
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Detail: "OpenClaw hook is not installed"}, nil
		}
		return Status{}, err
	}
	current, err := recordsURL()
	if err != nil {
		return Status{}, err
	}
	// A handler left by an older Gateway, or by one listening on another port,
	// reports nowhere. Re-patching is what rewrites it.
	if !strings.Contains(string(handler), jsonString(current)) {
		return Status{Detail: "OpenClaw hook needs refresh"}, nil
	}

	// The plugin beside it is what refuses a tool call, and a patch missing it
	// is the one an earlier Gateway wrote, before there was one.
	decision, err := decisionURL()
	if err != nil {
		return Status{}, err
	}
	entry, err := os.ReadFile(filepath.Join(layout.pluginDir, "index.js"))
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Detail: "OpenClaw permission plugin needs refresh"}, nil
		}
		return Status{}, err
	}
	if !strings.Contains(string(entry), jsonString(decision)) {
		return Status{Detail: "OpenClaw permission plugin needs refresh"}, nil
	}

	enabled, err := openclawEntriesEnabled(layout.configPath)
	if err != nil {
		return Status{}, err
	}
	if !enabled {
		return Status{Detail: "OpenClaw hook and plugin are installed but not both enabled"}, nil
	}
	if !IsApplied(target) {
		return Status{Patched: true, Detail: "OpenClaw hook was installed outside Gateway"}, nil
	}
	return Status{Patched: true, Detail: "OpenClaw hook active after gateway restart"}, nil
}

// Decides: the plugin's before_tool_call handler returns block, which OpenClaw
// treats as terminal and refuses the call on.
func (openclawPatcher) Decides() bool { return true }

func (openclawPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's OpenClaw hook and the plugin that checks each tool call.", "Restart the OpenClaw gateway after removing them."
	}
	return "Installs an OpenClaw hook that observes events, and a plugin that refuses a tool call this agent's permissions do not allow; an agent nobody has restricted is never held up.",
		"Restart the OpenClaw gateway to load them."
}

type openclawLayout struct {
	hookDir    string
	pluginDir  string
	configPath string
}

func (p openclawPatcher) layoutOf(target Target) (openclawLayout, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return openclawLayout{}, err
	}
	stateDir := filepath.Join(home, openclawStateDir)
	return openclawLayout{
		hookDir: filepath.Join(stateDir, "hooks", openclawHookName),
		// "extensions" under the state directory is one of the roots OpenClaw
		// discovers plugins from, so nothing has to be added to load.paths.
		pluginDir:  filepath.Join(stateDir, "extensions", openclawPluginId),
		configPath: filepath.Join(stateDir, openclawConfigFile),
	}, nil
}

func renderOpenclawHandler(target Target, ingestToken string) (string, error) {
	url, err := recordsURL()
	if err != nil {
		return "", err
	}
	replacements := map[string]string{
		"__CASBIN_GATEWAY_RECORDS_URL__":         jsonString(url),
		"__CASBIN_GATEWAY_AGENT_PATH__":          jsonString(target.Path),
		"__CASBIN_GATEWAY_OWNER__":               jsonString(target.Owner),
		"__CASBIN_GATEWAY_INGEST_TOKEN__":        jsonString(ingestToken),
		"__CASBIN_GATEWAY_INGEST_TOKEN_HEADER__": jsonString(agentmonitor.IngestTokenHeader),
	}
	handler := openclawHookHandler
	for placeholder, value := range replacements {
		handler = strings.ReplaceAll(handler, placeholder, value)
	}
	return handler, nil
}

// renderOpenclawPlugin fills in the plugin entry. It carries no records URL:
// the hook beside it does the reporting, and this one only asks.
func renderOpenclawPlugin(ingestToken string) (string, error) {
	url, err := decisionURL()
	if err != nil {
		return "", err
	}
	replacements := map[string]string{
		"__CASBIN_GATEWAY_PLUGIN_ID__":           jsonString(openclawPluginId),
		"__CASBIN_GATEWAY_AGENT__":               jsonString("openclaw"),
		"__CASBIN_GATEWAY_DECISION_URL__":        jsonString(url),
		"__CASBIN_GATEWAY_INGEST_TOKEN__":        jsonString(ingestToken),
		"__CASBIN_GATEWAY_INGEST_TOKEN_HEADER__": jsonString(agentmonitor.IngestTokenHeader),
	}
	entry := openclawPluginEntry
	for placeholder, value := range replacements {
		entry = strings.ReplaceAll(entry, placeholder, value)
	}
	return entry, nil
}

// writeOpenclawPlugin lays out the three files OpenClaw reads a plugin from:
// the manifest it identifies it by, the package.json naming the entry module,
// and the module itself.
func writeOpenclawPlugin(changes *ChangeSet, pluginDir string, entry string) error {
	if err := changes.MkdirAll(pluginDir); err != nil {
		return err
	}
	manifest, err := json.MarshalIndent(map[string]any{
		"id":          openclawPluginId,
		"name":        "Casbin Gateway Agent Permissions",
		"description": "Refuses a tool call the agent's Casbin Gateway permissions do not allow",
	}, "", "  ")
	if err != nil {
		return err
	}
	pkg, err := json.MarshalIndent(map[string]any{
		"name":    openclawPluginId,
		"version": "1.0.0",
		"private": true,
		"type":    "module",
		// Discovery refuses a package that does not name its entry here.
		"openclaw": map[string]any{"extensions": []string{"./index.js"}},
	}, "", "  ")
	if err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"openclaw.plugin.json": append(manifest, '\n'),
		"package.json":         append(pkg, '\n'),
		"index.js":             []byte(entry),
	} {
		if err := changes.WriteFile(filepath.Join(pluginDir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// enableOpenclawEntries turns on both of Gateway's artifacts in one write: the
// audit hook and the plugin that decides a tool call.
func enableOpenclawEntries(changes *ChangeSet, configPath string) error {
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
	hookEntries, err := ensureObject(internal, "entries")
	if err != nil {
		return err
	}
	hookEntry, err := ensureObject(hookEntries, openclawHookName)
	if err != nil {
		return err
	}
	internal["enabled"] = true
	hookEntry["enabled"] = true

	plugins, err := ensureObject(config, "plugins")
	if err != nil {
		return err
	}
	pluginEntries, err := ensureObject(plugins, "entries")
	if err != nil {
		return err
	}
	pluginEntry, err := ensureObject(pluginEntries, openclawPluginId)
	if err != nil {
		return err
	}
	pluginEntry["enabled"] = true

	updated, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return changes.WriteFile(configPath, append(updated, '\n'), 0o600)
}

// openclawEntriesEnabled reports that both artifacts are switched on. Either
// one off is a half-installed patch, and saying so is what offers the repair.
func openclawEntriesEnabled(configPath string) (bool, error) {
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
	if !ok || entry["enabled"] != true {
		return false, nil
	}

	plugins, ok := objectAt(config["plugins"])
	if !ok {
		return false, nil
	}
	pluginEntries, ok := objectAt(plugins["entries"])
	if !ok {
		return false, nil
	}
	pluginEntry, ok := objectAt(pluginEntries[openclawPluginId])
	return ok && pluginEntry["enabled"] == true, nil
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
