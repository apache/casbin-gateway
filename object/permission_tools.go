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

// This file holds the catalogue of what an agent can be allowed to do, and
// sorts the tools it offers its model into it. A tool the switch is off for is
// taken back out of the request, so the model is never offered it.

package object

import (
	"encoding/json"
	"strings"
	"sync"
)

// The groups the permissions are drawn in, in the order they are shown.
const (
	GroupShell   = "shell"
	GroupRead    = "read"
	GroupWrite   = "write"
	GroupNetwork = "network"
	GroupAgentic = "agentic"
	GroupMcp     = "mcp"
)

// otherItem is the last item of a group: every tool of that kind Gateway does
// not know by name. Switching it off is what closes a group for good, rather
// than for the tools that happened to be listed when it was set.
const otherItem = "other"

// ToolItem is one switch: a thing an agent can be allowed to do, and the tool
// names the agents call it by. The name is "<group>/<item>", which is also the
// casbin object it is checked as.
type ToolItem struct {
	Name  string `json:"name"`
	Group string `json:"group"`
	// Label names an item the UI has no wording of its own for, such as one MCP
	// server of this agent. The catalogue's own items are named by the page.
	Label string `json:"label,omitempty"`
	// Tools are the names this covers, as the agents write them. They are shown
	// under the switch, since every agent names the same tool differently.
	Tools []string `json:"tools"`
}

// ToolGroup is one group of switches, which can also be set in one go.
type ToolGroup struct {
	Name  string     `json:"name"`
	Items []ToolItem `json:"items"`
}

func item(group string, name string, tools ...string) ToolItem {
	if tools == nil {
		tools = []string{}
	}
	return ToolItem{Name: group + "/" + name, Group: group, Tools: tools}
}

// toolCatalog is every switch there is. The order is the order they are drawn
// in, and the "other" item of a group always comes last.
var toolCatalog = []ToolItem{
	item(GroupShell, "run", "Bash", "shell", "run_shell_command", "run_terminal_cmd", "execute_command"),
	item(GroupShell, "output", "BashOutput"),
	item(GroupShell, "kill", "KillShell", "KillBash"),
	item(GroupShell, otherItem),

	item(GroupRead, "file", "Read", "read_file", "view"),
	item(GroupRead, "many", "read_many_files"),
	item(GroupRead, "image", "view_image", "ReadImage"),
	item(GroupRead, "list", "LS", "list_directory", "list_dir"),
	item(GroupRead, "find", "Glob", "glob"),
	item(GroupRead, "grep", "Grep", "grep_search", "search_file_content"),
	item(GroupRead, "semantic", "codebase_search"),
	item(GroupRead, "notebook", "NotebookRead"),
	item(GroupRead, otherItem),

	item(GroupWrite, "create", "Write", "write_file", "create_file"),
	item(GroupWrite, "edit", "Edit", "edit_file", "replace", "str_replace_editor"),
	item(GroupWrite, "multi", "MultiEdit"),
	item(GroupWrite, "patch", "apply_patch"),
	item(GroupWrite, "notebook", "NotebookEdit"),
	item(GroupWrite, "delete", "delete_file", "remove_file"),
	item(GroupWrite, "move", "move_file", "rename_file"),
	item(GroupWrite, "mkdir", "create_directory", "mkdir"),
	item(GroupWrite, otherItem),

	item(GroupNetwork, "fetch", "WebFetch", "web_fetch", "fetch"),
	item(GroupNetwork, "search", "WebSearch", "web_search", "google_web_search"),
	item(GroupNetwork, "browser", "browser_navigate", "browser_click", "playwright"),
	item(GroupNetwork, otherItem),

	item(GroupAgentic, "subagent", "Task", "Agent"),
	item(GroupAgentic, "todo", "TodoWrite", "TodoRead", "update_plan"),
	item(GroupAgentic, "plan", "ExitPlanMode"),
	item(GroupAgentic, "ask", "AskUserQuestion"),
	item(GroupAgentic, "command", "SlashCommand"),
	item(GroupAgentic, "skill", "Skill"),
	item(GroupAgentic, "memory", "save_memory"),

	// One switch per MCP server is added to this group per agent, from the
	// servers that agent has installed; this one covers the rest.
	item(GroupMcp, otherItem),
}

