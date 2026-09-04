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

// This file answers the second enforcement point. The proxy takes a forbidden
// tool out of the request before a model is offered it; the hook an agent runs
// before executing a tool asks here instead, which is what holds an agent whose
// traffic never comes through Gateway at all.

package object

import "fmt"

// CheckAgentTool decides one tool call. The ids are the agents the caller
// speaks for: one configuration file can serve several front ends, and the
// strictest of their rules is the one that stands.
//
// It answers "allowed" for an agent nobody has restricted, and for a tool that
// falls under no switch, so a hook installed everywhere costs nothing until
// someone turns permissions on.
func CheckAgentTool(agentIds []string, tool string) (bool, string, error) {
	if tool == "" {
		return true, "", nil
	}

	for _, agentId := range agentIds {
		guard, err := LoadAgentGuard(agentId)
		if err != nil {
			// A rule that cannot be read is not one to stop an agent on: the
			// caller logs this and the tool call goes ahead.
			return true, "", err
		}
		if guard == nil || guard.AllowTool(tool) {
			continue
		}
		return false, fmt.Sprintf("the permissions of agent %s do not allow %s (%s)",
			agentId, tool, ToolItemOf(tool)), nil
	}
	return true, "", nil
}
