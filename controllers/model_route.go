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

// RoutePreviewStep is one attempt of a plan, as the page shows it: which
// provider would be asked, for which model, and which rule chose it.
type RoutePreviewStep struct {
	Provider    string `json:"provider"`
	DisplayName string `json:"displayName"`
	Model       string `json:"model"`
	Route       string `json:"route"`
	Suspended   bool   `json:"suspended"`
}

// GetModelRoutes is every routing rule, disabled ones included, in the order
// they are read in.
func (c *ApiController) GetModelRoutes() {
	if c.RequireAdmin() {
		return
	}

	routes, err := object.GetModelRoutes()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(routes)
}

func (c *ApiController) AddModelRoute() {
	if c.RequireAdmin() {
		return
	}

	route := object.ModelRoute{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &route); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.AddModelRoute(&route); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(&route)
}

// UpdateModelRoute writes an edited rule back. The name comes from the query
// rather than the body, so an edit cannot rename the row it is editing.
func (c *ApiController) UpdateModelRoute() {
	if c.RequireAdmin() {
		return
	}

	name := c.Input().Get("name")
	if name == "" {
		c.ResponseError("no routing rule was named")
		return
	}

	route := object.ModelRoute{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &route); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.UpdateModelRoute(name, &route); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(&route)
}

func (c *ApiController) DeleteModelRoute() {
	if c.RequireAdmin() {
		return
	}

	name := c.Input().Get("name")
	if name == "" {
		c.ResponseError("no routing rule was named")
		return
	}
	if err := object.DeleteModelRoute(name); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}

// PreviewModelRoute answers what a request naming this model would actually be
// sent, without sending it. A ladder is only worth the rules it is written from
// if someone can see where the rules land, and the second and later steps are
// exactly the part nobody sees until an upstream is already failing.
func (c *ApiController) PreviewModelRoute() {
	if c.RequireAdmin() {
		return
	}

	model := c.Input().Get("model")
	if model == "" {
		c.ResponseError("no model was named")
		return
	}

	attempts, err := c.planPreview(c.Input().Get("agent"), model)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	steps := []RoutePreviewStep{}
	for _, attempt := range attempts {
		steps = append(steps, RoutePreviewStep{
			Provider:    attempt.Provider.GetId(),
			DisplayName: attempt.Provider.DisplayName,
			Model:       attempt.Model,
			Route:       attempt.Route,
			Suspended:   object.IsProviderSuspended(attempt.Provider.GetId()),
		})
	}
	c.ResponseOk(steps)
}

// planPreview is the plan the proxy would build, for an agent or for a bare
// model name.
func (c *ApiController) planPreview(agentId string, model string) ([]object.RouteAttempt, error) {
	if agentId == "" {
		return object.PlanModelRoute(model)
	}

	providers, err := object.GetProvidersByAgent(agentId)
	if err != nil {
		return nil, err
	}
	return object.PlanAgentRoute(agentId, model, providers)
}
