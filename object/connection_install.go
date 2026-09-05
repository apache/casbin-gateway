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

package object

import (
	"fmt"
	"sort"

	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/connector"
	"github.com/apache/casbin-gateway/mcpproxy"
)

// InstallConnection writes one connection's MCP server into each agent in
// agentIds and takes it out of every agent it was in before. One target's
// failure is reported against that target and the rest still run, which is how
// the rest of agentconfig behaves.
//
// What is written is not the server itself but Gateway standing in front of it,
// so the third-party credential never reaches the agent's configuration file
// and every call through it can be asked about.
func InstallConnection(connection *Connection, agentIds []string) ([]*agentconfig.PlanItem, error) {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return nil, fmt.Errorf("no connector named %q", connection.Name)
	}
	// Rendering here does not produce what is written; it is what refuses to
	// install a connection whose credentials are incomplete.
	if _, err := found.Render(connection.Credentials); err != nil {
		return nil, err
	}

	planned := []*agentconfig.PlanItem{}
	for _, agentId := range agentIds {
		request, err := proxyRequest(connection, found, agentId)
		if err != nil {
			planned = append(planned, &agentconfig.PlanItem{
				AgentId: agentId, Name: found.Server.Name, Action: "skip", Reason: err.Error(),
			})
			continue
		}
		// One agent at a time: each is given a credential of its own, so the
		// entry written is not the same entry twice.
		items, err := agentconfig.AddMcp(request)
		if err != nil {
			planned = append(planned, &agentconfig.PlanItem{
				AgentId: agentId, Name: found.Server.Name, Action: "skip", Reason: err.Error(),
			})
			continue
		}
		planned = append(planned, items...)
	}

	planned = append(planned, removeFrom(connection, found.Server.Name, dropped(connection.Agents, agentIds))...)

	connection.Agents = installed(planned, agentIds)
	// Remembering which Gateway the entries name is what lets a later start
	// notice that they are stale, see EnsureConnectionsCurrent.
	if executable, err := agentpatch.GatewayExecutable(); err == nil {
		connection.Executable = executable
	}
	if gatewayUrl, err := agentpatch.GatewayBaseUrl(); err == nil {
		connection.Endpoint = gatewayUrl
	}
	if err := SaveConnection(connection); err != nil {
		return nil, err
	}
	return planned, nil
}

// proxyRequest is the entry one agent gets: this executable, told which
// connection to reach and how to reach Gateway.
//
// The token in it is Gateway's own, not the service's: it works from loopback
// only, names one agent, and is revoked from here. The credential that would be
// worth stealing stays in Gateway's database.
func proxyRequest(connection *Connection, found connector.Connector, agentId string) (agentconfig.McpRequest, error) {
	executable, err := agentpatch.GatewayExecutable()
	if err != nil {
		return agentconfig.McpRequest{}, err
	}
	gatewayUrl, err := agentpatch.GatewayBaseUrl()
	if err != nil {
		return agentconfig.McpRequest{}, err
	}
	token, err := agentpatch.IssueIngestToken(agentpatch.Target{AgentId: agentId, Owner: connection.Owner})
	if err != nil {
		return agentconfig.McpRequest{}, err
	}

	return agentconfig.McpRequest{
		Owner:     connection.Owner,
		To:        []string{agentId},
		Name:      found.Server.Name,
		Transport: agentconfig.TransportStdio,
		Command:   executable,
		// "--flag=value" rather than two arguments: a machine-wide installation
		// has no owner, and an empty value written as its own argument makes the
		// flag swallow the next one instead.
		Args: []string{
			mcpproxy.Subcommand,
			"--connection=" + connection.Name,
			"--agent=" + agentId,
			"--owner=" + connection.Owner,
			"--gateway-url=" + gatewayUrl,
			"--token=" + token,
		},
		Overwrite: true,
	}, nil
}

