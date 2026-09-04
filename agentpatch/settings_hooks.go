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

// hookTimeoutMs is what these CLIs wait for the hook, in milliseconds.
// Reporting a record is one loopback request, and a hook that hangs must not
// hold up the session.
const hookTimeoutMs = 5000

// settingsHookPatcher installs Gateway's audit-only command hooks into one
// agent's settings.json. The Gemini CLI and the forks that took its
// configuration all spell hooks the same way — events keyed by name, each an
// array of groups holding command handlers — and differ only in where the file
// sits and which events they fire.
type settingsHookPatcher struct {
	agentId string
	// name is how the agent is called in the status line and the patch notice.
	name string
	// dir is the agent's state directory under the home, ".gemini" and the like.
	dir    string
	events []string
}

func (p settingsHookPatcher) AgentId() string { return p.agentId }

func (settingsHookPatcher) Supported() bool { return true }

func (p settingsHookPatcher) Patch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := p.settingsPath(target)
	if err != nil {
		return err
	}
	config, mode, _, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	command, err := p.command(target)
	if err != nil {
		return err
	}

	hooks, err := p.hooksOf(config)
	if err != nil {
		return err
	}
	for _, event := range p.events {
		groups, err := withoutHooks(hooks[event], p.owns)
		if err != nil {
			return fmt.Errorf("hooks.%s: %w", event, err)
		}
		hooks[event] = append(groups, map[string]any{"hooks": []any{newSettingsHook(command)}})
	}
	return writeJSONConfig(path, config, mode)
}

func (p settingsHookPatcher) Unpatch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := p.settingsPath(target)
	if err != nil {
		return err
	}
	config, mode, exists, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	// The credential is revoked on every unpatch path, including the ones that
	// change no file, so a stale hook can never keep reporting.
	if exists && p.remove(config) {
		if err := writeJSONConfig(path, config, mode); err != nil {
			return err
		}
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p settingsHookPatcher) Status(target Target) (Status, error) {
	path, err := p.settingsPath(target)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		return Status{}, err
	}
	if !exists {
		return Status{Detail: p.name + " hooks are not installed"}, nil
	}
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return Status{Detail: p.name + " hooks are not installed"}, nil
	}
	for _, event := range p.events {
		if !hasHook(hooks[event], p.owns) {
			return Status{Detail: p.name + " hooks need refresh"}, nil
		}
	}
	return Status{Patched: true, Detail: p.name + " hooks active"}, nil
}

func (p settingsHookPatcher) PatchNotice(patched bool) (string, string) {
	restart := "Restart any " + p.name + " session that is already running."
	if patched {
		return "Removes Gateway's audit-only " + p.name + " hooks.", restart
	}
	return "Installs audit-only " + p.name + " hooks. Hooks observe events and never block an action.", restart
}

func (p settingsHookPatcher) settingsPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p.dir, "settings.json"), nil
}

// command is the whole command line, since these CLIs take a command string
// rather than a program and its arguments.
func (p settingsHookPatcher) command(target Target) (string, error) {
	executable, err := gatewayExecutable()
	if err != nil {
		return "", err
	}
	args, err := hookArgs(p.agentId, target)
	if err != nil {
		return "", err
	}
	return hookCommandLine(executable, args), nil
}

func (settingsHookPatcher) hooksOf(config map[string]any) (map[string]any, error) {
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

func (p settingsHookPatcher) remove(config map[string]any) bool {
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return false
	}
	changed := false
	for _, event := range p.events {
		if !hasHook(hooks[event], p.owns) {
			continue
		}
		groups, err := withoutHooks(hooks[event], p.owns)
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

// owns recognizes Gateway's own handler by the command it runs, which is the
// only thing one of these hook entries carries.
func (p settingsHookPatcher) owns(handler map[string]any) bool {
	if handlerType, _ := handler["type"].(string); handlerType != "command" {
		return false
	}
	command, _ := handler["command"].(string)
	return strings.Contains(command, agenthook.OwnershipFlag) &&
		strings.Contains(command, "--agent "+p.agentId) &&
		strings.Contains(command, "--records-url")
}

func newSettingsHook(command string) map[string]any {
	return map[string]any{
		"type":    "command",
		"command": command,
		"name":    gatewayEntryName,
		"timeout": hookTimeoutMs,
	}
}
