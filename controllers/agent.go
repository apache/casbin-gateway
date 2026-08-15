// Copyright 2025 The casbin Authors. All Rights Reserved.
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
	"net"
	"strconv"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/agentpatch"
)

type discoveredAgent struct {
	agent.Installation
	agentpatch.Status
}

// GetAgents scans known installation locations and returns the AI agents
// installed in the environment where Casbin Gateway is running.
func (c *ApiController) GetAgents() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(c.GetString("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	result := make([]*discoveredAgent, 0, len(installations))
	for _, installation := range installations {
		result = append(result, &discoveredAgent{
			Installation: installation,
			Status:       agentpatch.StatusOf(targetOf(installation)),
		})
	}
	c.ResponseOk(result)
}

// PatchAgent enables monitoring for one discovered installation.
func (c *ApiController) PatchAgent() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentpatch.Patch(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentpatch.StatusOf(target))
}

// UnpatchAgent disables monitoring and restores any configuration it changed.
func (c *ApiController) UnpatchAgent() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentpatch.Unpatch(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentpatch.StatusOf(target))
}

// GetAgentRecords returns the current process's in-memory agent activity.
func (c *ApiController) GetAgentRecords() {
	if c.RequireAdmin() {
		return
	}

	limit := 200
	if value := c.Input().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		limit = parsed
	}
	c.ResponseOk(agentmonitor.ListRecords(agentmonitor.RecordQuery{
		Agent:     c.Input().Get("agent"),
		EventType: c.Input().Get("eventType"),
		Outcome:   c.Input().Get("outcome"),
		Session:   c.Input().Get("session"),
		Limit:     limit,
	}))
}

// GetAgentSessions groups the current in-memory records by agent session.
func (c *ApiController) GetAgentSessions() {
	if c.RequireAdmin() {
		return
	}
	c.ResponseOk(agentmonitor.ListSessions(agentmonitor.RecordQuery{}))
}

// AddAgentRecord accepts reports from a hook or MCP process launched locally
// by Gateway. Those processes have no browser session, so loopback is the
// authorization boundary.
func (c *ApiController) AddAgentRecord() {
	remoteAddr := c.Ctx.Request.RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		c.ResponseError("agent record ingestion is limited to loopback")
		return
	}

	var record agentmonitor.Record
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &record); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if record.Agent == "" {
		c.ResponseError("agent is required")
		return
	}
	record.ClientIp = ip.String()
	agentmonitor.AddRecord(&record)
	c.ResponseOk()
}

func (c *ApiController) readAgentPatchTarget() (agentpatch.Target, bool) {
	var target agentpatch.Target
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &target); err != nil {
		c.ResponseError(err.Error())
		return target, false
	}
	return target, true
}

func targetOf(installation agent.Installation) agentpatch.Target {
	return agentpatch.Target{
		AgentId: installation.AgentId,
		Path:    installation.Path,
		Owner:   installation.Owner,
	}
}
