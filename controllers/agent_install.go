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
		// Version pins the install to one published release, empty for the one
		// the manager calls current.
		Version string `json:"version"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.IsKnownAgentId(form.AgentId) {
		c.ResponseError("unknown agent: " + form.AgentId)
		return
	}

	job, err := agentinstall.Start(agentinstall.InstallVersionPlan(form.AgentId, form.Version))
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

// SetAgentVersion moves one installation onto a chosen release, up or down,
// through the package manager that installed it. The version is the only part
// of the command that comes from the request, and it is refused unless it reads
// as a version number.
func (c *ApiController) SetAgentVersion() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	installation, ok := c.readAgentInstallation()
	if !ok {
		return
	}

	plan := agentinstall.VersionPlan(
		installation.AgentId, installation.InstallMethod, form.Version, installation.Version)
	job, err := agentinstall.Start(plan)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(job)
}

// UninstallAgent removes one discovered installation with the manager that put
// it there. Only the program goes: the agent's own state directory keeps its
// sign-in and its history, so reinstalling it finds them again.
func (c *ApiController) UninstallAgent() {
	if c.RequireAdmin() {
		return
	}

	installation, ok := c.readAgentInstallation()
	if !ok {
		return
	}

	job, err := agentinstall.Start(
		agentinstall.UninstallPlan(installation.AgentId, installation.InstallMethod))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(job)
}

// GetAgentVersions lists what the package manager publishes for one agent: the
// release it calls current, and the older ones a downgrade can go back to.
func (c *ApiController) GetAgentVersions() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.GetString("agentId")
	if !agent.IsKnownAgentId(agentId) {
		c.ResponseError("unknown agent: " + agentId)
		return
	}

	c.ResponseOk(agentinstall.VersionsOf(agentId, c.GetString("installMethod"), c.GetString("refresh") == "true"))
}

// GetAgentUpdates reports which installations have a newer release waiting.
// The lookups are cached, so the pages can ask on every load; refresh asks the
// registries again. scope=all adds a row for every agent this host has none of,
// naming the release an install would land on.
func (c *ApiController) GetAgentUpdates() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(false)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	force := c.GetString("refresh") == "true"
	updates := agentinstall.UpdatesFor(installations, force)
	if c.GetString("scope") == "all" {
		updates = append(updates, agentinstall.UpdatesForMissing(installations, force)...)
	}
	c.ResponseOk(updates)
}
