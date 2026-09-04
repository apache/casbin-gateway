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
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agenthook"
)

const (
	// cursorHooksVersion is the schema version the file declares.
	cursorHooksVersion = 1
	// cursorHookTimeoutMs is what Cursor waits for the hook. Reporting a record
	// is one loopback request, and a hook that hangs must not hold up the agent.
	cursorHookTimeoutMs = 5000
	// cursorMonitorId is the agent Cursor records are reported under. The editor
	// and the CLI read one ~/.cursor/hooks.json, so they share this file and the
	// monitoring it feeds.
	cursorMonitorId = "cursor"
)

type cursorPatcher struct {
	id string
}

func init() {
	register(cursorPatcher{id: "cursor"})
	register(cursorPatcher{id: "cursor-agent"})
}

func (p cursorPatcher) AgentId() string { return p.id }

func (cursorPatcher) Supported() bool { return true }

// Decides: Cursor waits on preToolUse, beforeShellExecution and
// beforeMCPExecution, and a hook that answers "deny" stops the call.
func (cursorPatcher) Decides() bool { return true }

func (p cursorPatcher) Patch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := p.hooksPath(target)
	if err != nil {
		return err
	}
	config, mode, _, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	command, err := cursorHookCommand(target)
	if err != nil {
		return err
	}

	hooks, err := cursorHooks(config)
	if err != nil {
		return err
	}
	// The version the file already declares is the one Cursor wrote, and a
	// later schema is not Gateway's to downgrade.
	if _, declared := config["version"]; !declared {
		config["version"] = cursorHooksVersion
	}
	for _, event := range agenthook.CursorEvents {
		handlers, err := withoutFlatHooks(hooks[event], isCursorHook)
		if err != nil {
			return fmt.Errorf("hooks.%s: %w", event, err)
		}
		hooks[event] = append(handlers, newCursorHook(command))
	}
	return writeJSONConfig(path, config, mode)
}

func (p cursorPatcher) Unpatch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := p.hooksPath(target)
	if err != nil {
		return err
	}
	config, mode, exists, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	// The credential is revoked on every unpatch path, including the ones that
	// change no file, so a stale hook can never keep reporting.
	if exists && removeCursorHooks(config) {
		if err := writeJSONConfig(path, config, mode); err != nil {
			return err
		}
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p cursorPatcher) Status(target Target) (Status, error) {
	path, err := p.hooksPath(target)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		return Status{}, err
	}
	if !exists {
		return Status{Detail: "Cursor hooks are not installed"}, nil
	}
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return Status{Detail: "Cursor hooks are not installed"}, nil
	}
	for _, event := range agenthook.CursorEvents {
		if !hasFlatHook(hooks[event], isCurrentCursorHook) {
			return Status{Detail: "Cursor hooks need refresh"}, nil
		}
	}
	return Status{Patched: true, Detail: "Cursor hooks active"}, nil
}

func (cursorPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's Cursor hooks. The editor and the CLI share them, so both stop reporting and stop being checked.",
			"Restart any Cursor session that is already running."
	}
	return "Installs Gateway's Cursor hooks. They observe events, and refuse a shell command, an MCP call or a tool call this agent's permissions do not allow; the CLI reports fewer events than the editor, and an agent nobody has restricted is never held up.",
		"Restart any Cursor session that is already running."
}

// hooksPath is the user-level hooks file, which both Cursor front ends read.
func (cursorPatcher) hooksPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cursor", "hooks.json"), nil
}

// cursorHookCommand is the whole command line, since Cursor takes a command
// string rather than a program and its arguments.
func cursorHookCommand(target Target) (string, error) {
	executable, err := gatewayExecutable()
	if err != nil {
		return "", err
	}
	args, err := hookArgs(cursorMonitorId, target)
	if err != nil {
		return "", err
	}
	return hookCommandLine(executable, args), nil
}

func cursorHooks(config map[string]any) (map[string]any, error) {
	value, exists := config["hooks"]
	if !exists {
		hooks := map[string]any{}
		config["hooks"] = hooks
		return hooks, nil
	}
	hooks, ok := objectAt(value)
	if !ok {
		return nil, fmt.Errorf("hooks must be a JSON object")
	}
	return hooks, nil
}

// newCursorHook is one entry. failClosed is written even though it is the
// default: monitoring must never be the reason an action is refused, and the
// file says so where an operator reads it.
func newCursorHook(command string) map[string]any {
	return map[string]any{
		"command":    command,
		"type":       "command",
		"timeout":    cursorHookTimeoutMs,
		"failClosed": false,
	}
}

func removeCursorHooks(config map[string]any) bool {
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return false
	}
	changed := false
	for _, event := range agenthook.CursorEvents {
		if !hasFlatHook(hooks[event], isCursorHook) {
			continue
		}
		handlers, err := withoutFlatHooks(hooks[event], isCursorHook)
		if err != nil {
			continue
		}
		changed = true
		if len(handlers) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = handlers
		}
	}
	if changed && len(hooks) == 0 {
		delete(config, "hooks")
	}
	return changed
}

func isCurrentCursorHook(handler map[string]any) bool {
	if !isCursorHook(handler) {
		return false
	}
	command, _ := handler["command"].(string)
	return commandIsCurrent(command)
}

func isCursorHook(handler map[string]any) bool {
	command, _ := handler["command"].(string)
	return strings.Contains(command, agenthook.OwnershipFlag) &&
		strings.Contains(command, "--agent "+cursorMonitorId) &&
		strings.Contains(command, "--records-url")
}
