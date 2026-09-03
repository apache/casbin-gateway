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
	"github.com/apache/casbin-gateway/object"
)

// agentPermissionInfo is one agent's permissions with everything the card needs
// to draw them: the groups it has a switch for, and the casbin model and policy
// they compile to, which is what the advanced view shows.
type agentPermissionInfo struct {
	Permission *object.AgentPermission `json:"permission"`
	Groups     []object.ToolGroup      `json:"groups"`
	Model      string                  `json:"model"`
	Policy     []string                `json:"policy"`
}

func newAgentPermissionInfo(permission *object.AgentPermission) *agentPermissionInfo {
	return &agentPermissionInfo{
		Permission: permission,
		Groups:     object.ToolGroups(),
		Model:      object.PermissionModelText,
		Policy:     permission.PolicyText(),
	}
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

	c.ResponseOk(newAgentPermissionInfo(permission))
}

// UpdateAgentPermission stores the switches of one agent's Permissions card.
// The policy compiled from them decides the next request that agent relays.
func (c *ApiController) UpdateAgentPermission() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId    string                  `json:"agentId"`
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

	c.ResponseOk(newAgentPermissionInfo(form.Permission))
}
