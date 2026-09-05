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
	"github.com/apache/casbin-gateway/connector"
)

// InstallConnection writes one connection's MCP server into each agent in
// agentIds and takes it out of every agent it was in before. One target's
// failure is reported against that target and the rest still run, which is how
// the rest of agentconfig behaves.
func InstallConnection(connection *Connection, agentIds []string) ([]*agentconfig.PlanItem, error) {
	found, ok := connector.Get(connection.Name)
	if !ok {
		return nil, fmt.Errorf("no connector named %q", connection.Name)
	}
	rendered, err := found.Render(connection.Credentials)
	if err != nil {
		return nil, err
	}

	planned := []*agentconfig.PlanItem{}
	if len(agentIds) > 0 {
		planned, err = agentconfig.AddMcp(agentconfig.McpRequest{
			Owner:     connection.Owner,
			To:        agentIds,
			Name:      rendered.Name,
			Transport: rendered.Transport,
			Command:   rendered.Command,
			Args:      rendered.Args,
			Env:       rendered.Env,
			Url:       rendered.Url,
			Headers:   rendered.Headers,
			Overwrite: true,
		})
		if err != nil {
			return nil, err
		}
	}

	planned = append(planned, removeFrom(connection, rendered.Name, dropped(connection.Agents, agentIds))...)

	connection.Agents = installed(planned, agentIds)
	if err := SaveConnection(connection); err != nil {
		return nil, err
	}
	return planned, nil
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