// groupOrder is the order the groups are drawn in.
var groupOrder = []string{GroupShell, GroupRead, GroupWrite, GroupNetwork, GroupAgentic, GroupMcp}

// ToolGroups is the catalogue as the UI draws it, with the extra items an agent
// has of its own - its MCP servers - added to the group they belong to.
func ToolGroups(extra []ToolItem) []ToolGroup {
	groups := []ToolGroup{}
	for _, name := range groupOrder {
		group := ToolGroup{Name: name, Items: []ToolItem{}}
		for _, entry := range toolCatalog {
			if entry.Group != name || entry.isOther() {
				continue
			}
			group.Items = append(group.Items, entry)
		}
		for _, entry := range extra {
			if entry.Group == name {
				group.Items = append(group.Items, entry)
			}
		}
		// The catch-all always comes last, whatever was added above it.
		for _, entry := range toolCatalog {
			if entry.Group == name && entry.isOther() {
				group.Items = append(group.Items, entry)
			}
		}
		groups = append(groups, group)
	}
	return groups
}

func (entry ToolItem) isOther() bool {
	return strings.HasSuffix(entry.Name, "/"+otherItem)
}

// McpServerItem is the switch for one MCP server an agent has installed. The
// tools of that server arrive named "mcp__<server>__<tool>", so the item is
// named after the server and matches every tool it offers.
func McpServerItem(server string) ToolItem {
	return ToolItem{
		Name:  GroupMcp + "/" + normalizeToolName(strings.ToLower(server)),
		Group: GroupMcp,
		Label: server,
		Tools: []string{"mcp__" + server + "__*"},
	}
}

// McpToolItem is the switch for one tool of one MCP server, which exists only
// once a connection has been tested and said what it offers. It sits under that
// server's own switch and takes precedence over it, so a server can be left on
// with one of its tools taken away, or off with one of them left.
func McpToolItem(server string, tool string) ToolItem {
	return ToolItem{
		Name:  GroupMcp + "/" + normalizeToolName(strings.ToLower(server)) + "/" + normalizeToolName(strings.ToLower(tool)),
		Group: GroupMcp,
		Label: server + " · " + tool,
		Tools: []string{"mcp__" + server + "__" + tool},
	}
}

// McpToolItemOf names the switch one MCP tool would have of its own, which is
// not the same as there being one: the caller decides what to do when nobody
// has set it.
func McpToolItemOf(name string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	server, tool, ok := mcpPartsOf(lower)
	if !ok || server == "" || tool == "" {
		return "", false
	}
	return GroupMcp + "/" + server + "/" + tool, true
}

// CatalogItemNames is every switch of the built-in catalogue, which is what a
// stored permission is read against.
func CatalogItemNames() []string {
	names := []string{}
	for _, entry := range toolCatalog {
		names = append(names, entry.Name)
	}
	return names
}

// toolIndex maps a tool's name, normalized, to the item it falls under. It is
// built from the catalogue so the two can never drift apart.
var (
	toolIndexOnce sync.Once
	toolIndex     map[string]string
)

func itemOfTool(normalized string) (string, bool) {
	toolIndexOnce.Do(func() {
		toolIndex = map[string]string{}
		for _, entry := range toolCatalog {
			for _, tool := range entry.Tools {
				toolIndex[normalizeToolName(strings.ToLower(tool))] = entry.Name
			}
		}
	})
	name, found := toolIndex[normalized]
	return name, found
}

// toolKeywords catch the tools of an agent Gateway has never seen. A tool they
// match belongs to that group's "other" item, in the order tried here.
var toolKeywords = []struct {
	group    string
	keywords []string
}{
	{GroupNetwork, []string{"web", "http", "url", "browser", "curl", "download", "internet", "google", "fetch"}},
	{GroupShell, []string{"bash", "shell", "terminal", "command", "cmd", "exec", "process", "script"}},
	{GroupWrite, []string{"write", "edit", "patch", "create", "delete", "remove", "move", "rename", "insert", "replace", "mkdir"}},
	{GroupRead, []string{"read", "view", "cat", "open", "glob", "grep", "search", "find", "list", "dir", "tree"}},
}