// UninstallConnection takes the server out of every agent that has it and
// forgets the connection along with its credentials.
func UninstallConnection(owner string, name string) ([]*agentconfig.PlanItem, error) {
	connection, err := GetConnection(owner, name)
	if err != nil || connection == nil {
		return nil, err
	}
	found, ok := connector.Get(name)
	if !ok {
		return nil, fmt.Errorf("no connector named %q", name)
	}

	planned := removeFrom(connection, found.Server.Name, connection.Agents)
	if err := DeleteConnection(owner, name); err != nil {
		return nil, err
	}
	return planned, nil
}

// ResolveConnection is what the proxy asks for when an agent starts it: the
// server behind one connection, with its credentials filled in. It is answered
// over loopback to a process this Gateway's own configuration launched, and the
// result is never written to disk.
func ResolveConnection(owner string, name string) (*connector.Rendered, error) {
	connection, err := GetConnection(owner, name)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, fmt.Errorf("%q is not connected", name)
	}
	found, ok := connector.Get(name)
	if !ok {
		return nil, fmt.Errorf("no connector named %q", name)
	}

	// A session is about to start on this credential, so this is the moment a
	// grant near its end is worth renewing. Doing it here rather than on a timer
	// means a connection nobody uses costs nothing.
	if err := refreshIfNeeded(connection, found); err != nil {
		return nil, err
	}

	rendered, err := found.Render(connection.Credentials)
	if err != nil {
		return nil, err
	}
	return &rendered, nil
}

// removeFrom deletes the server entry from each agent, reporting what happened
// per agent. An agent that no longer has the entry is not an error: the
// operator may have deleted it by hand, and the connection still has to let go.
func removeFrom(connection *Connection, serverName string, agentIds []string) []*agentconfig.PlanItem {
	planned := []*agentconfig.PlanItem{}
	for _, agentId := range agentIds {
		item := &agentconfig.PlanItem{AgentId: agentId, Name: serverName, Action: "remove"}
		if err := agentconfig.Delete(agentId, connection.Owner, agentconfig.KindMcp, serverName); err != nil {
			item.Action, item.Reason = "skip", err.Error()
		}
		planned = append(planned, item)
	}
	return planned
}

// dropped is every agent in before that is not in after.
func dropped(before []string, after []string) []string {
	keep := map[string]bool{}
	for _, agentId := range after {
		keep[agentId] = true
	}

	found := []string{}
	for _, agentId := range before {
		if !keep[agentId] {
			found = append(found, agentId)
		}
	}
	return found
}

// installed is the agents the write actually reached, so a connection never
// claims an agent whose configuration could not be written.
func installed(planned []*agentconfig.PlanItem, agentIds []string) []string {
	failed := map[string]bool{}
	for _, item := range planned {
		if item.Action == "skip" {
			failed[item.AgentId] = true
		}
	}

	found := []string{}
	for _, agentId := range agentIds {
		if !failed[agentId] {
			found = append(found, agentId)
		}
	}
	sort.Strings(found)
	return found
}

// EnsureConnectionsCurrent writes again every connection whose entries name a
// Gateway this one is not. An installation's hooks get the same treatment from
// agentpatch, and for the same reason: a port that changed or a program that
// moved leaves the entry naming something that is no longer there, and nothing
// else on the machine would ever notice.
//
// It compares two strings per connection and touches no file when they match,
// so it is cheap enough to run wherever the agents are listed.
func EnsureConnectionsCurrent() error {
	executable, err := agentpatch.GatewayExecutable()
	if err != nil {
		return err
	}
	gatewayUrl, err := agentpatch.GatewayBaseUrl()
	if err != nil {
		return err
	}

	connections, err := getAllConnections()
	if err != nil {
		return err
	}

	var failed error
	for _, connection := range connections {
		if len(connection.Agents) == 0 {
			continue
		}
		if connection.Endpoint == gatewayUrl && connection.Executable == executable {
			continue
		}
		if _, err := InstallConnection(connection, connection.Agents); err != nil {
			failed = fmt.Errorf("connection %s: %w", connection.GetId(), err)
		}
	}
	return failed
}
