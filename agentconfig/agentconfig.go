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

// Package agentconfig reads and edits the skills, MCP servers and instruction
// files that AI coding agents keep in their own configuration files, so the
// agents installed on one host can be compared and their configuration moved
// between them.
//
// Every agent stores the same three things in a different place and a different
// format. layout.go holds that per-agent knowledge, and nothing else does.
package agentconfig

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// The kinds of configuration Gateway manages.
const (
	KindSkill = "skill"
	KindMcp   = "mcp"
	// KindPrompt is the Markdown file an agent reads before every session.
	KindPrompt = "prompt"
)

// ErrUnsupported reports a kind an agent has no known location for, which is
// different from an agent that has one and it is empty.
var ErrUnsupported = errors.New("Gateway does not know where this agent keeps that configuration")

// Item is one skill, MCP server or instruction file as it exists in an agent's
// own configuration.
type Item struct {
	AgentId string `json:"agentId"`
	Owner   string `json:"owner"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`

	// Shared is the name two agents' copies of this item are matched by, when
	// that is not the name itself. Every agent's instruction file holds the same
	// thing under a different file name.
	Shared string `json:"shared,omitempty"`

	Description string `json:"description,omitempty"`

	// Path is what an edit would change: the skill's own directory, the config
	// file the MCP server is one entry of, or the instruction file itself.
	Path string `json:"path"`

	// Transport, Command and Url summarize an MCP server in a list.
	Transport string `json:"transport,omitempty"`
	Command   string `json:"command,omitempty"`
	Url       string `json:"url,omitempty"`

	// Files and Bytes summarize a skill in a list.
	Files int   `json:"files,omitempty"`
	Bytes int64 `json:"bytes,omitempty"`

	// Link is what a skill installed as a symbolic link points at, empty for a
	// skill that is a folder of its own. A linked skill follows its source
	// wherever that goes, which is the difference the listing has to show.
	Link string `json:"link,omitempty"`

	// Scope tells a skill the operator wrote from one a plugin ships or one a
	// project checkout carries, and Origin names that plugin or project.
	Scope  string `json:"scope,omitempty"`
	Origin string `json:"origin,omitempty"`
	// Project is the checkout a project-scope skill belongs to.
	Project string `json:"project,omitempty"`

	// Digest identifies the content, and Modified is the newest file behind it.
	// Two agents holding the same name with different digests hold different
	// versions of it, and the newer Modified is the newer one.
	Digest   string `json:"digest,omitempty"`
	Modified int64  `json:"modified,omitempty"`

	// Update is where a skill came from and whether that source still holds the
	// same content, which is what tells an out-of-date copy from a current one.
	Update *SkillUpdate `json:"update,omitempty"`

	// Managed marks an entry Gateway wrote itself, which is not the operator's
	// to migrate and is removed by turning monitoring off instead.
	Managed bool `json:"managed,omitempty"`
	// ReadOnly explains why Gateway will not delete this item. Empty for the
	// items it may.
	ReadOnly string `json:"readOnly,omitempty"`
	// Missing marks an item listed because the agent would read it, not because
	// it is there. An instruction file nobody has written is still the answer to
	// what that agent is told.
	Missing bool `json:"missing,omitempty"`
}

// Inventory is everything Gateway can read for one installation. A location it
// cannot read leaves the corresponding list empty and adds to Errors, so one
// unreadable file never hides the rest of the host.
type Inventory struct {
	AgentId string `json:"agentId"`
	Owner   string `json:"owner"`
	// Home is the account directory the locations below were resolved under.
	// Two agents can be compared and copied between exactly when they share it.
	Home string `json:"home,omitempty"`

	// SkillsDir is where Gateway writes a skill copied into this agent, and
	// SkillsDirs every directory the listing was read from, plugins included.
	SkillsDir  string   `json:"skillsDir,omitempty"`
	SkillsDirs []string `json:"skillsDirs,omitempty"`
	McpFile    string   `json:"mcpFile,omitempty"`
	PromptFile string   `json:"promptFile,omitempty"`

	SkillsSupported bool `json:"skillsSupported"`
	McpSupported    bool `json:"mcpSupported"`
	PromptSupported bool `json:"promptSupported"`
	// McpWritable is false for a config Gateway can read but must not rewrite,
	// because writing it back would lose something it cannot represent.
	McpWritable bool   `json:"mcpWritable"`
	McpReadOnly string `json:"mcpReadOnly,omitempty"`

	Skills     []*Item  `json:"skills"`
	McpServers []*Item  `json:"mcpServers"`
	Prompts    []*Item  `json:"prompts"`
	Errors     []string `json:"errors,omitempty"`
}

