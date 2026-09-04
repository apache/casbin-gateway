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

package agentconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/apache/casbin-gateway/agenthome"
)

// ManagedEntryName is the MCP server Gateway registers for its own monitoring.
// It is listed like any other so an operator can see it, but it is never
// migrated or deleted from here: turning monitoring off is what removes it.
const ManagedEntryName = "casbin-gateway-agent-monitor"

// layout is where one agent keeps its skills, its MCP servers and the
// instructions it reads at the start of every session. A nil part means the
// agent has no such location that Gateway knows about, which the UI shows as
// unsupported rather than as empty.
type layout struct {
	skills *skillLayout
	mcp    *mcpLayout
	prompt *promptLayout
}

// Where a skill came from. A plugin's skills are read like any other but are
// not Gateway's to delete: the plugin owns them. A project's skills belong to a
// checkout the agent has been run in, not to the account.
const (
	ScopeUser    = "user"
	ScopePlugin  = "plugin"
	ScopeProject = "project"
)

// skillLayout is every place one agent finds skills. Each is a directory of
// skill folders with a SKILL.md, which is what makes copying a skill between
// two agents a plain directory copy.
type skillLayout struct {
	sources []skillSource
}

// skillSource is one such place: the agent's own skills directory, a plugin
// tree that is searched for the skills folders the plugins ship, or the same
// directory inside every project the agent has been run in.
type skillSource struct {
	segments []string
	scope    string
	scan     bool
	// projects lists the checkouts to look in instead of the home directory.
	// A project keeps its skills beside its other agent configuration, and the
	// agent reads them there, so a listing without them is short.
	projects func(home string) []string
}

func (s skillSource) dir(home string) string {
	return filepath.Join(append([]string{home}, s.segments...)...)
}

// roots is every directory this source contributes: one under the home
// directory, or one under each project the agent has worked in.
func (s skillSource) roots(home string) []string {
	if s.projects == nil {
		return []string{s.dir(home)}
	}

	roots := []string{}
	for _, project := range s.projects(home) {
		roots = append(roots, filepath.Join(append([]string{project}, s.segments...)...))
	}
	return roots
}

// dir is where Gateway writes a skill it copies into this agent: the agent's
// own skills directory, never a plugin's.
func (l *skillLayout) dir(home string) string {
	for _, source := range l.sources {
		if source.scope == ScopeUser && !source.scan && source.projects == nil {
			return source.dir(home)
		}
	}
	return ""
}

func userSkills(segments ...string) skillSource {
	return skillSource{segments: segments, scope: ScopeUser}
}

func pluginSkills(segments ...string) skillSource {
	return skillSource{segments: segments, scope: ScopePlugin, scan: true}
}

// projectSkills reads the same folder inside every project the agent lists in
// its own configuration, which is where a skill checked into a repository sits.
func projectSkills(projects func(home string) []string, segments ...string) skillSource {
	return skillSource{segments: segments, scope: ScopeProject, projects: projects}
}

// mcpLayout is one config file holding named MCP server entries.
type mcpLayout struct {
	file  func(home string) string
	store mcpStore
	// readOnly explains, in the operator's words, why Gateway will not write
	// this file. Empty for the files it may write.
	readOnly string
}

func (l *mcpLayout) path(home string) string {
	return l.file(home)
}

// promptLayout is the one Markdown file an agent reads before every session.
// The agents spell it differently — CLAUDE.md, AGENTS.md, global_rules.md — and
// hold the same thing in it.
type promptLayout struct {
	segments []string
}

func (l *promptLayout) path(home string) string {
	return filepath.Join(append([]string{home}, l.segments...)...)
}

// promptFile names one agent's instruction file, last segment first so the file
// name reads before the directory it sits in.
func promptFile(name string, dir ...string) *promptLayout {
	return &promptLayout{segments: append(dir, name)}
}

