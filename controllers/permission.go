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

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/object"
)

// agentPermissionInfo is one agent's permissions with everything the card needs
// to draw them: the switches it has, and the casbin model and policy they
// compile to, which is what the advanced view shows.
type agentPermissionInfo struct {
	Permission *object.AgentPermission `json:"permission"`
	Groups     []object.ToolGroup      `json:"groups"`
	Model      string                  `json:"model"`
	Policy     []string                `json:"policy"`
}

func newAgentPermissionInfo(permission *object.AgentPermission, owner string) *agentPermissionInfo {
	return &agentPermissionInfo{
		Permission: permission,
		Groups:     object.ToolGroups(mcpItemsOf(permission.Name, owner)),
		Model:      object.PermissionModelText,
		Policy:     permission.PolicyText(),
	}
}

// mcpItemsOf is one switch per MCP server the agent has installed, so a server
// can be taken away from it by name rather than all of them at once. An agent
// whose configuration cannot be read keeps the catch-all switch alone.
func mcpItemsOf(agentId string, owner string) []object.ToolItem {
	items := []object.ToolItem{}
	if owner == "" {
		return items
	}

	inventory := agentconfig.Read(agentId, owner)
	if !inventory.McpSupported {
		return items
	}
	for _, server := range inventory.McpServers {
		items = append(items, object.McpServerItem(server.Name))
	}
	// A connection that has been tested knows what it offers, so its tools get a
	// switch each, under the server switch they already had.
	items = append(items, object.ConnectionToolItems(agentId)...)
	return items
}

// GetAgentPermission answers with what one agent is allowed to ask for. An
// agent nobody has configured answers with the unrestricted default rather than
// with nothing, so the card renders the same either way.
func (c *ApiController) GetAgentPermission() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.Input().Get("agentId")
	if agentId == "" {
		c.ResponseError("the agent id is empty")
		return
	}

	permission, err := object.GetAgentPermission(agentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(newAgentPermissionInfo(permission, c.Input().Get("owner")))
}

// GetAgentPermissions lists what every configured agent is held to, for the
// page that shows them side by side.
func (c *ApiController) GetAgentPermissions() {
	if c.RequireAdmin() {
		return
	}

	permissions, err := object.GetAgentPermissions()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	stored := []*object.AgentPermission{}
	for _, permission := range permissions {
		stored = append(stored, permission)
	}
	c.ResponseOk(stored)
}

// UpdateAgentPermission stores the switches of one agent's Permissions card.
// The policy compiled from them decides the next request that agent relays.
func (c *ApiController) UpdateAgentPermission() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId    string                  `json:"agentId"`
		Owner      string                  `json:"owner"`
		Permission *object.AgentPermission `json:"permission"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.IsKnownAgentId(form.AgentId) {
		c.ResponseError("unknown agent: " + form.AgentId)
		return
	}

	if err := object.UpdateAgentPermission(form.AgentId, form.Permission); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(newAgentPermissionInfo(form.Permission, form.Owner))
}
