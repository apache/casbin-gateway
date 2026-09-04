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
	"github.com/apache/casbin-gateway/util"
)

// BrowseLocalPath lists one directory of this host, for the file picker that
// says where an agent no scan can find is installed.
func (c *ApiController) BrowseLocalPath() {
	if c.RequireAdmin() {
		return
	}

	listing, err := util.BrowseDir(c.GetString("path"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(listing)
}

// AddAgentPath records a program someone chose as an installation of an agent.
// The host decides whether it is one, not the request.
func (c *ApiController) AddAgentPath() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId string `json:"agentId"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	installation, err := agent.AddManualInstallation(form.AgentId, form.Path)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(installation)
}

// RemoveAgentPath forgets one. The program stays where it is.
func (c *ApiController) RemoveAgentPath() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId string `json:"agentId"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := agent.RemoveManualInstallation(form.AgentId, form.Path); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(form.AgentId)
}