// mcpStore is the file format half: JSON objects for most agents, TOML tables
// for Codex. Entries are keyed by server name and hold that server's own
// fields, in whatever spelling the file used.
type mcpStore interface {
	read(file string) (map[string]map[string]any, error)
	write(file string, name string, entry map[string]any) error
	remove(file string, name string) error
}

// layouts is the whole of Gateway's per-agent knowledge. The Codex CLI, the
// Codex VS Code extension and the ChatGPT desktop app share one CODEX_HOME, and
// Cursor and its CLI share one ~/.cursor, so those ids share a layout.
var layouts = map[string]layout{
	"claude-code": {
		skills: &skillLayout{sources: []skillSource{
			userSkills(".claude", "skills"),
			pluginSkills(".claude", "plugins"),
			projectSkills(jsonProjects(".claude.json"), ".claude", "skills"),
		}},
		mcp:    &mcpLayout{file: under(".claude.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
		prompt: promptFile("CLAUDE.md", ".claude"),
	},
	"claude-desktop": {
		mcp: &mcpLayout{file: claudeDesktopConfig, store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
	},
	"codex":        codexLayout,
	"codex-cli":    codexLayout,
	"cursor":       cursorLayout,
	"cursor-agent": cursorLayout,
	// The Gemini CLI reads the shared ~/.agents skills too, and prefers them
	// over its own; writes go to its own directory, which is the first user
	// source.
	"gemini-cli": {
		skills: &skillLayout{sources: []skillSource{
			userSkills(".gemini", "skills"),
			userSkills(".agents", "skills"),
			pluginSkills(".gemini", "extensions"),
		}},
		mcp:    &mcpLayout{file: under(".gemini", "settings.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
		prompt: promptFile("GEMINI.md", ".gemini"),
	},
	// Qwen Code is a Gemini CLI fork and keeps the same layout under its own
	// directory, the shared ~/.agents skills included.
	"qwen-code": {
		skills: &skillLayout{sources: []skillSource{
			userSkills(".qwen", "skills"),
			userSkills(".agents", "skills"),
			pluginSkills(".qwen", "extensions"),
		}},
		mcp:    &mcpLayout{file: under(".qwen", "settings.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
		prompt: promptFile("QWEN.md", ".qwen"),
	},
	"iflow-cli": {
		mcp:    &mcpLayout{file: under(".iflow", "settings.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
		prompt: promptFile("IFLOW.md", ".iflow"),
	},
	// Cline's IDE extension, CLI and SDK share one ~/.cline: the skills sit
	// directly under it, the MCP servers with the rest of the app state.
	"cline": {
		skills: &skillLayout{sources: []skillSource{
			userSkills(".cline", "skills"),
			userSkills(".agents", "skills"),
			pluginSkills(".cline", "plugins"),
		}},
		mcp: &mcpLayout{
			file:  under(".cline", "data", "settings", "cline_mcp_settings.json"),
			store: &jsonStore{paths: [][]string{{"mcpServers"}}},
		},
	},
	"windsurf": {
		mcp: &mcpLayout{
			file:  under(".codeium", "windsurf", "mcp_config.json"),
			store: &jsonStore{paths: [][]string{{"mcpServers"}}},
		},
		prompt: promptFile("global_rules.md", ".codeium", "windsurf", "memories"),
	},
	// dsh reads the shared ~/.agents skills too, but writes go to its own
	// directory, which is the first user source.
	"dsh": {
		skills: &skillLayout{sources: []skillSource{
			userSkills(".dsh", "skills"),
			userSkills(".agents", "skills"),
		}},
		mcp: &mcpLayout{file: under(".dsh", "cordis.patch.yml"), store: &cordisStore{}},
	},
	"openclaw": {
		skills: &skillLayout{sources: []skillSource{
			userSkills(".openclaw", "skills"),
			pluginSkills(".openclaw", "plugins"),
			projectSkills(jsonProjects(".openclaw", "openclaw.json"), ".openclaw", "skills"),
		}},
		mcp: &mcpLayout{
			file: under(".openclaw", "openclaw.json"),
			// OpenClaw has spelled this three ways across versions, and reads
			// all three; the first is where a new entry goes.
			store: &jsonStore{paths: [][]string{{"mcp", "servers"}, {"mcp", "mcpServers"}, {"mcpServers"}}},
		},
		prompt: promptFile("AGENTS.md", ".openclaw"),
	},
	"opencode":         opencodeLayout,
	"opencode-desktop": opencodeLayout,
}

var codexLayout = layout{
	skills: &skillLayout{sources: []skillSource{
		userSkills(".codex", "skills"),
		pluginSkills(".codex", "plugins"),
	}},
	mcp:    &mcpLayout{file: under(".codex", "config.toml"), store: &tomlStore{table: "mcp_servers"}},
	prompt: promptFile("AGENTS.md", ".codex"),
}

// The opencode CLI and its desktop app read one ~/.config/opencode, on every
// platform, so the two ids share a layout.
var opencodeLayout = layout{
	skills: &skillLayout{sources: []skillSource{
		userSkills(".config", "opencode", "skill"),
		userSkills(".config", "opencode", "skills"),
	}},
	mcp:    &mcpLayout{file: opencodeConfig, store: newOpencodeStore()},
	prompt: promptFile("AGENTS.md", ".config", "opencode"),
}

var cursorLayout = layout{
	skills: &skillLayout{sources: []skillSource{
		userSkills(".cursor", "skills-cursor"),
		pluginSkills(".cursor", "plugins"),
	}},
	mcp: &mcpLayout{file: under(".cursor", "mcp.json"), store: &jsonStore{paths: [][]string{{"mcpServers"}}}},
}

func layoutOf(agentId string) (layout, bool) {
	found, ok := layouts[agentId]
	return found, ok
}

// Supports reports whether Gateway knows where agentId keeps either kind of
// configuration, so a caller can skip an installation before resolving a home.
func Supports(agentId string) bool {
	_, ok := layouts[agentId]
	return ok
}

// McpConfigPath is where agentId keeps its MCP servers, for the account whose
// home directory is home. Callers outside this package use it to edit one entry
// of a file this package otherwise owns.
func McpConfigPath(agentId string, home string) (string, bool) {
	found, ok := layouts[agentId]
	if !ok || found.mcp == nil {
		return "", false
	}
	return found.mcp.path(home), true
}

func under(segments ...string) func(string) string {
	return func(home string) string {
		return filepath.Join(append([]string{home}, segments...)...)
	}
}

// opencodeConfig is the config file to edit. opencode reads config.json,
// opencode.json and opencode.jsonc in that order and merges them, so they are
// tried here in reverse: the last one present is the one whose settings win. A
// home with none of them gets the documented name.
func opencodeConfig(home string) string {
	dir := filepath.Join(home, ".config", "opencode")
	for _, name := range []string{"opencode.jsonc", "opencode.json", "config.json"} {
		if path := filepath.Join(dir, name); exists(path) {
			return path
		}
	}
	return filepath.Join(dir, "opencode.json")
}

func claudeDesktopConfig(home string) string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	default:
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}
}

func homeOf(owner string) (string, error) {
	return agenthome.Resolve(owner)
}

// KnownAgents lists the agent ids Gateway has a layout for, in a stable order.
func KnownAgents() []string {
	ids := make([]string, 0, len(layouts))
	for id := range layouts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Configured reports whether agentId has either of its locations on disk for
// owner. An agent can be set up on an account without Gateway recognizing an
// installation of it, and those skills are just as real as any other.
func Configured(agentId string, owner string) bool {
	found, ok := layouts[agentId]
	if !ok {
		return false
	}
	home, err := homeOf(owner)
	if err != nil {
		return false
	}

	if found.skills != nil {
		for _, source := range found.skills.sources {
			for _, root := range source.roots(home) {
				if exists(root) {
					return true
				}
			}
		}
	}
	if found.mcp != nil && exists(found.mcp.path(home)) {
		return true
	}
	return found.prompt != nil && exists(found.prompt.path(home))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
