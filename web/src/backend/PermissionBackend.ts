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

import {query, request} from "@/backend/request";
import type {AgentPermission, AgentPermissionInfo} from "@/types";

/** What one agent may do. An agent nobody has configured answers with the
 *  unrestricted default rather than with nothing. The owner is the host user the
 *  installation belongs to, which is where its MCP servers are read from. */
export function getAgentPermission(agentId: string, owner = "") {
  return request<AgentPermissionInfo>(
    `/api/get-agent-permission${query({agentId: agentId, owner: owner})}`,
  );
}

/** Every agent somebody has configured, for the list on the Permissions page. */
export function getAgentPermissions() {
  return request<AgentPermission[]>("/api/get-agent-permissions");
}

export function updateAgentPermission(agentId: string, permission: AgentPermission, owner = "") {
  return request<AgentPermissionInfo>("/api/update-agent-permission", "POST", {
    agentId: agentId,
    owner: owner,
    permission: permission,
  });
}
