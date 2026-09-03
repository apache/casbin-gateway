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
  Agent,
  AgentCatalogEntry,
  AgentInstallJob,
  AgentInstance,
  AgentProviderConfig,
  AgentProviderFile,
  AgentRecord,
  AgentRuntime,
  AgentSession,
  AgentTranscript,
  AgentUsage,
} from "@/types";

export interface PatchTarget {
  agentId: string;
  path: string;
  owner: string;
}

/** `data2` tells whether the scan ran inside a container. */
export function getAgents(forceRefresh = false) {
  return request<Agent[], boolean>(`/api/get-agents${forceRefresh ? "?refresh=true" : ""}`);
}

/** Every agent Gateway knows, which is more than the ones installed here. */
export function getAgentCatalog() {
  return request<AgentCatalogEntry[]>("/api/get-agent-catalog");
}

/** Installs an agent this machine lacks; the job runs on past this response. */
export function installAgent(agentId: string) {
  return request<AgentInstallJob>("/api/install-agent", "POST", {agentId: agentId});
}

/** Updates one installation through the package manager that installed it. */
export function upgradeAgent(target: PatchTarget) {
  return request<AgentInstallJob>("/api/upgrade-agent", "POST", target);
}

/** Every install and upgrade this Gateway process has run, newest first. */
export function getAgentInstallJobs() {
  return request<AgentInstallJob[]>("/api/get-agent-install-jobs");
}

export function patchAgent(target: PatchTarget) {
  return request<{followup?: string}>("/api/patch-agent", "POST", target);
}

export function unpatchAgent(target: PatchTarget) {
  return request<{followup?: string}>("/api/unpatch-agent", "POST", target);
}

export function getAgentProcesses(forceRefresh = false) {
  return request<AgentRuntime[]>(`/api/get-agent-processes${forceRefresh ? "?refresh=true" : ""}`);
}

export function startAgent(target: PatchTarget) {
  return request<AgentRuntime>("/api/start-agent", "POST", target);
}

export function stopAgent(target: PatchTarget) {
  return request<AgentRuntime>("/api/stop-agent", "POST", target);
}

/** The extra copies of one agent, each with its own account and run state. */
export function getAgentInstances(agent = "", forceRefresh = false) {
  return request<AgentInstance[]>(
    `/api/get-agent-instances${query({agent: agent, refresh: forceRefresh ? "true" : ""})}`,
  );
}

/**
 * Registers one more copy of an installed agent and lays out its state. The
 * server names it, so adding one is a single click; the name is edited after.
 */
export function addAgentInstance(target: PatchTarget) {
  return request<AgentInstance>("/api/add-agent-instance", "POST", target);
}

/** Renames one instance in the lists; its state directory stays where it is. */
export function renameAgentInstance(name: string, displayName: string) {
  return request<AgentInstance>("/api/update-agent-instance", "POST", {
    name: name,
    displayName: displayName,
  });
}

/** Forgets one instance; its state directory is left on disk. */
export function deleteAgentInstance(name: string) {
  return request<string>("/api/delete-agent-instance", "POST", {name: name});
}

export function startAgentInstance(name: string) {
  return request<AgentInstance>("/api/start-agent-instance", "POST", {name: name});
}

export function stopAgentInstance(name: string) {
  return request<AgentInstance>("/api/stop-agent-instance", "POST", {name: name});
}

export interface AgentRouting {
  provider: string;
  fallbacks: string[];
  mode: string;
}

export function updateAgentRouting(agentId: string, routing: AgentRouting) {
  return request("/api/update-agent-routing", "POST", {agentId: agentId, ...routing});
}

/** What a switch would write, rendered without touching a file. */
export function planAgentProvider(target: PatchTarget) {
  return request<AgentProviderFile[]>("/api/plan-agent-provider", "POST", target);
}

export function applyAgentProvider(target: PatchTarget) {
  return request<AgentProviderConfig>("/api/apply-agent-provider", "POST", target);
}

export function restoreAgentProvider(target: PatchTarget) {
  return request<AgentProviderConfig>("/api/restore-agent-provider", "POST", target);
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

/**
 * What the agents spent, read from the transcripts they write themselves. LLM
 * Records only holds what went through Gateway, so this is the account of an
 * agent talking to its vendor directly - and the one with something to show
 * before anything is routed at all.
 *
 * `days` bounds the window to that many calendar days ending today.
 */
export function getAgentUsage(agent = "", days = 0) {
  return request<AgentUsage>(`/api/get-agent-usage${query({agent: agent, days: days})}`);
}

/** The conversation inside one session, read from the agent's own transcript. */
export function getAgentSession(agent: string, session: string) {
  return request<AgentTranscript>(`/api/get-agent-session${query({agent: agent, session: session})}`);
}
