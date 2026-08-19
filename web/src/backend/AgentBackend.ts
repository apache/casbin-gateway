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
import type {Agent, AgentRecord, AgentSession} from "@/types";

export interface PatchTarget {
  agentId: string;
  path: string;
  owner: string;
}

export interface AgentConfigStatus {
  takenOver: boolean;
  endpoint?: string;
}

export function getAgents(forceRefresh = false) {
  return request<Agent[]>(`/api/get-agents${forceRefresh ? "?refresh=true" : ""}`);
}

export function patchAgent(target: PatchTarget) {
  return request<{followup?: string}>("/api/patch-agent", "POST", target);
}

export function unpatchAgent(target: PatchTarget) {
  return request<{followup?: string}>("/api/unpatch-agent", "POST", target);
}

export function takeoverAgentConfig(target: PatchTarget, endpoint: string, token: string) {
  return request<AgentConfigStatus>("/api/takeover-agent-config", "POST", {
    ...target,
    endpoint,
    token,
  });
}

export function restoreAgentConfig(target: PatchTarget) {
  return request("/api/restore-agent-config", "POST", target);
}

export function getAgentRecords(agent = "", eventType = "", outcome = "", session = "", limit = 200) {
  return request<AgentRecord[]>(
    `/api/get-agent-records${query({
      agent: agent,
      eventType: eventType,
      outcome: outcome,
      session: session,
      limit: limit,
    })}`,
  );
}

export function getAgentSessions(agent = "") {
  return request<AgentSession[]>(`/api/get-agent-sessions${query({agent: agent})}`);
}
