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
	"strings"

	"github.com/apache/casbin-gateway/agenthook"
)

// Claude Code and the Gemini CLI spell an event the same way: an array of
// groups, each holding the command handlers that run for it. These walk that
// shape, so that a patcher only has to say which handler is its own.

// withoutHooks drops the handlers own reports, leaving every other handler and
// group exactly as the file had them.
func withoutHooks(value any, own func(map[string]any) bool) ([]any, error) {
	if value == nil {
		return nil, nil
	}
	groups, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a JSON array")
	}

	result := make([]any, 0, len(groups))
	for _, rawGroup := range groups {
		group, ok := objectAt(rawGroup)
		if !ok {
			result = append(result, rawGroup)
			continue
		}
		handlers, ok := group["hooks"].([]any)
		if !ok {
			result = append(result, rawGroup)
			continue
		}
		kept := make([]any, 0, len(handlers))
		for _, rawHandler := range handlers {
			handler, ok := objectAt(rawHandler)
			if !ok || !own(handler) {
				kept = append(kept, rawHandler)
			}
		}
		if len(kept) == 0 {
			continue
		}
		if len(kept) != len(handlers) {
			group["hooks"] = kept
		}
		result = append(result, group)
	}
	return result, nil
}

// hasHook reports whether one event already runs own handler.
func hasHook(value any, own func(map[string]any) bool) bool {
	groups, ok := value.([]any)
	if !ok {
		return false
	}
	for _, rawGroup := range groups {
		group, ok := objectAt(rawGroup)
		if !ok {
			continue
		}
		handlers, ok := group["hooks"].([]any)
		if !ok {
			continue
		}
		for _, rawHandler := range handlers {
			handler, ok := objectAt(rawHandler)
			if ok && own(handler) {
				return true
			}
		}
	}
	return false
}

// A Cursor or Windsurf event holds its handlers directly, without the groups
// Claude Code and the Gemini CLI wrap theirs in.
func withoutFlatHooks(value any, own func(map[string]any) bool) ([]any, error) {
	if value == nil {
		return nil, nil
	}
	handlers, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("must be a JSON array")
	}
	kept := make([]any, 0, len(handlers))
	for _, rawHandler := range handlers {
		handler, ok := objectAt(rawHandler)
		if !ok || !own(handler) {
			kept = append(kept, rawHandler)
		}
	}
	return kept, nil
}

func hasFlatHook(value any, own func(map[string]any) bool) bool {
	handlers, ok := value.([]any)
	if !ok {
		return false
	}
	for _, rawHandler := range handlers {
		if handler, ok := objectAt(rawHandler); ok && own(handler) {
			return true
		}
	}
	return false
}

// hookArgs are the arguments Gateway's own hook subcommand is invoked with.
func hookArgs(agentId string, target Target) ([]string, error) {
	url, err := recordsURL()
	if err != nil {
		return nil, err
	}
	token, err := IssueIngestToken(monitorTarget(target))
	if err != nil {
		return nil, err
	}
	return []string{
		agenthook.Subcommand,
		agenthook.OwnershipFlag,
		"--agent", agentId,
		"--records-url", url,
		"--agent-path", target.Path,
		"--user", target.Owner,
		"--ingest-token", token,
	}, nil
}

// hookCommandLine renders one command line for an agent whose configuration
// takes a command string rather than a program and its arguments.
func hookCommandLine(executable string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	for _, part := range append([]string{executable}, args...) {
		parts = append(parts, quoteArg(part))
	}
	return strings.Join(parts, " ")
}

// quoteArg quotes what a shell would otherwise split or swallow. Gateway's own
// path is the usual case: it holds spaces on both Windows and macOS. A
// backslash is left alone, since cmd.exe takes one literally inside quotes and
// a path that reaches this anywhere else has none.
func quoteArg(value string) string {
	if value == "" {
		return `""`
	}
	if !strings.ContainsAny(value, " \t\"'$`&|<>()") {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func gatewayExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve Gateway executable: %w", err)
	}
	return executable, nil
}
