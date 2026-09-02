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
	"github.com/apache/casbin-gateway/agentinstall"
)

// InstallAgent installs an agent this machine does not have, with a package
// manager it does. The command comes from the fingerprint of the requested
// agent id and nothing else, so a request names an agent, never a command.
func (c *ApiController) InstallAgent() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId string `json:"agentId"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.IsKnownAgentId(form.AgentId) {
		c.ResponseError("unknown agent: " + form.AgentId)
		return
	}

	job, err := agentinstall.Start(agentinstall.InstallPlan(form.AgentId))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(job)
}

// UpgradeAgent updates one discovered installation through the package manager
// that installed it. The manager runs as the account Gateway runs as, so an
// installation found in another user's home is updated only where that account
// shares the same global prefix.
func (c *ApiController) UpgradeAgent() {
	if c.RequireAdmin() {
		return
	}

	installation, ok := c.readAgentInstallation()
	if !ok {
		return
	}

	job, err := agentinstall.Start(agentinstall.UpgradePlan(installation.AgentId, installation.InstallMethod))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(job)
}

// GetAgentInstallJobs reports the installs and upgrades of this process, the
// ones still running included. A package manager takes minutes, so the page
// polls this instead of waiting on the request that started the job.
func (c *ApiController) GetAgentInstallJobs() {
	if c.RequireAdmin() {
		return
	}

	c.ResponseOk(agentinstall.Jobs())
}
