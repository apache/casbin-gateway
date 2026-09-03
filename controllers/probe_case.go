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

	"github.com/apache/casbin-gateway/object"
)

// GetProbeCases is the whole suite a probe runs, disabled cases included: what
// is not being asked is as much a part of a published method as what is.
func (c *ApiController) GetProbeCases() {
	if c.RequireAdmin() {
		return
	}

	cases, err := object.GetProbeCases()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(cases)
}

// AddProbeCase stores a case someone wrote.
func (c *ApiController) AddProbeCase() {
	if c.RequireAdmin() {
		return
	}

	probeCase := object.ProbeCase{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &probeCase); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.AddProbeCase(&probeCase); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(&probeCase)
}

// UpdateProbeCase writes an edited case back. The name is taken from the query
// rather than the body, so renaming a case is not something an edit can do by
// accident to the reports that name it.
func (c *ApiController) UpdateProbeCase() {
	if c.RequireAdmin() {
		return
	}

	name := c.Input().Get("name")
	if name == "" {
		c.ResponseError("no test case was named")
		return
	}

	probeCase := object.ProbeCase{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &probeCase); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.UpdateProbeCase(name, &probeCase); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(&probeCase)
}

// DeleteProbeCase drops a case. A built-in one comes back with a reset.
func (c *ApiController) DeleteProbeCase() {
	if c.RequireAdmin() {
		return
	}

	name := c.Input().Get("name")
	if name == "" {
		c.ResponseError("no test case was named")
		return
	}
	if err := object.DeleteProbeCase(name); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}

// ResetProbeCases puts the shipped suite back, leaving cases someone wrote
// where they are.
func (c *ApiController) ResetProbeCases() {
	if c.RequireAdmin() {
		return
	}

	if err := object.ResetProbeCases(); err != nil {
		c.ResponseError(err.Error())
		return
	}

	cases, err := object.GetProbeCases()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(cases)
}
