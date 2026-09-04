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

package object

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/apache/casbin-gateway/agent"
)

// The resources an "add this to Gateway" link can carry. The names are CC
// Switch's, because the links Gateway reads are the ones vendors already
// publish for it.
const (
	ImportResourceProvider = "provider"
	ImportResourceMcp      = "mcp"
	ImportResourcePrompt   = "prompt"
	ImportResourceSkill    = "skill"
)

// importLinkAgents translates the app names CC Switch links use into Gateway's
// own agent ids. Only the ones that differ are here: an app a link names by an
// id Gateway already knows resolves on its own, so every agent in the catalogue
// can be the target of a link without an entry of its own.
//
// Two ids appear where the same files are read under both — the Codex CLI and
// ChatGPT Desktop share ~/.codex, Cursor and its CLI share ~/.cursor, opencode
// and its desktop app share ~/.config/opencode — because the listing keeps one
// entry per location, under whichever of the two was found first.
var importLinkAgents = map[string][]string{
	"claude":   {"claude-code"},
	"codex":    {"codex-cli", "codex"},
	"cursor":   {"cursor", "cursor-agent"},
	"gemini":   {"gemini-cli"},
	"hermes":   {"hermes-agent"},
	"iflow":    {"iflow-cli"},
	"kimi":     {"kimi-code"},
	"opencode": {"opencode", "opencode-desktop"},
	"qwen":     {"qwen-code"},
}

// ImportLink is what one link carries, read and handed to the page. Nothing in
// it is stored: a link like this arrives from a website, so the person it
// arrives at sees the values before any of them are written.
type ImportLink struct {
	Resource string        `json:"resource"`
	Provider *Provider     `json:"provider,omitempty"`
	Mcp      *McpImport    `json:"mcp,omitempty"`
	Prompt   *PromptImport `json:"prompt,omitempty"`
	Skill    *SkillImport  `json:"skill,omitempty"`
}

// McpImport is one or more MCP servers, kept as the JSON block the link
// carried. Reading that block is the browser's job: the page already knows
// every shape a server is written in, and this way the reader sees the same
// text the link held.
type McpImport struct {
	Name   string `json:"name"`
	Config string `json:"config"`
	// Targets are the agents the link asks for, as ids of this host's agents.
	Targets []string `json:"targets"`
	// Unknown are the apps it named that Gateway does not manage.
	Unknown []string `json:"unknown"`
}

// PromptImport is a set of instructions for an agent to read before every
// session. Applying it replaces the file the agent has, which is why the
// content travels to the page in full.
type PromptImport struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Targets     []string `json:"targets"`
	Unknown     []string `json:"unknown"`
}

// SkillImport is a place to install skills from, in the shape agentconfig
// records a GitHub source in.
type SkillImport struct {
	Name   string `json:"name"`
	Repo   string `json:"repo"`
	Ref    string `json:"ref"`
	Subdir string `json:"subdir"`
}

// ParseImportLink reads one link and answers with what it would fill in. It
// stores nothing and reaches no agent: every resource here is applied by the
// page, through the same endpoint that adds one by hand.
//
// The parameters Gateway has no use for are ignored rather than refused, so a
// link written for a fuller CC Switch still imports the part that means
// something here: a provider's whole config file, the script it queries a
// balance with, and the flags that would activate what was imported — Gateway
// asks the reader that instead.
func ParseImportLink(owner string, raw string) (*ImportLink, error) {
	// New API sites hand out connection info as JSON rather than a link; it is
	// pasted into the same box, so it is read here.
	if provider, isChannel, err := providerFromNewApiChannel(owner, raw); isChannel {
		if err != nil {
			return nil, err
		}
		return &ImportLink{Resource: ImportResourceProvider, Provider: provider}, nil
	}

	query, err := importLinkQuery(raw)
	if err != nil {
		return nil, err
	}

	link := &ImportLink{Resource: query.Get("resource")}
	switch link.Resource {
	case ImportResourceProvider:
		link.Provider, err = providerFromLink(owner, query)
	case ImportResourceMcp:
		link.Mcp, err = mcpFromLink(query)
	case ImportResourcePrompt:
		link.Prompt, err = promptFromLink(query)
	case ImportResourceSkill:
		link.Skill, err = skillFromLink(query)
	default:
		return nil, fmt.Errorf("this link carries a %q, which Gateway cannot import", link.Resource)
	}
	if err != nil {
		return nil, err
	}
	return link, nil
}

// importLinkQuery checks the envelope every link shares and returns what it
// carries.
func importLinkQuery(raw string) (url.Values, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("this is not a link: %w", err)
	}
	if parsed.Scheme != ImportLinkScheme {
		return nil, fmt.Errorf("an import link starts with %q, not %q", ImportLinkScheme+"://", parsed.Scheme)
	}
	if parsed.Host != "v1" {
		return nil, fmt.Errorf("unsupported link version: %s", parsed.Host)
	}
	if strings.Trim(parsed.Path, "/") != "import" {
		return nil, fmt.Errorf("unsupported link path: %s", parsed.Path)
	}
	return parsed.Query(), nil
}

