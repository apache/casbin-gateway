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

// This file sorts the tools an agent offers its model into the handful of
// groups worth a switch, and takes the ones a switch is off for back out of the
// request. A tool the model is never offered is one the agent cannot run.

package object

import (
	"encoding/json"
	"strings"
)

// The tool groups. Each is one switch on the Permissions card and one casbin
// object, "tool:<group>".
const (
	ToolShell     = "shell"
	ToolFileRead  = "fileRead"
	ToolFileWrite = "fileWrite"
	ToolNetwork   = "network"
	ToolMcp       = "mcp"
)

// ToolGroup is one group as the web UI is told about it: the name its switch
// saves, and a few of the tools that fall in it, since every agent names its
// own differently.
type ToolGroup struct {
	Name     string   `json:"name"`
	Examples []string `json:"examples"`
}

// toolGroups is the order the groups are shown and matched in. Network comes
// before the file groups so that a "web_search" is not read as a search of the
// disk, and the file groups before shell so that "apply_patch" is a write.
var toolGroups = []ToolGroup{
	{Name: ToolShell, Examples: []string{"Bash", "shell", "run_shell_command"}},
	{Name: ToolFileRead, Examples: []string{"Read", "Grep", "read_file"}},
	{Name: ToolFileWrite, Examples: []string{"Write", "Edit", "apply_patch"}},
	{Name: ToolNetwork, Examples: []string{"WebFetch", "WebSearch", "web_fetch"}},
	{Name: ToolMcp, Examples: []string{"mcp__*"}},
}

// ToolGroups lists the groups, for the UI that draws a switch per group.
func ToolGroups() []ToolGroup {
	groups := make([]ToolGroup, len(toolGroups))
	copy(groups, toolGroups)
	return groups
}

// toolNames are the tools of the agents Gateway knows, named as they name them.
// They are matched before the keywords below, which is what keeps a "TodoWrite"
// out of the group that writes files.
var toolNames = map[string]string{
	"bash":              ToolShell,
	"bashoutput":        ToolShell,
	"killbash":          ToolShell,
	"killshell":         ToolShell,
	"shell":             ToolShell,
	"localshell":        ToolShell,
	"runshellcommand":   ToolShell,
	"runterminalcmd":    ToolShell,
	"executecommand":    ToolShell,
	"runcommand":        ToolShell,
	"terminal":          ToolShell,
	"read":              ToolFileRead,
	"readfile":          ToolFileRead,
	"readmanyfiles":     ToolFileRead,
	"view":              ToolFileRead,
	"glob":              ToolFileRead,
	"grep":              ToolFileRead,
	"ls":                ToolFileRead,
	"listdir":           ToolFileRead,
	"listdirectory":     ToolFileRead,
	"searchfilecontent": ToolFileRead,
	"codebasesearch":    ToolFileRead,
	"write":             ToolFileWrite,
	"writefile":         ToolFileWrite,
	"edit":              ToolFileWrite,
	"editfile":          ToolFileWrite,
	"multiedit":         ToolFileWrite,
	"notebookedit":      ToolFileWrite,
	"applypatch":        ToolFileWrite,
	"patch":             ToolFileWrite,
	"replace":           ToolFileWrite,
	"strreplaceeditor":  ToolFileWrite,
	"createfile":        ToolFileWrite,
	"deletefile":        ToolFileWrite,
	"webfetch":          ToolNetwork,
	"websearch":         ToolNetwork,
	"webfetchtool":      ToolNetwork,
	"googlewebsearch":   ToolNetwork,
	"fetch":             ToolNetwork,
	// The planning and delegation tools of the agents, named here so the
	// keywords below do not read them as something they take from the disk.
	"todowrite":    "",
	"todoread":     "",
	"updateplan":   "",
	"exitplanmode": "",
	"task":         "",
	"agent":        "",
}

// toolKeywords catch the tools of an agent Gateway has never seen, in the order
// they are tried.
var toolKeywords = []struct {
	group    string
	keywords []string
}{
	{ToolNetwork, []string{"web", "http", "url", "browser", "curl", "download", "internet", "google", "fetch"}},
	{ToolShell, []string{"bash", "shell", "terminal", "command", "cmd", "exec", "process", "script"}},
	{ToolFileWrite, []string{"write", "edit", "patch", "create", "delete", "remove", "move", "rename", "insert", "replace", "mkdir"}},
	{ToolFileRead, []string{"read", "view", "cat", "open", "glob", "grep", "search", "find", "list", "dir", "tree"}},
}

