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

	"github.com/apache/casbin-gateway/agentfile"
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

// Patch registers exactly one MCP server entry and leaves the rest of
// claude_desktop_config.json alone. Claude Desktop rewrites that file whenever
// the user edits their servers, so a whole-file backup could never be restored
// safely; owning a single key keeps Patch and Unpatch reliable regardless.
func (p claudeDesktopPatcher) Patch(target Target) error {
	configPath, err := p.configPath(target)
	if err != nil {
		return err
	}
	entry, err := p.serverEntry(target)
	if err != nil {
		return err
	}

	stateMutex.Lock()
	err = updateClaudeDesktopServers(configPath, func(servers map[string]any) bool {
		servers[claudeDesktopServerName] = entry
		return true
	})
	stateMutex.Unlock()
	if err != nil {
		return err
	}

	if runtime.GOOS != "windows" {
		return nil
	}
	if err := agentmonitor.EnableCoworkMonitor(target.Path, target.Owner); err != nil {
		_ = p.Unpatch(target)
		return err
	}
	return nil
}

func (p claudeDesktopPatcher) Unpatch(target Target) error {
	configPath, err := p.configPath(target)
	if err != nil {
		return err
	}

	stateMutex.Lock()
	err = updateClaudeDesktopServers(configPath, func(servers map[string]any) bool {
		if _, found := servers[claudeDesktopServerName]; !found {
			return false
		}
		delete(servers, claudeDesktopServerName)
		return true
	})
	// Discard any manifest written by an earlier release that backed up the whole
	// file, without restoring it over the user's current configuration.
	discardStateLocked(target)
	stateMutex.Unlock()
	if err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		if err := agentmonitor.DisableCoworkMonitor(target.Path); err != nil {
			return err
		}
	}
	return RevokeIngestToken(target)
}

// updateClaudeDesktopServers applies mutate to the mcpServers object and writes
// the file back only when mutate reports a change.
func updateClaudeDesktopServers(configPath string, mutate func(map[string]any) bool) error {
	return agentfile.UpdateJSON(configPath, func(config map[string]any, _ bool) (agentfile.Action, error) {
		servers, found := config["mcpServers"]
		if !found {
			servers = map[string]any{}
		}
		object, ok := objectAt(servers)
		if !ok {
			return agentfile.Keep, errors.New("mcpServers must be a JSON object")
		}
		if !mutate(object) {
			return agentfile.Keep, nil
		}
		if len(object) == 0 {
			delete(config, "mcpServers")
		} else {
			config["mcpServers"] = object
		}
		return agentfile.Write, nil
	})
}

func (p claudeDesktopPatcher) Status(target Target) (Status, error) {
	configPath, err := p.configPath(target)
	if err != nil {
		return Status{}, err
	}
	config, exists, err := agentfile.ReadJSON(configPath)
	if err != nil {
		return Status{}, err
	}
	if !exists || !hasClaudeDesktopServer(config) {
		return Status{Detail: "MCP recorder is not registered"}, nil
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
	url, err := recordsURL()
	if err != nil {
		return nil, err
	}
	token, err := IssueIngestToken(target)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"command": executable,
		"args": []string{
			mcpserver.Subcommand,
			"--agent", p.AgentId(),
			"--agent-path", target.Path,
			"--user", target.Owner,
			"--records-url", url,
			"--ingest-token", token,
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
