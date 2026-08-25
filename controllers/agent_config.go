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

package controllers

import (
	"encoding/json"
	"sort"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentconfig"
)

// agentConfigView is one installation's configuration, named the way the agent
// list names it.
type agentConfigView struct {
	*agentconfig.Inventory

	Name string `json:"name"`
	Path string `json:"path,omitempty"`
	// Installed is false for configuration found without a matching installation.
	Installed bool `json:"installed"`
	// SharedWith names the other agents reading the same files. Cursor and its
	// CLI share one ~/.cursor, and the Codex front ends one CODEX_HOME, so those
	// appear once with the others listed here rather than as separate copies of
	// the same configuration.
	SharedWith []string `json:"sharedWith,omitempty"`
}

// GetAgentConfigs lists the skills, MCP servers and instructions of every agent
// on this host whose configuration layout Gateway knows.
func (c *ApiController) GetAgentConfigs() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(c.GetString("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	collected := &agentConfigs{seen: map[string]*agentConfigView{}}
	for _, installation := range installations {
		collected.add(installation.AgentId, installation.Owner, installation.Name, installation.Path, true)
	}

	// An agent can be configured on the account Gateway runs as without its
	// installation being recognized — installed through a package manager
	// Gateway does not scan, or already uninstalled. Its skills are on disk
	// either way, so the page would be wrong to leave them out.
	for _, agentId := range agentconfig.KnownAgents() {
		name := agent.DisplayNameOf(agentId)
		if name == "" || !agentconfig.Configured(agentId, "") {
			continue
		}
		collected.add(agentId, "", name, "", false)
	}

	sort.Slice(collected.views, func(i, j int) bool { return collected.views[i].Name < collected.views[j].Name })
	c.ResponseOk(collected.views)
}

// agentConfigs collects one view per configuration location. Several agent ids
// can read the same files, and the page lists what is on disk rather than who
// reads it, so the second id to arrive is recorded as a reader of the first.
type agentConfigs struct {
	views []*agentConfigView
	seen  map[string]*agentConfigView
}

func (collected *agentConfigs) add(agentId string, owner string, name string, path string, installed bool) {
	if !agentconfig.Supports(agentId) {
		return
	}

	inventory := agentconfig.Read(agentId, owner)
	key := inventory.SkillsDir + "\x00" + inventory.McpFile + "\x00" + inventory.PromptFile
	if first, ok := collected.seen[key]; ok {
		if first.AgentId != agentId && !contains(first.SharedWith, name) {
			first.SharedWith = append(first.SharedWith, name)
		}
		return
	}

	view := &agentConfigView{
		Inventory: inventory,
		Name:      name,
		Path:      path,
		Installed: installed,
	}
	collected.seen[key] = view
	collected.views = append(collected.views, view)
}

func contains(names []string, name string) bool {
	for _, each := range names {
		if each == name {
			return true
		}
	}
	return false
}

// SaveAgentConfigPrompt replaces the instructions one agent reads before every
// session, writing the file in that agent's own place and under its own name.
func (c *ApiController) SaveAgentConfigPrompt() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId string `json:"agentId"`
		Owner   string `json:"owner"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	item, err := agentconfig.SavePrompt(form.AgentId, form.Owner, form.Content)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(item)
}

// GetAgentConfigItem returns one skill's manifest, one MCP server's entry, or
// one agent's instructions.
func (c *ApiController) GetAgentConfigItem() {
	if c.RequireAdmin() {
		return
	}

	detail, err := agentconfig.ReadDetail(
		c.GetString("agentId"), c.GetString("owner"), c.GetString("kind"), c.GetString("name"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(detail)
}

// DeleteAgentConfigItem removes one skill, MCP server or instruction file from
// the agent's own configuration.
func (c *ApiController) DeleteAgentConfigItem() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId string `json:"agentId"`
		Owner   string `json:"owner"`
		Kind    string `json:"kind"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if form.Name == agentconfig.ManagedEntryName {
		c.ResponseError("this entry belongs to Gateway's agent monitoring; turn monitoring off to remove it")
		return
	}

	if err := agentconfig.Delete(form.AgentId, form.Owner, form.Kind, form.Name); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(form.Name)
}

// GetAgentConfigTrash lists what deleting has removed and can still be put
// back.
func (c *ApiController) GetAgentConfigTrash() {
	if c.RequireAdmin() {
		return
	}

	entries, err := agentconfig.ListTrash(c.GetString("owner"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(entries)
}

// RestoreAgentConfigItem puts one deleted item back where it came from. It
// refuses when something is there again unless replace says otherwise, and that
// replaced item goes to the recycle bin in its turn.
func (c *ApiController) RestoreAgentConfigItem() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Owner   string `json:"owner"`
		Id      string `json:"id"`
		Replace bool   `json:"replace"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	entry, err := agentconfig.RestoreTrash(form.Owner, form.Id, form.Replace)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(entry)
}

// PurgeAgentConfigTrash deletes one trashed item for good, or all of them when
// no id is given.
func (c *ApiController) PurgeAgentConfigTrash() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Owner string `json:"owner"`
		Id    string `json:"id"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := agentconfig.PurgeTrash(form.Owner, form.Id); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(form.Id)
}

// UpdateAgentConfigSkill replaces one skill with the current content of the
// source it was copied from, after putting the version it replaces in the
// recycle bin.
func (c *ApiController) UpdateAgentConfigSkill() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId string `json:"agentId"`
		Owner   string `json:"owner"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	item, err := agentconfig.UpdateSkill(form.AgentId, form.Owner, form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(item)
}

// AddAgentConfigMcp writes one new MCP server into the agents it names, so a
// server can be set up from Gateway instead of by hand in each agent's file.
func (c *ApiController) AddAgentConfigMcp() {
	if c.RequireAdmin() {
		return
	}

	var request agentconfig.McpRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	result, err := agentconfig.AddMcp(request)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(result)
}

// PlanAgentConfigCopy reports what CopyAgentConfig would do, so the operator
// sees which targets already have an item before anything is written.
func (c *ApiController) PlanAgentConfigCopy() {
	if c.RequireAdmin() {
		return
	}

	request, ok := c.readCopyRequest()
	if !ok {
		return
	}
	planned, err := agentconfig.Plan(request)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(planned)
}

// CopyAgentConfig migrates the selected skills, MCP servers or instructions
// into the target agents' own configuration files.
func (c *ApiController) CopyAgentConfig() {
	if c.RequireAdmin() {
		return
	}

	request, ok := c.readCopyRequest()
	if !ok {
		return
	}
	result, err := agentconfig.Copy(request)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(result)
}

func (c *ApiController) readCopyRequest() (agentconfig.CopyRequest, bool) {
	var request agentconfig.CopyRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return request, false
	}
	return request, true
}
