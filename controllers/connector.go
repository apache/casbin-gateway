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
	"slices"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/agentpatch"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/connector"
	"github.com/apache/casbin-gateway/object"
)

// ConnectorEntry is one catalog card: the connector itself, plus whether this
// machine has connected it and to which agents.
type ConnectorEntry struct {
	connector.Connector
	Connected bool     `json:"connected"`
	Agents    []string `json:"agents"`
}

// ConnectorTarget is one agent on this machine a connection can be installed
// into: an installation Gateway found whose MCP configuration it knows how to
// write.
type ConnectorTarget struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`
	Owner   string `json:"owner"`
}

// ConnectorCatalog is everything the connectors page draws itself from.
type ConnectorCatalog struct {
	Connectors []*ConnectorEntry  `json:"connectors"`
	Categories []string           `json:"categories"`
	Targets    []*ConnectorTarget `json:"targets"`
}

// GetConnectors lists the whole catalog with each entry's connection state, the
// sections to group it by, and the agents a connection can be installed into.
func (c *ApiController) GetConnectors() {
	if c.RequireAdmin() {
		return
	}

	connections, err := object.GetConnections(c.GetString("owner"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	state := map[string]*object.Connection{}
	for _, connection := range connections {
		state[connection.Name] = connection
	}

	entries := []*ConnectorEntry{}
	for _, found := range connector.List() {
		entry := &ConnectorEntry{Connector: found, Agents: []string{}}
		if connection, ok := state[found.Id]; ok {
			entry.Connected = true
			entry.Agents = connection.Agents
		}
		entries = append(entries, entry)
	}

	targets, err := connectorTargets()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(&ConnectorCatalog{Connectors: entries, Categories: connector.Categories(), Targets: targets})
}

// connectorTargets is every installation found on this machine whose MCP
// configuration Gateway knows how to write, one row per agent rather than per
// installation: a connection is written into an agent's own configuration, and
// two installations of one agent share it.
func connectorTargets() ([]*ConnectorTarget, error) {
	installations, err := agent.Scan(false)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	targets := []*ConnectorTarget{}
	for _, installation := range installations {
		if seen[installation.AgentId] || !agentconfig.SupportsMcp(installation.AgentId) {
			continue
		}
		seen[installation.AgentId] = true
		targets = append(targets, &ConnectorTarget{
			AgentId: installation.AgentId,
			Name:    installation.Name,
			Owner:   installation.Owner,
		})
	}

	sort.Slice(targets, func(i int, j int) bool { return targets[i].Name < targets[j].Name })
	return targets, nil
}

// GetConnection returns one connection with its secrets masked, which is what
// fills the edit dialog.
func (c *ApiController) GetConnection() {
	if c.RequireAdmin() {
		return
	}

	connection, err := object.GetConnection(c.GetString("owner"), c.GetString("name"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if connection == nil {
		c.ResponseOk(nil)
		return
	}
	c.ResponseOk(connection.Masked())
}

// Connect stores one connector's credentials and writes its MCP server into
// the agents that were ticked. Sending it again is how a connection is edited:
// the agents it names become the agents it is installed in.
func (c *ApiController) Connect() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Owner       string            `json:"owner"`
		Name        string            `json:"name"`
		Credentials map[string]string `json:"credentials"`
		Agents      []string          `json:"agents"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if _, ok := connector.Get(form.Name); !ok {
		c.ResponseError("no connector named " + form.Name)
		return
	}

	connection, err := object.GetConnection(form.Owner, form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if connection == nil {
		connection = &object.Connection{Owner: form.Owner, Name: form.Name}
	}
	connection.Credentials = form.Credentials

	// Storing before installing keeps a credential that was accepted from being
	// lost because one agent's configuration could not be written.
	if err := object.SaveConnection(connection); err != nil {
		c.ResponseError(err.Error())
		return
	}

	planned, err := object.InstallConnection(connection, form.Agents)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(planned)
}

// Disconnect removes the server from every agent that has it and forgets the
// credentials, so revoking one connection is a single action rather than an
// edit of every agent it reached.
func (c *ApiController) Disconnect() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	planned, err := object.UninstallConnection(form.Owner, form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(planned)
}

// ResolveConnection hands the proxy the server behind one connection, with its
// credentials filled in. This is the one place a stored credential leaves
// Gateway, so it is fenced the way tool checks are: a direct loopback request,
// presenting the installation credential issued when the connection was
// written, and only for the agent that credential names.
func (c *ApiController) ResolveConnection() {
	if _, ok := c.directLoopbackClient(); !ok {
		c.ResponseError("resolving a connection is limited to direct loopback requests")
		return
	}
	tokenAgentId, ok := agentpatch.ValidateIngestToken(c.Ctx.Input.Header(agentmonitor.IngestTokenHeader))
	if !ok {
		c.ResponseError("resolving a connection requires a valid installation token")
		return
	}

	var form struct {
		Owner string `json:"owner"`
		Name  string `json:"name"`
		Agent string `json:"agent"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if tokenAgentId != "" && !strings.EqualFold(form.Agent, tokenAgentId) {
		c.ResponseError("agent does not match the installation this token was issued for")
		return
	}

	// A connection this agent was never given is not one it may resolve, or one
	// agent's token would open every connection on the machine.
	connection, err := object.GetConnection(form.Owner, form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if connection == nil || !slices.Contains(connection.Agents, form.Agent) {
		c.ResponseError("this connection is not installed in " + form.Agent)
		return
	}

	rendered, err := object.ResolveConnection(form.Owner, form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(rendered)
}
