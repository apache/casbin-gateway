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
	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentegress"
	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agentprocess"
	"github.com/apache/casbin-gateway/object"
)

const egressFindingLimit = 20

type agentEgress struct {
	agentegress.Report
	// Snapshots are packages the agent left on disk, whether or not the watch
	// was running when they went out.
	Snapshots []agentegress.Snapshot `json:"snapshots"`
	Findings  []*object.AgentRecord  `json:"findings"`
}

// GetAgentEgress returns where one agent's processes have been sending data.
func (c *ApiController) GetAgentEgress() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.GetString("agentId")
	if agentId == "" {
		c.ResponseError("the agent id is empty")
		return
	}
	result := agentEgress{
		Report:    agentegress.ReportOf(agentId),
		Snapshots: []agentegress.Snapshot{},
		Findings:  object.GetAgentRecords(object.AgentRecordFilter{Agent: agentId, EventType: agentegress.EventType, Limit: egressFindingLimit}),
	}
	if home, err := agenthome.Resolve(c.GetString("owner")); err == nil {
		if snapshots := agentegress.SnapshotsOf(agentId, home); snapshots != nil {
			result.Snapshots = snapshots
		}
	}
	c.ResponseOk(result)
}

// AgentPids maps the live processes of every installation found by the last
// scan to its agent id. It is what the egress watch polls, so it never scans.
func AgentPids() map[int]string {
	pids := map[int]string{}
	for _, installation := range agent.Cached() {
		for _, pid := range agentprocess.StatusOf(processTarget(installation)).Pids {
			pids[pid] = installation.AgentId
		}
	}
	return pids
}
