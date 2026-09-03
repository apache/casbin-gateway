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
	"sort"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/object"
)

// trayAgent is one agent the tray offers a provider menu for. Installations are
// discovered per path while the routing is stored per agent id, so the menu is
// one entry per id however many copies are installed.
type trayAgent struct {
	AgentId  string `json:"agentId"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

// trayProvider is one provider a tray menu entry can be switched to.
type trayProvider struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}

// GetTrayMenu is everything the desktop tray draws: the agents installed here,
// the provider each is bound to, and the providers they can be switched to. It
// is one call because the menu is rebuilt as a whole.
func (c *ApiController) GetTrayMenu() {
	if c.RequireLocalAdmin() {
		return
	}

	installations, err := agent.Scan(false)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	routings, err := object.GetAgents()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	agents := []*trayAgent{}
	seen := map[string]bool{}
	for _, installation := range installations {
		if seen[installation.AgentId] {
			continue
		}
		seen[installation.AgentId] = true

		item := &trayAgent{
			AgentId: installation.AgentId,
			Name:    installation.Name,
		}
		if stored, ok := routings[installation.AgentId]; ok {
			item.Provider = stored.Provider
		}
		agents = append(agents, item)
	}
	sort.Slice(agents, func(i, j int) bool { return agents[i].Name < agents[j].Name })

	stored, err := object.GetProviders("")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	providers := []*trayProvider{}
	for _, provider := range stored {
		name := provider.DisplayName
		if name == "" {
			name = provider.Name
		}
		providers = append(providers, &trayProvider{
			Id:       provider.GetId(),
			Name:     name,
			Disabled: provider.Status == "disabled",
		})
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name < providers[j].Name })

	c.ResponseOk(map[string]interface{}{"agents": agents, "providers": providers})
}

// SetAgentProvider binds one agent to one provider, leaving the rest of its
// routing alone. It is the tray's one click: the fallbacks and the mode chosen
// on the pages survive a switch made from the menu.
func (c *ApiController) SetAgentProvider() {
	if c.RequireLocalAdmin() {
		return
	}

	var form struct {
		AgentId  string `json:"agentId"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.IsKnownAgentId(form.AgentId) {
		c.ResponseError("unknown agent: " + form.AgentId)
		return
	}

	fallbacks := []string{}
	mode := object.ModeGateway
	routing, err := object.GetAgent(form.AgentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if routing != nil {
		fallbacks = routing.Fallbacks
		if routing.Mode != "" {
			mode = routing.Mode
		}
	}

	if err := saveAgentRouting(form.AgentId, form.Provider, fallbacks, mode); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(form.Provider)
}
