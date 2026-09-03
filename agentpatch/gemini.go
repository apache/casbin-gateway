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

// geminiHookTimeoutMs is what the Gemini CLI waits for the hook, in
// milliseconds. Reporting a record is one loopback request, and a hook that
// hangs must not hold up the session.
const geminiHookTimeoutMs = 5000

type geminiPatcher struct{}

func init() {
	register(geminiPatcher{})
}

func (geminiPatcher) AgentId() string { return "gemini-cli" }

func (geminiPatcher) Supported() bool { return true }

func (p geminiPatcher) Patch(target Target) error {
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
	command, err := geminiHookCommand(target)
	if err != nil {
		return err
	}

	hooks, err := geminiHooks(config)
	if err != nil {
		return err
	}
	for _, event := range agenthook.GeminiEvents {
		groups, err := withoutHooks(hooks[event], isGeminiHook)
		if err != nil {
			return fmt.Errorf("hooks.%s: %w", event, err)
		}
		hooks[event] = append(groups, map[string]any{"hooks": []any{newGeminiHook(command)}})
	}
	return writeJSONConfig(path, config, mode)
}

func (p geminiPatcher) Unpatch(target Target) error {
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
	if exists && removeGeminiHooks(config) {
		if err := writeJSONConfig(path, config, mode); err != nil {
			return err
		}
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p geminiPatcher) Status(target Target) (Status, error) {
	path, err := p.settingsPath(target)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		return Status{}, err
	}
	if !exists {
		return Status{Detail: "Gemini CLI hooks are not installed"}, nil
	}
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return Status{Detail: "Gemini CLI hooks are not installed"}, nil
	}
	for _, event := range agenthook.GeminiEvents {
		if !hasHook(hooks[event], isGeminiHook) {
			return Status{Detail: "Gemini CLI hooks need refresh"}, nil
		}
	}
	return Status{Patched: true, Detail: "Gemini CLI hooks active"}, nil
}

func (geminiPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's audit-only Gemini CLI hooks.", "Restart any Gemini CLI session that is already running."
	}
	return "Installs audit-only Gemini CLI hooks. Hooks observe events and never block an action.",
		"Restart any Gemini CLI session that is already running."
}

func (geminiPatcher) settingsPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "settings.json"), nil
}

// geminiHookCommand is the whole command line, since the Gemini CLI takes a
// command string rather than a program and its arguments.
func geminiHookCommand(target Target) (string, error) {
	executable, err := gatewayExecutable()
	if err != nil {
		return "", err
	}
	args, err := hookArgs("gemini-cli", target)
	if err != nil {
		return "", err
	}
	return hookCommandLine(executable, args), nil
}

func geminiHooks(config map[string]any) (map[string]any, error) {
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

func newGeminiHook(command string) map[string]any {
	return map[string]any{
		"type":    "command",
		"command": command,
		"name":    gatewayEntryName,
		"timeout": geminiHookTimeoutMs,
	}
}

func removeGeminiHooks(config map[string]any) bool {
	hooks, ok := objectAt(config["hooks"])
	if !ok {
		return false
	}
	changed := false
	for _, event := range agenthook.GeminiEvents {
		if !hasHook(hooks[event], isGeminiHook) {
			continue
		}
		groups, err := withoutHooks(hooks[event], isGeminiHook)
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

// isGeminiHook recognizes Gateway's own handler by the command it runs, which
// is the only thing a Gemini CLI hook entry carries.
func isGeminiHook(handler map[string]any) bool {
	if handlerType, _ := handler["type"].(string); handlerType != "command" {
		return false
	}
	command, _ := handler["command"].(string)
	return strings.Contains(command, agenthook.OwnershipFlag) &&
		strings.Contains(command, "--agent gemini-cli") &&
		strings.Contains(command, "--records-url")
}
