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

type windsurfPatcher struct{}

func init() {
	register(windsurfPatcher{})
}

func (windsurfPatcher) AgentId() string { return "windsurf" }

func (windsurfPatcher) Supported() bool { return true }

func (p windsurfPatcher) Patch(target Target) error {
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
	entry, err := newWindsurfHook(target)
	if err != nil {
		return err
	}

	hooks, err := windsurfHooks(config)
	if err != nil {
		return err
	}
	for _, event := range agenthook.WindsurfEvents {
		handlers, err := withoutFlatHooks(hooks[event], isWindsurfHook)
		if err != nil {
			return fmt.Errorf("hooks.%s: %w", event, err)
		}
		hooks[event] = append(handlers, entry)
	}
	return writeJSONConfig(path, config, mode)
}

func (p windsurfPatcher) Unpatch(target Target) error {
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
	if exists && removeWindsurfHooks(config) {
		if err := writeJSONConfig(path, config, mode); err != nil {
			return err
		}
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p windsurfPatcher) Status(target Target) (Status, error) {
	path, err := p.hooksPath(target)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		return Status{}, err
	}
	if !exists {
		return Status{Detail: "Cascade hooks are not installed"}, nil
	}
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return Status{Detail: "Cascade hooks are not installed"}, nil
	}
	for _, event := range agenthook.WindsurfEvents {
		if !hasFlatHook(hooks[event], isCurrentWindsurfHook) {
			return Status{Detail: "Cascade hooks need refresh"}, nil
		}
	}
	return Status{Patched: true, Detail: "Cascade hooks active"}, nil
}

// Decides: Cascade waits on its pre_ hooks and blocks the action when one exits
// with code 2, which is how a refusal is written there.
func (windsurfPatcher) Decides() bool { return true }

func (windsurfPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's Cascade hooks, and with them the check before a command, an edit or an MCP call.", "Restart Windsurf to stop running them."
	}
	return "Installs Gateway's Cascade hooks. They observe events, and refuse a command, an edit or an MCP call this agent's permissions do not allow; an agent nobody has restricted is never held up.",
		"Restart Windsurf to load them."
}

// hooksPath is the user-level Cascade hooks file, beside the MCP configuration
// the skill and server listings already read.
func (windsurfPatcher) hooksPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codeium", "windsurf", "hooks.json"), nil
}

// newWindsurfHook is one entry. Cascade runs command through a shell and
// powershell through PowerShell, where a quoted path is a string rather than
// something to run until the call operator says otherwise.
func newWindsurfHook(target Target) (map[string]any, error) {
	executable, err := gatewayExecutable()
	if err != nil {
		return nil, err
	}
	args, err := hookArgs("windsurf", target)
	if err != nil {
		return nil, err
	}
	command := hookCommandLine(executable, args)
	return map[string]any{
		"command":    command,
		"powershell": "& " + command,
		// Monitoring is not something the operator asked to watch happen.
		"show_output": false,
	}, nil
}

func windsurfHooks(config map[string]any) (map[string]any, error) {
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

func removeWindsurfHooks(config map[string]any) bool {
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return false
	}
	changed := false
	for _, event := range agenthook.WindsurfEvents {
		if !hasFlatHook(hooks[event], isWindsurfHook) {
			continue
		}
		handlers, err := withoutFlatHooks(hooks[event], isWindsurfHook)
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

func isCurrentWindsurfHook(handler map[string]any) bool {
	if !isWindsurfHook(handler) {
		return false
	}
	command, _ := handler["command"].(string)
	return commandIsCurrent(command)
}

func isWindsurfHook(handler map[string]any) bool {
	command, _ := handler["command"].(string)
	return strings.Contains(command, agenthook.OwnershipFlag) &&
		strings.Contains(command, "--agent windsurf") &&
		strings.Contains(command, "--records-url")
}