// Detail is one item's full definition, for the viewer.
type Detail struct {
	Item *Item `json:"item"`
	// Content is the SKILL.md of a skill, the JSON of one MCP server entry, or
	// the instruction file itself.
	Content string `json:"content"`
	// Files lists the other files a skill directory carries.
	Files []string `json:"files,omitempty"`
}

// source is one item of the migration source, read once per Copy rather than
// once per target.
type source struct {
	item  *Item
	entry map[string]any
}

// Read returns one installation's inventory. It never fails: an unreadable
// location is reported in Errors, because a page listing every agent on the
// host must stay useful when one of them has a broken config file.
func Read(agentId string, owner string) *Inventory {
	inventory := &Inventory{
		AgentId:    agentId,
		Owner:      owner,
		Skills:     []*Item{},
		McpServers: []*Item{},
		Prompts:    []*Item{},
	}

	found, ok := layoutOf(agentId)
	if !ok {
		return inventory
	}
	inventory.SkillsSupported = found.skills != nil
	inventory.McpSupported = found.mcp != nil
	inventory.PromptSupported = found.prompt != nil
	if found.mcp != nil {
		inventory.McpWritable = found.mcp.readOnly == ""
		inventory.McpReadOnly = found.mcp.readOnly
	}

	home, err := homeOf(owner)
	if err != nil {
		inventory.Errors = append(inventory.Errors, err.Error())
		return inventory
	}
	inventory.Home = home

	if found.skills != nil {
		inventory.SkillsDir = found.skills.dir(home)
		scan := readSkills(agentId, owner, found.skills, home)
		attachSkillUpdates(scan.items, home)
		inventory.Skills = scan.items
		inventory.SkillsDirs = scan.dirs
		inventory.Errors = append(inventory.Errors, scan.problems...)
	}

	if found.mcp != nil {
		inventory.McpFile = found.mcp.path(home)
		entries, err := found.mcp.store.read(inventory.McpFile)
		if err != nil {
			inventory.Errors = append(inventory.Errors, err.Error())
		}
		inventory.McpServers = mcpItems(agentId, owner, inventory.McpFile, entries)
	}

	if found.prompt != nil {
		inventory.PromptFile = found.prompt.path(home)
		inventory.Prompts = []*Item{readPrompt(agentId, owner, inventory.PromptFile)}
	}
	return inventory
}

// ReadDetail returns one item's full definition.
func ReadDetail(agentId string, owner string, kind string, name string) (*Detail, error) {
	found, home, err := resolve(agentId, owner, kind)
	if err != nil {
		return nil, err
	}
	if kind == KindSkill || kind == KindPrompt {
		item, err := findItem(agentId, owner, kind, name)
		if err != nil {
			return nil, err
		}
		if kind == KindPrompt {
			return promptDetail(item)
		}
		return skillDetail(item)
	}

	file := found.mcp.path(home)
	entries, err := found.mcp.store.read(file)
	if err != nil {
		return nil, err
	}
	entry, ok := entries[name]
	if !ok {
		return nil, fmt.Errorf("%s: no MCP server named %q in %s", agentId, name, file)
	}
	return mcpDetail(agentId, owner, file, name, entry)
}

// Delete takes one item out of the agent's own configuration. Nothing is erased
// here: the item is moved into Gateway's trash first, so a delete made by
// mistake can be restored.
func Delete(agentId string, owner string, kind string, name string) error {
	found, home, err := resolve(agentId, owner, kind)
	if err != nil {
		return err
	}
	item, err := findItem(agentId, owner, kind, name)
	if err != nil {
		return err
	}
	if item.ReadOnly != "" {
		return fmt.Errorf("%s: %s", name, item.ReadOnly)
	}

	if kind == KindSkill {
		return trashSkill(home, item)
	}
	if kind == KindPrompt {
		return trashPath(home, item)
	}
	if found.mcp.readOnly != "" {
		return fmt.Errorf("%s: %s", agentId, found.mcp.readOnly)
	}

	file := found.mcp.path(home)
	entries, err := found.mcp.store.read(file)
	if err != nil {
		return err
	}
	if err := trashMcp(home, item, entries[name]); err != nil {
		return err
	}
	return found.mcp.store.remove(file, name)
}

