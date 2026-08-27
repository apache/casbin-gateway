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
	"github.com/apache/casbin-gateway/agentprocess"
)

// agentRuntime is one installation's run state, keyed the way the list keys an
// installation: the same agent can be installed once per account.
type agentRuntime struct {
	AgentId string `json:"agentId"`
	Path    string `json:"path"`
	Owner   string `json:"owner"`
	// Desktop tells the UI what a start would open: the app itself, or a console
	// window for a CLI.
	Desktop bool `json:"desktop"`
	agentprocess.Status
}

// GetAgentProcesses reports which discovered installations are running now. It
// is separate from the agent listing because reading the process table is worth
// repeating on its own, without scanning the disk again.
func (c *ApiController) GetAgentProcesses() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(false)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	// A page asking again right after a start or a stop wants what the host says
	// now, not what the last caller was told.
	if c.GetString("refresh") == "true" {
		agentprocess.Refresh()
	}

	result := make([]*agentRuntime, 0, len(installations))
	for _, installation := range installations {
		result = append(result, runtimeOf(installation))
	}
	c.ResponseOk(result)
}

// StartAgent runs one installation: a desktop app opens on its own, a CLI opens
// in a console window on the host's desktop.
func (c *ApiController) StartAgent() {
	if c.RequireAdmin() {
		return
	}

	installation, ok := c.readAgentInstallation()
	if !ok {
		return
	}
	if err := agentprocess.Start(processTarget(installation)); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(runtimeOf(installation))
}

// StopAgent ends every process of one installation.
func (c *ApiController) StopAgent() {
	if c.RequireAdmin() {
		return
	}

	installation, ok := c.readAgentInstallation()
	if !ok {
		return
	}
	if err := agentprocess.Stop(processTarget(installation)); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(runtimeOf(installation))
}

func runtimeOf(installation agent.Installation) *agentRuntime {
	target := processTarget(installation)
	return &agentRuntime{
		AgentId: installation.AgentId,
		Path:    installation.Path,
		Owner:   installation.Owner,
		Desktop: target.Desktop,
		Status:  agentprocess.StatusOf(target),
	}
}

func processTarget(installation agent.Installation) agentprocess.Target {
	launch := agent.LaunchOf(installation)
	return agentprocess.Target{
		AgentId:    installation.AgentId,
		Path:       installation.Path,
		Owner:      installation.Owner,
		Executable: launch.Executable,
		Args:       launch.Args,
		Desktop:    launch.Desktop,
	}
}