func mcpFromLink(query url.Values) (*McpImport, error) {
	config, err := linkJson("config", query.Get("config"))
	if err != nil {
		return nil, err
	}

	targets, unknown := linkTargets(strings.Split(query.Get("apps"), ","))
	return &McpImport{
		Name:    strings.TrimSpace(query.Get("name")),
		Config:  config,
		Targets: targets,
		Unknown: unknown,
	}, nil
}

func promptFromLink(query url.Values) (*PromptImport, error) {
	name := strings.TrimSpace(query.Get("name"))
	if name == "" {
		return nil, fmt.Errorf("the link does not say what to call these instructions")
	}
	content := linkText(query.Get("content"))
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("the link carries no instructions")
	}

	targets, unknown := linkTargets([]string{query.Get("app")})
	return &PromptImport{
		Name:        name,
		Description: strings.TrimSpace(query.Get("description")),
		Content:     content,
		Targets:     targets,
		Unknown:     unknown,
	}, nil
}

func skillFromLink(query url.Values) (*SkillImport, error) {
	repo := strings.Trim(strings.TrimSpace(query.Get("repo")), "/")
	repoOwner, repoName, found := strings.Cut(repo, "/")
	if !found || repoOwner == "" || repoName == "" || strings.Contains(repoName, "/") {
		return nil, fmt.Errorf("a skill link names its repository as owner/name, not %q", repo)
	}

	ref := strings.TrimSpace(query.Get("branch"))
	if err := checkLinkPath("branch", ref); err != nil {
		return nil, err
	}
	subdir := strings.Trim(strings.TrimSpace(query.Get("directory")), "/")
	if err := checkLinkPath("directory", subdir); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(query.Get("name"))
	if name == "" {
		name = repo
	}
	return &SkillImport{Name: name, Repo: repo, Ref: ref, Subdir: subdir}, nil
}

// checkLinkPath keeps the parts of a skill link that end up in a download URL
// or a path under Gateway's store to what they are meant to name. A branch is
// pasted into the address an archive is fetched from, where a "../" would
// resolve to a different file on the same host — which nobody writing down a
// branch name ever needs.
func checkLinkPath(field string, value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, `\`) {
		return fmt.Errorf("the %s in this link is not a name: %q", field, value)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("the %s in this link is not a name: %q", field, value)
		}
	}
	for _, r := range value {
		if r <= ' ' || r == 0x7f || strings.ContainsRune(`%?#"'`, r) {
			return fmt.Errorf("the %s in this link holds a character a name cannot: %q", field, value)
		}
	}
	return nil
}

// linkTargets turns the apps a link names into the agents on this host that
// read their configuration, and reports the ones Gateway does not manage.
func linkTargets(apps []string) (targets []string, unknown []string) {
	targets, unknown = []string{}, []string{}
	seen := map[string]bool{}
	for _, app := range apps {
		app = strings.ToLower(strings.TrimSpace(app))
		if app == "" {
			continue
		}
		agentIds, known := importLinkAgents[app]
		if !known && agent.IsKnownAgentId(app) {
			agentIds, known = []string{app}, true
		}
		if !known {
			unknown = append(unknown, app)
			continue
		}
		for _, agentId := range agentIds {
			if !seen[agentId] {
				seen[agentId] = true
				targets = append(targets, agentId)
			}
		}
	}
	return targets, unknown
}

// linkText reads a parameter carrying a document rather than a word. The format
// base64-encodes those, and encoders disagree about padding and about which
// alphabet, so all four are tried; a value that is not base64 is taken as the
// text it already is, which is what a link written by hand carries. Guessing
// wrong is visible either way: the page shows what was read before it is
// written anywhere.
func linkText(raw string) string {
	value := strings.TrimSpace(raw)
	if decoded, ok := decodeLinkBase64(value); ok {
		return decoded
	}
	return value
}

// linkJson reads a parameter carrying JSON, which is the one case where a wrong
// guess about base64 can be ruled out rather than shown.
func linkJson(field string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("the link carries no %s", field)
	}
	if decoded, ok := decodeLinkBase64(value); ok && json.Valid([]byte(decoded)) {
		return decoded, nil
	}
	if !json.Valid([]byte(value)) {
		return "", fmt.Errorf("the %s in this link is not JSON", field)
	}
	return value, nil
}

func decodeLinkBase64(value string) (string, bool) {
	if value == "" {
		return "", false
	}

	candidates := []string{value}
	// A "+" of the standard alphabet arrives as a space when the link was put
	// together without escaping it.
	if strings.Contains(value, " ") {
		candidates = append([]string{strings.ReplaceAll(value, " ", "+")}, candidates...)
	}
	for _, candidate := range candidates {
		for _, encoding := range []*base64.Encoding{
			base64.StdEncoding, base64.RawStdEncoding,
			base64.URLEncoding, base64.RawURLEncoding,
		} {
			decoded, err := encoding.DecodeString(candidate)
			if err == nil && utf8.Valid(decoded) {
				return string(decoded), true
			}
		}
	}
	return "", false
}
