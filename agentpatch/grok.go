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
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agenthook"
)

// grokHookFile is Gateway's own file among Grok's global hooks. Grok loads
// every JSON file in $GROK_HOME/hooks as trusted, so the hand-edited
// config.toml is never touched and unpatching is deleting this file.
const grokHookFile = "casbin-gateway.json"

// grokHookTimeout is in seconds. Grok gives Stop and PostToolUse ten minutes
// by default, which a hook that only reports should never hold a turn for.
const grokHookTimeout = 5

type grokPatcher struct{}

func init() {
	register(grokPatcher{})
}

func (grokPatcher) AgentId() string { return "grok-build" }

func (grokPatcher) Supported() bool { return true }

// Decides: Grok waits on PreToolUse and refuses the call on a deny decision.
func (grokPatcher) Decides() bool { return true }

func (p grokPatcher) Patch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := grokHookPath(target.Owner)
	if err != nil {
		return err
	}
	executable, err := gatewayExecutable()
	if err != nil {
		return err
	}
	args, err := hookArgs(p.AgentId(), target)
	if err != nil {
		return err
	}
	command := grokCommandLine(executable, args)

	hooks := map[string]any{}
	for _, event := range agenthook.GrokEvents {
		hooks[event] = []any{map[string]any{"hooks": []any{map[string]any{
			"type":    "command",
			"command": command,
			"timeout": grokHookTimeout,
		}}}}
	}
	return writeJSONConfig(path, map[string]any{"hooks": hooks}, 0o600)
}

func (p grokPatcher) Unpatch(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := grokHookPath(target.Owner)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p grokPatcher) Status(target Target) (Status, error) {
	path, err := grokHookPath(target.Owner)
	if err != nil {
		return Status{}, err
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		return Status{}, err
	}
	hooks, ok := objectAt(config["hooks"])
	if !exists || !ok {
		return Status{Detail: "Grok Build hooks are not installed"}, nil
	}
	for _, event := range agenthook.GrokEvents {
		if !hasHook(hooks[event], grokHookIsCurrent) {
			return Status{Detail: "Grok Build hooks need refresh"}, nil
		}
	}
	return Status{Patched: true, Detail: "Grok Build hooks active"}, nil
}

func (grokPatcher) PatchNotice(patched bool) (string, string) {
	restart := "Restart any Grok Build session that is already running."
	if patched {
		return "Removes Gateway's Grok Build hooks, and with them the check before each tool call.", restart
	}
	return "Installs Gateway's Grok Build hooks. They observe events, and refuse a tool call this agent's permissions do not allow; an agent nobody has restricted is never held up.", restart
}

func grokHookIsCurrent(handler map[string]any) bool {
	command, _ := handler["command"].(string)
	if !strings.Contains(command, agenthook.OwnershipFlag) || !strings.Contains(command, "grok-build") {
		return false
	}
	executable, err := gatewayExecutable()
	if err != nil {
		return false
	}
	url, err := recordsURL()
	if err != nil {
		return false
	}
	decision, err := decisionURL()
	if err != nil {
		return false
	}
	return strings.HasPrefix(command, grokCommandLine(executable, nil)) &&
		strings.Contains(command, url) && strings.Contains(command, decision)
}

// grokCommandLine is one command line for Grok's shell. Grok ignores an args
// array and hands the command to PowerShell on Windows, where a quoted program
// needs the call operator and an empty argument is dropped outright.
func grokCommandLine(executable string, args []string) string {
	if runtime.GOOS != "windows" {
		return hookCommandLine(executable, args)
	}
	parts := []string{"&", powershellQuote(executable)}
	for index := 0; index < len(args); index++ {
		if strings.HasPrefix(args[index], "--") && index+1 < len(args) && args[index+1] == "" {
			parts = append(parts, args[index]+"=")
			index++
			continue
		}
		parts = append(parts, powershellQuote(args[index]))
	}
	return strings.Join(parts, " ")
}

func powershellQuote(value string) string {
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' ||
			strings.ContainsRune(`-_./:=\`, char)) {
			return "'" + strings.ReplaceAll(value, "'", "''") + "'"
		}
	}
	return value
}

// grokHookPath is where Grok looks for global hooks. GROK_HOME moves it, but
// only for the account Gateway runs as.
func grokHookPath(owner string) (string, error) {
	if current, err := user.Current(); err == nil && agenthome.SameAccount(owner, current.Username) {
		if configured := strings.TrimSpace(os.Getenv("GROK_HOME")); configured != "" {
			if !filepath.IsAbs(configured) {
				return "", errors.New("GROK_HOME must be an absolute path")
			}
			return filepath.Join(filepath.Clean(configured), "hooks", grokHookFile), nil
		}
	}
	home, err := agenthome.Resolve(owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".grok", "hooks", grokHookFile), nil
}