// findItem locates one item in what the agent actually has. Skills are found by
// scanning rather than by joining the name onto a directory: a skill can come
// from a plugin or from a group inside the skills directory, and the path it
// was found at is the only one an edit may touch.
func findItem(agentId string, owner string, kind string, name string) (*Item, error) {
	if name == "" {
		return nil, errors.New("the name is empty")
	}
	for _, item := range itemsOf(Read(agentId, owner), kind) {
		if item.Name == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("%s: no %s named %q", agentId, kind, name)
}

// CopyRequest is one migration: some of a source agent's items, into one or
// more other agents belonging to the same account.
type CopyRequest struct {
	Owner     string   `json:"owner"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	Kind      string   `json:"kind"`
	Names     []string `json:"names"`
	Overwrite bool     `json:"overwrite"`
}

// What one planned copy would do to the target.
const (
	ActionCreate    = "create"
	ActionOverwrite = "overwrite"
	ActionSkip      = "skip"
	ActionFailed    = "failed"
)

// PlanItem is one item's fate at one target, decided before anything is written
// and reported again afterwards with what actually happened.
type PlanItem struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`
	Action  string `json:"action"`
	Reason  string `json:"reason,omitempty"`
	Path    string `json:"path,omitempty"`
}

// Plan reports what Copy would do, without touching anything.
func Plan(request CopyRequest) ([]*PlanItem, error) {
	sources, err := readSources(request)
	if err != nil {
		return nil, err
	}
	return plan(request, sources), nil
}

func plan(request CopyRequest, sources map[string]*source) []*PlanItem {
	planned := []*PlanItem{}
	for _, agentId := range request.To {
		existing, err := targetItems(agentId, request.Owner, request.Kind)
		for _, name := range request.Names {
			item := &PlanItem{AgentId: agentId, Name: name}
			from := sources[name]
			var target *Item
			if from != nil {
				target = existing[sharedKey(from.item)]
			}
			switch {
			case err != nil:
				item.Action, item.Reason = ActionSkip, err.Error()
			case from == nil:
				item.Action, item.Reason = ActionSkip, "not found in the source agent"
			case from.item.Missing:
				item.Action, item.Reason = ActionSkip, "there is nothing there to copy"
			case from.item.Managed:
				item.Action, item.Reason = ActionSkip, "written by Gateway, not migrated"
			case target == nil:
				item.Action = ActionCreate
			case sameContent(from.item, target):
				item.Action, item.Reason = ActionSkip, "already up to date"
			case request.Overwrite:
				item.Action, item.Reason = ActionOverwrite, replaces(from.item, target)
			default:
				item.Action, item.Reason = ActionSkip, "a different version is already there"
			}
			planned = append(planned, item)
		}
	}
	return planned
}

// sameContent reports two copies of one item as the same version. A digest is
// computed from the content, so equal digests mean the target already has what
// the copy would write.
func sameContent(from *Item, target *Item) bool {
	return from.Digest != "" && from.Digest == target.Digest
}

// replaces says which way round the two versions are, so an overwrite that
// would move an agent backwards is visible before it is applied.
func replaces(from *Item, target *Item) string {
	if from.Modified == 0 || target.Modified == 0 {
		return "replaces a different version"
	}
	if target.Modified > from.Modified {
		return "replaces a newer version"
	}
	return "replaces an older version"
}

// sharedKey is what one agent's copy of an item is matched to another's. Two
// agents' instruction files are the same item under two file names, so they say
// so rather than being matched by name.
func sharedKey(item *Item) string {
	if item.Shared != "" {
		return item.Shared
	}
	return targetName(item.Kind, item.Name)
}

// targetName is the name an item takes at the target. A skill from a plugin or
// from a group inside the skills directory is qualified, and it lands in the
// target's skills directory under the last part of that name.
func targetName(kind string, name string) string {
	if kind != KindSkill {
		return name
	}
	if index := strings.LastIndexAny(name, ":/"); index >= 0 {
		return name[index+1:]
	}
	return name
}

// Copy writes the selected items into every target agent. One item's failure is
// recorded against that item and the rest still run, so a partly finished
// migration is reported item by item instead of hidden behind the first error.
func Copy(request CopyRequest) ([]*PlanItem, error) {
	sources, err := readSources(request)
	if err != nil {
		return nil, err
	}

	planned := plan(request, sources)
	for _, item := range planned {
		if item.Action != ActionCreate && item.Action != ActionOverwrite {
			continue
		}
		path, err := writeItem(request, sources[item.Name], item.AgentId)
		if err != nil {
			item.Action, item.Reason = ActionFailed, err.Error()
			continue
		}
		item.Path = path
	}
	return planned, nil
}

// writeItem puts one already-read source item into one target agent.
func writeItem(request CopyRequest, from *source, agentId string) (string, error) {
	found, home, err := resolve(agentId, request.Owner, request.Kind)
	if err != nil {
		return "", err
	}

	if request.Kind == KindSkill {
		path, err := copySkill(from.item.Path, found.skills.dir(home), targetName(request.Kind, from.item.Name))
		if err != nil {
			return "", err
		}
		recordSkillOrigin(home, path, request.From, from.item.Name, from.item.Path)
		return path, nil
	}
	if request.Kind == KindPrompt {
		return copyPrompt(home, agentId, request.Owner, found.prompt.path(home), from.item)
	}
	if found.mcp.readOnly != "" {
		return "", errors.New(found.mcp.readOnly)
	}
	file := found.mcp.path(home)
	return file, found.mcp.store.write(file, from.item.Name, from.entry)
}

// readSources validates the request and loads the source agent's items once.
func readSources(request CopyRequest) (map[string]*source, error) {
	switch {
	case request.From == "":
		return nil, errors.New("the source agent is empty")
	case len(request.To) == 0:
		return nil, errors.New("no target agent was selected")
	case len(request.Names) == 0:
		return nil, errors.New("nothing was selected to copy")
	case request.Kind != KindSkill && request.Kind != KindMcp && request.Kind != KindPrompt:
		return nil, fmt.Errorf("unknown configuration kind: %s", request.Kind)
	}
	for _, agentId := range request.To {
		if agentId == request.From {
			return nil, errors.New("the source agent cannot also be a target")
		}
	}

	found, home, err := resolve(request.From, request.Owner, request.Kind)
	if err != nil {
		return nil, err
	}

	inventory := Read(request.From, request.Owner)
	if len(inventory.Errors) > 0 {
		return nil, errors.New(strings.Join(inventory.Errors, "; "))
	}

	entries := map[string]map[string]any{}
	if request.Kind == KindMcp {
		entries, err = found.mcp.store.read(found.mcp.path(home))
		if err != nil {
			return nil, err
		}
	}

	sources := map[string]*source{}
	for _, item := range itemsOf(inventory, request.Kind) {
		sources[item.Name] = &source{item: item, entry: entries[item.Name]}
	}
	return sources, nil
}

// targetItems is what the target already has, so planning can tell a new item
// from one that would be replaced, and an identical copy from an older one.
func targetItems(agentId string, owner string, kind string) (map[string]*Item, error) {
	if _, _, err := resolve(agentId, owner, kind); err != nil {
		return nil, err
	}

	inventory := Read(agentId, owner)
	if len(inventory.Errors) > 0 {
		return nil, errors.New(strings.Join(inventory.Errors, "; "))
	}

	items := map[string]*Item{}
	for _, item := range itemsOf(inventory, kind) {
		// A listed instruction file the target has not written is nothing to
		// replace, so a copy into it creates rather than overwrites.
		if item.Missing {
			continue
		}
		items[sharedKey(item)] = item
	}
	return items, nil
}

func itemsOf(inventory *Inventory, kind string) []*Item {
	switch kind {
	case KindSkill:
		return inventory.Skills
	case KindPrompt:
		return inventory.Prompts
	}
	return inventory.McpServers
}

// resolve is the check every read and write shares: the agent is known, it
// keeps that kind of configuration somewhere, and the owner's home resolves.
func resolve(agentId string, owner string, kind string) (layout, string, error) {
	found, ok := layoutOf(agentId)
	if !ok {
		return layout{}, "", fmt.Errorf("unknown agent: %s", agentId)
	}
	if (kind == KindSkill && found.skills == nil) ||
		(kind == KindMcp && found.mcp == nil) ||
		(kind == KindPrompt && found.prompt == nil) {
		return layout{}, "", fmt.Errorf("%s: %w", agentId, ErrUnsupported)
	}
	home, err := homeOf(owner)
	if err != nil {
		return layout{}, "", err
	}
	return found, home, nil
}

func sortItems(items []*Item) {
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
}