// ToolItemOf is the switch one tool is held to, or "" for a tool that fits none
// of them and is therefore always allowed.
func ToolItemOf(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return ""
	}

	// An MCP tool is named after the server it came from, whichever agent is
	// asking: it is the one item a name says outright.
	if server, ok := mcpServerOf(lower); ok {
		if server == "" {
			return GroupMcp + "/" + otherItem
		}
		return GroupMcp + "/" + server
	}

	normalized := normalizeToolName(lower)
	if entry, found := itemOfTool(normalized); found {
		return entry
	}

	for _, entry := range toolKeywords {
		for _, keyword := range entry.keywords {
			if strings.Contains(normalized, keyword) {
				return entry.group + "/" + otherItem
			}
		}
	}
	return ""
}

// mcpServerOf reads the server out of an MCP tool name, which the agents write
// as "mcp__<server>__<tool>".
func mcpServerOf(lower string) (string, bool) {
	server, _, ok := mcpPartsOf(lower)
	return server, ok
}

// mcpPartsOf splits an MCP tool name into the server and the tool. A name that
// carries no tool part yields an empty one rather than a failure: it is still
// that server's, which is what the server's own switch is for.
func mcpPartsOf(lower string) (string, string, bool) {
	for _, prefix := range []string{"mcp__", "mcp_", "mcp."} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		rest := strings.TrimPrefix(lower, prefix)
		for _, separator := range []string{"__", "_", "."} {
			if index := strings.Index(rest, separator); index > 0 {
				return normalizeToolName(rest[:index]), normalizeToolName(rest[index:]), true
			}
		}
		return normalizeToolName(rest), "", true
	}
	return "", "", false
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

// FilterTools takes the tools this agent may not use back out of a request
// body, and reports the ones it removed. The body is left untouched when there
// is nothing to remove, so a request that changes nothing is relayed byte for
// byte as before.
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
	for _, entry := range items {
		filtered, names := guard.filterTool(entry)
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
func (guard *AgentGuard) filterTool(entry json.RawMessage) (json.RawMessage, []string) {
	fields := struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
		FunctionDeclarations []json.RawMessage `json:"functionDeclarations"`
	}{}
	if err := json.Unmarshal(entry, &fields); err != nil {
		return entry, nil
	}

	// Gemini wraps its tools in one entry, so the entry stays as long as any of
	// the declarations inside it does.
	if fields.FunctionDeclarations != nil {
		return guard.filterDeclarations(entry, fields.FunctionDeclarations)
	}

	name := fields.Name
	if name == "" && fields.Function != nil {
		name = fields.Function.Name
	}
	if name == "" {
		// A built-in tool of the API itself, such as OpenAI's web search, is
		// named by its type alone.
		name = fields.Type
	}
	if name == "" || guard.AllowTool(name) {
		return entry, nil
	}
	return nil, []string{name}
}

func (guard *AgentGuard) filterDeclarations(entry json.RawMessage, declarations []json.RawMessage) (json.RawMessage, []string) {
	kept := []json.RawMessage{}
	removed := []string{}
	for _, declaration := range declarations {
		named := struct {
			Name string `json:"name"`
		}{}
		if err := json.Unmarshal(declaration, &named); err != nil || guard.AllowTool(named.Name) {
			kept = append(kept, declaration)
			continue
		}
		removed = append(removed, named.Name)
	}
	if len(removed) == 0 {
		return entry, nil
	}
	if len(kept) == 0 {
		return nil, removed
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(entry, &fields); err != nil {
		return entry, nil
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return entry, nil
	}
	fields["functionDeclarations"] = encoded
	rewritten, err := json.Marshal(fields)
	if err != nil {
		return entry, nil
	}
	return rewritten, removed
}