// ToolGroupOf is the group one tool falls in, or "" for a tool that fits none
// of them and is therefore always allowed.
func ToolGroupOf(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return ""
	}

	// An MCP tool is named after the server it came from, whichever agent is
	// asking: it is the one group a name says outright.
	if strings.HasPrefix(lower, "mcp__") || strings.HasPrefix(lower, "mcp_") || strings.HasPrefix(lower, "mcp.") {
		return ToolMcp
	}

	normalized := normalizeToolName(lower)
	if group, listed := toolNames[normalized]; listed {
		return group
	}

	for _, entry := range toolKeywords {
		for _, keyword := range entry.keywords {
			if strings.Contains(normalized, keyword) {
				return entry.group
			}
		}
	}
	return ""
}

// normalizeToolName drops what only separates the words of a tool's name, so
// "run_shell_command" and "runShellCommand" are the same tool.
func normalizeToolName(lower string) string {
	builder := strings.Builder{}
	for _, char := range lower {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// FilterTools takes the tools of the groups this agent may not use back out of
// a request body, and reports the ones it removed. The body is left untouched
// when there is nothing to remove, so a request that changes nothing is relayed
// byte for byte as before.
//
// Only the "tools" array is read, which every format the gateway speaks carries
// under that name: OpenAI names a tool in "function", Anthropic and the
// Responses API at the top level, and Gemini inside "functionDeclarations".
func (guard *AgentGuard) FilterTools(body []byte) ([]byte, []string) {
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &fields); err != nil {
		return body, nil
	}

	items := []json.RawMessage{}
	if err := json.Unmarshal(fields["tools"], &items); err != nil || len(items) == 0 {
		return body, nil
	}

	kept := []json.RawMessage{}
	removed := []string{}
	for _, item := range items {
		filtered, names := guard.filterTool(item)
		removed = append(removed, names...)
		if filtered != nil {
			kept = append(kept, filtered)
		}
	}

	if len(removed) == 0 {
		return body, nil
	}

	if len(kept) == 0 {
		// An empty tools array is rejected by some APIs, so a request left with
		// no tool at all is sent as one that never asked for any.
		delete(fields, "tools")
		delete(fields, "tool_choice")
	} else {
		encoded, err := json.Marshal(kept)
		if err != nil {
			return body, nil
		}
		fields["tools"] = encoded
	}

	filtered, err := json.Marshal(fields)
	if err != nil {
		return body, nil
	}
	return filtered, removed
}

// filterTool answers with the entry to keep, nil to drop it altogether, and the
// names of whatever it removed.
func (guard *AgentGuard) filterTool(item json.RawMessage) (json.RawMessage, []string) {
	entry := struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
		FunctionDeclarations []json.RawMessage `json:"functionDeclarations"`
	}{}
	if err := json.Unmarshal(item, &entry); err != nil {
		return item, nil
	}

	// Gemini wraps its tools in one entry, so the entry stays as long as any of
	// the declarations inside it does.
	if entry.FunctionDeclarations != nil {
		kept := []json.RawMessage{}
		removed := []string{}
		for _, declaration := range entry.FunctionDeclarations {
			name := struct {
				Name string `json:"name"`
			}{}
			if err := json.Unmarshal(declaration, &name); err != nil || guard.AllowTool(name.Name) {
				kept = append(kept, declaration)
				continue
			}
			removed = append(removed, name.Name)
		}
		if len(removed) == 0 {
			return item, nil
		}
		if len(kept) == 0 {
			return nil, removed
		}

		fields := map[string]json.RawMessage{}
		if err := json.Unmarshal(item, &fields); err != nil {
			return item, nil
		}
		encoded, err := json.Marshal(kept)
		if err != nil {
			return item, nil
		}
		fields["functionDeclarations"] = encoded
		rewritten, err := json.Marshal(fields)
		if err != nil {
			return item, nil
		}
		return rewritten, removed
	}

	name := entry.Name
	if name == "" && entry.Function != nil {
		name = entry.Function.Name
	}
	if name == "" {
		// A built-in tool of the API itself, such as OpenAI's web search, is
		// named by its type alone.
		name = entry.Type
	}
	if name == "" || guard.AllowTool(name) {
		return item, nil
	}
	return nil, []string{name}
}
