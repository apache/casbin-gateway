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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/mcpserver"
)

const claudeDesktopServerName = "casbin-gateway-agent-monitor"

type claudeDesktopPatcher struct{}

func init() {
	register(claudeDesktopPatcher{})
}

func (claudeDesktopPatcher) AgentId() string { return "claude-desktop" }

func (claudeDesktopPatcher) Supported() bool { return true }

func (p claudeDesktopPatcher) Patch(target Target) error {
	configPath, err := p.configPath(target)
	if err != nil {
		return err
	}
	entry, err := p.serverEntry(target)
	if err != nil {
		return err
	}
	if err := Apply(target, func(changes *ChangeSet) error {
		if err := changes.MkdirAll(filepath.Dir(configPath)); err != nil {
			return err
		}
		config, err := readJSONObject(changes, configPath)
		if err != nil {
			return err
		}
		servers, exists := config["mcpServers"]
		if !exists {
			servers = map[string]any{}
			config["mcpServers"] = servers
		}
		object, ok := objectAt(servers)
		if !ok {
			return errors.New("mcpServers must be a JSON object")
		}
		object[claudeDesktopServerName] = entry
		updated, err := json.MarshalIndent(config, "", "  ")
		if err != nil {
			return err
		}
		return changes.WriteFile(configPath, append(updated, '\n'), 0o600)
	}); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return nil
	}
	if err := agentmonitor.EnableCoworkMonitor(target.Path, target.Owner); err != nil {
		_ = Revert(target)
		return err
	}
	return nil
}

func (claudeDesktopPatcher) Unpatch(target Target) error {
	if err := Revert(target); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return agentmonitor.DisableCoworkMonitor(target.Path)
	}
	return nil
}

func (p claudeDesktopPatcher) Status(target Target) (Status, error) {
	configPath, err := p.configPath(target)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(configPath)
	if err != nil {
		return Status{}, err
	}
	if !exists || !hasClaudeDesktopServer(config) {
		return Status{Detail: "MCP recorder is not registered"}, nil
	}
	if !IsApplied(target) {
		return Status{Patched: true, Detail: "MCP recorder is registered outside Gateway"}, nil
	}
	if runtime.GOOS != "windows" {
		return Status{Patched: true, Detail: "MCP recorder registered; restart Claude Desktop to apply it"}, nil
	}
	active, detail := agentmonitor.CoworkMonitorStatus(target.Path)
	return Status{Patched: active, Detail: "MCP recorder registered; " + detail}, nil
}

func (p claudeDesktopPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's Claude Desktop MCP recorder.", "Restart Claude Desktop after removing the recorder."
	}
	return "Registers Gateway's audit-only MCP recorder with Claude Desktop.", "Restart Claude Desktop to load the recorder."
}

func (p claudeDesktopPatcher) configPath(target Target) (string, error) {
	home, err := homeOf(target)
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	default:
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
	}
}

func (p claudeDesktopPatcher) serverEntry(target Target) (map[string]any, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve Gateway executable: %w", err)
	}
	return map[string]any{
		"command": executable,
		"args": []string{
			mcpserver.Subcommand,
			"--agent", p.AgentId(),
			"--agent-path", target.Path,
			"--user", target.Owner,
			"--records-url", recordsURL(),
		},
	}, nil
}

func hasClaudeDesktopServer(config map[string]any) bool {
	servers, ok := objectAt(config["mcpServers"])
	if !ok {
		return false
	}
	entry, ok := objectAt(servers[claudeDesktopServerName])
	if !ok {
		return false
	}
	args := stringArray(entry["args"])
	return stringIndex(args, mcpserver.Subcommand) >= 0 && strings.Contains(strings.Join(args, " "), "--records-url")
}

func readJSONObject(changes *ChangeSet, path string) (map[string]any, error) {
	data, err := changes.ReadFile(path)
	if err != nil {
		return nil, err
	}
	config := map[string]any{}
	if strings.TrimSpace(string(data)) == "" {
		return config, nil
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if config == nil {
		return nil, fmt.Errorf("parse %s: root must be a JSON object", path)
	}
	return config, nil
}
