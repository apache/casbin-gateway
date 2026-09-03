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
	"fmt"
	"os"
	"path/filepath"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agenthook"
)

const claudeCodeHookTimeoutSeconds = 5

var claudeCodeHookEvents = []string{
	"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"PostToolUseFailure", "PermissionRequest", "PermissionDenied",
	"SubagentStart", "SubagentStop", "PreCompact", "PostCompact", "Stop",
	"StopFailure", "SessionEnd",
}

type claudeCodePatcher struct{}

func init() {
	register(claudeCodePatcher{})
}

func (claudeCodePatcher) AgentId() string { return "claude-code" }

func (claudeCodePatcher) Supported() bool { return true }

func (claudeCodePatcher) Patch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := claudeCodeConfigPath(target)
	if err != nil {
		return err
	}
	config, mode, _, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	command, args, err := claudeCodeHookCommand(target)
	if err != nil {
		return err
	}
	if err := normalizeClaudeCodeHooks(config, command, args); err != nil {
		return err
	}
	return writeJSONConfig(path, config, mode)
}

func (claudeCodePatcher) Unpatch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := claudeCodeConfigPath(target)
	if err != nil {
		return err
	}
	config, mode, exists, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	// The credential is revoked on every unpatch path, including the ones that
	// change no file, so a stale hook can never keep reporting.
	if exists && removeClaudeCodeHooks(config) {
		if err := writeJSONConfig(path, config, mode); err != nil {
			return err
		}
	}
	return RevokeIngestToken(target)
}

// PatchNotice explains the change in the operator's own words. Every other
// patcher supplies one, so without it the Claude Code row was the only one in
// the UI that gave no hint about what changes or whether a restart is needed.
func (claudeCodePatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's audit-only Claude Code hooks.", "Restart any Claude Code session that is already running."
	}
	return "Installs audit-only Claude Code hooks. Hooks observe events and never block an action.", "Restart any Claude Code session that is already running."
}

func (claudeCodePatcher) Status(target Target) (Status, error) {
	path, err := claudeCodeConfigPath(target)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		return Status{}, err
	}
	if !exists {
		return Status{Detail: "Claude Code hooks are not installed"}, nil
	}
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return Status{Detail: "Claude Code hooks are not installed"}, nil
	}
	for _, event := range claudeCodeHookEvents {
		if !hasHook(hooks[event], isClaudeCodeHook) {
			return Status{Detail: "Claude Code hooks need refresh"}, nil
		}
	}
	return Status{Patched: true, Detail: "Claude Code hooks active"}, nil
}

func claudeCodeConfigPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func claudeCodeHookCommand(target Target) (string, []string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", nil, fmt.Errorf("resolve Gateway executable: %w", err)
	}
	url, err := recordsURL()
	if err != nil {
		return "", nil, err
	}
	token, err := IssueIngestToken(target)
	if err != nil {
		return "", nil, err
	}
	return executable, []string{
		agenthook.Subcommand,
		agenthook.OwnershipFlag,
		"--agent", "claude-code",
		"--records-url", url,
		"--agent-path", target.Path,
		"--user", target.Owner,
		"--ingest-token", token,
	}, nil
}

// normalizeClaudeCodeHooks leaves non-Gateway handlers untouched and leaves
// exactly one current Gateway handler for each Claude Code event.
func normalizeClaudeCodeHooks(config map[string]any, command string, args []string) error {
	hooks, exists := config["hooks"]
	if !exists {
		hooks = map[string]any{}
		config["hooks"] = hooks
	}
	object, ok := objectAt(hooks)
	if !ok {
		return fmt.Errorf("hooks must be a JSON object")
	}
	for _, event := range claudeCodeHookEvents {
		groups, err := withoutHooks(object[event], isClaudeCodeHook)
		if err != nil {
			return fmt.Errorf("hooks.%s: %w", event, err)
		}
		group := map[string]any{"hooks": []any{newClaudeCodeHook(command, args)}}
		if hookEventSupportsMatcher(event) {
			group["matcher"] = ""
		}
		object[event] = append(groups, group)
	}
	return nil
}

func newClaudeCodeHook(command string, args []string) map[string]any {
	return map[string]any{
		"type":    "command",
		"command": command,
		"args":    args,
		"async":   true,
		"timeout": claudeCodeHookTimeoutSeconds,
	}
}

func removeClaudeCodeHooks(config map[string]any) bool {
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return false
	}
	changed := false
	for _, event := range claudeCodeHookEvents {
		if !hasHook(hooks[event], isClaudeCodeHook) {
			continue
		}
		groups, err := withoutHooks(hooks[event], isClaudeCodeHook)
		if err != nil {
			continue
		}
		changed = true
		if len(groups) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = groups
		}
	}
	if changed && len(hooks) == 0 {
		delete(config, "hooks")
	}
	return changed
}

func isClaudeCodeHook(handler map[string]any) bool {
	if handlerType, _ := handler["type"].(string); handlerType != "command" {
		return false
	}
	args := stringArray(handler["args"])
	agentIndex := stringIndex(args, "--agent")
	return len(args) > 0 && args[0] == agenthook.Subcommand && stringIndex(args, agenthook.OwnershipFlag) >= 0 &&
		agentIndex >= 0 && agentIndex+1 < len(args) && args[agentIndex+1] == "claude-code" &&
		stringIndex(args, "--records-url") >= 0
}

func hookEventSupportsMatcher(event string) bool {
	switch event {
	case "PreToolUse", "PostToolUse", "PostToolUseFailure", "PermissionRequest", "PermissionDenied":
		return true
	default:
		return false
	}
}

func stringIndex(items []string, target string) int {
	for index, item := range items {
		if item == target {
			return index
		}
	}
	return -1
}
