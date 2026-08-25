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
import type {
  AgentConfigDetail,
  AgentConfigInventory,
  AgentConfigItem,
  AgentConfigKind,
  AgentConfigPlanItem,
  AgentConfigTrashEntry,
  McpTransport,
} from "@/types";

export interface CopyRequest {
  owner: string;
  from: string;
  to: string[];
  kind: AgentConfigKind;
  names: string[];
  overwrite: boolean;
}

/** One new MCP server, for every agent in `to`. */
export interface McpRequest {
  owner: string;
  to: string[];
  name: string;
  transport: McpTransport;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  overwrite: boolean;
}

export function addAgentConfigMcp(body: McpRequest) {
  return request<AgentConfigPlanItem[]>("/api/add-agent-config-mcp", "POST", body);
}

/** Replaces the instructions one agent reads before every session. */
export function saveAgentConfigPrompt(agentId: string, owner: string, content: string) {
  return request<AgentConfigItem>("/api/save-agent-config-prompt", "POST", {
    agentId: agentId,
    owner: owner,
    content: content,
  });
}

export function getAgentConfigs(forceRefresh = false) {
  return request<AgentConfigInventory[]>(`/api/get-agent-configs${forceRefresh ? "?refresh=true" : ""}`);
}

export function getAgentConfigItem(agentId: string, owner: string, kind: AgentConfigKind, name: string) {
  return request<AgentConfigDetail>(
    `/api/get-agent-config-item${query({agentId: agentId, owner: owner, kind: kind, name: name})}`,
  );
}

export function deleteAgentConfigItem(agentId: string, owner: string, kind: AgentConfigKind, name: string) {
  return request("/api/delete-agent-config-item", "POST", {
    agentId: agentId,
    owner: owner,
    kind: kind,
    name: name,
  });
}

/** What deleting removed and can still be put back. */
export function getAgentConfigTrash(owner: string) {
  return request<AgentConfigTrashEntry[]>(`/api/get-agent-config-trash${query({owner: owner})}`);
}

/** Puts one deleted item back. `replace` recycles whatever took its place first. */
export function restoreAgentConfigItem(owner: string, id: string, replace = false) {
  return request<AgentConfigTrashEntry>("/api/restore-agent-config-item", "POST", {
    owner: owner,
    id: id,
    replace: replace,
  });
}

/** Replaces one skill with the current content of the source it was copied from. */
export function updateAgentConfigSkill(agentId: string, owner: string, name: string) {
  return request<AgentConfigItem>("/api/update-agent-config-skill", "POST", {
    agentId: agentId,
    owner: owner,
    name: name,
  });
}

/** Deletes one trashed item for good, or all of them when no id is given. */
export function purgeAgentConfigTrash(owner: string, id = "") {
  return request("/api/purge-agent-config-trash", "POST", {owner: owner, id: id});
}

/** What a copy would do, asked for before anything is written. */
export function planAgentConfigCopy(body: CopyRequest) {
  return request<AgentConfigPlanItem[]>("/api/plan-agent-config-copy", "POST", body);
}

export function copyAgentConfig(body: CopyRequest) {
  return request<AgentConfigPlanItem[]>("/api/copy-agent-config", "POST", body);
}
