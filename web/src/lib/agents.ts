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

import * as React from "react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import type {BadgeVariant} from "@/components/ui/badge";
import {providerProtocol, servesResponsesApi} from "@/lib/providers";
import type {
  Agent,
  AgentCatalogEntry,
  AgentRuntime,
  AgentSession,
  AgentUsage,
  AgentUsageStat,
  Provider,
} from "@/types";

/** How long a started app is given before its process is looked for again. */
const runtimeSettleMs = 2000;

/** Radix rejects an empty item value, so "the agent's own model" needs one. */
export const builtinProvider = "-";

/** Routing through the local proxy, which is what makes a switch hot. */
export const gatewayMode = "gateway";
export const directMode = "direct";

/**
 * What the agent talks to with nothing bound: the model its own configuration
 * names, or the service it signs in to. Binding nothing puts it back on this.
 */
export function agentBuiltin(agent: Agent) {
  return agent.providerConfig?.builtin || i18next.t("agent:Built-in model");
}

/** The monitor keys the two Codex front ends under one agent id. */
export function monitorAgentId(agentId: string) {
  return agentId === "codex_vscode" || agentId === "codex-vscode" ? "codex-cli" : agentId;
}

/**
 * agentKey identifies one installation. The same agent can be installed twice
 * on a host - once per user account, or once per install method - so neither
 * the agent id nor the path is unique on its own.
 */
export function agentKey(agent: Pick<Agent, "owner" | "path">) {
  return `${agent.owner}:${agent.path}`;
}

/**
 * The base URL an agent is pointed at to reach its own provider. One URL serves
 * every wire format: an OpenAI client appends /chat/completions to it, Codex
 * appends /responses, and an Anthropic one appends /v1/messages.
 */
export function agentProxyBaseUrl(agentId: string) {
  return `${Setting.ServerUrl || window.location.origin}/v1/agents/${encodeURIComponent(agentId)}`;
}

/**
 * A config file that carries the same setting for good, for the agents that
 * have one. Empty when only the environment is known to work.
 */
export function agentSetupNoteKey(agentId: string) {
  switch (agentId) {
  case "claude-code":
    return "agent:Claude Code config hint";
  case "claude-desktop":
    return "agent:Claude Desktop sandbox hint";
  case "codex-cli":
    return "agent:Codex CLI config hint";
  case "opencode":
  case "opencode-desktop":
    return "agent:opencode config hint";
  case "gemini-cli":
    return "agent:Gemini CLI config hint";
  default:
    return "";
  }
}

/**
 * Mirrors the check in agentprovider's Codex writer: Codex speaks only the
 * OpenAI Responses API, so it reaches a chat completions provider through
 * Gateway, which translates, or not at all.
 */
export function agentNeedsResponsesApi(agentId: string) {
  return agentId.startsWith("codex");
}

/**
 * Why an agent can only be routed through the gateway, as the key of the line
 * that says so. Empty for an agent that can also be bound to a provider
 * directly.
 */
export function agentGatewayOnlyKey(agent: Agent, bound: Provider | undefined) {
  if (agentProtocol(agent) === "gemini") {
    return "agent:Gemini gateway only hint";
  }
  if (agentNeedsResponsesApi(agent.agentId) && !servesResponsesApi(bound)) {
    return "agent:Gateway only hint";
  }
  return "";
}

/**
 * The wire format an agent's client speaks, as the server reports it. Empty for
 * an agent Gateway does not know, which is then not filtered on.
 */
export function agentProtocol(agent: Agent) {
  return agent.providerConfig?.protocol ?? "";
}

/**
 * Whether an agent can be pointed at a provider serving this wire format.
 * Through the gateway any provider will do: the proxy translates between the
 * APIs. A direct binding writes the provider's own URL into the agent config,
 * so there the two have to speak the same one.
 */
export function agentSpeaks(agent: Agent, protocol: string) {
  if ((agent.mode || gatewayMode) !== directMode) {
    return true;
  }
  const spoken = agentProtocol(agent);
  return spoken === "" || spoken === protocol;
}

/**
 * Whether this agent, routed the way it is now, can be pointed at this provider
 * at all. Through the gateway anything goes, since the proxy translates; a
 * direct binding hands the agent the provider's own URL, and there both the
 * wire format and, for Codex, the Responses API have to line up.
 */
export function agentCanUse(agent: Agent, provider: Provider) {
  if ((agent.mode || gatewayMode) !== directMode) {
    return true;
  }
  if (agentNeedsResponsesApi(agent.agentId) && !servesResponsesApi(provider)) {
    return false;
  }
  return agentSpeaks(agent, providerProtocol(provider.type));
}

/**
 * The detail page route for one installation. The path is only spelled out when
 * the same agent is installed more than once, which is what makes the id alone
 * ambiguous; otherwise the URL stays short.
 */
export function agentDetailPath(agent: Agent, agents: Agent[] = []) {
  const url = `/agents/${encodeURIComponent(agent.agentId)}`;
  const duplicated = agents.filter(candidate => candidate.agentId === agent.agentId).length > 1;
  return duplicated ? `${url}?path=${encodeURIComponent(agent.path)}` : url;
}

/**
 * useAgents owns the local-installation scan and the monitoring toggle, which
 * the dashboard, the list page and the detail page all need.
 */
export function useAgents(enabled = true) {
  const [agents, setAgents] = React.useState<Agent[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [busyKey, setBusyKey] = React.useState("");
  // The run state is keyed by installation and loaded on its own, so it can be
  // refreshed after a start or a stop without scanning the disk again.
  const [runtime, setRuntime] = React.useState<Record<string, AgentRuntime>>({});
  const [runBusyKey, setRunBusyKey] = React.useState("");
  // A scan inside a container reads the container's filesystem, not the host's,
  // which is why it can come back empty on a machine full of agents.
  const [inContainer, setInContainer] = React.useState(false);
  // A scan is only "done" once the first response has landed, which is what
  // tells an empty list apart from a list that has not arrived yet.
  const [scanned, setScanned] = React.useState(false);

  const scan = React.useCallback(
    (forceRefresh = false) => {
      if (!enabled) {
        return;
      }

      setLoading(true);
      setError("");
      AgentBackend.getAgents(forceRefresh)
        .then(res => {
          if (res.status === "ok") {
            setAgents(res.data ?? []);
            setInContainer(res.data2 === true);
          } else {
            setError(res.msg || i18next.t("agent:Failed to scan agents"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => {
          setLoading(false);
          setScanned(true);
        });
    },
    [enabled],
  );

  const loadRuntime = React.useCallback((forceRefresh = false) => {
    if (!enabled) {
      return;
    }

    AgentBackend.getAgentProcesses(forceRefresh)
      .then(res => {
        if (res.status === "ok") {
          const next: Record<string, AgentRuntime> = {};
          (res.data ?? []).forEach(item => {
            next[agentKey(item)] = item;
          });
          setRuntime(next);
        }
      })
      .catch(() => undefined);
  }, [enabled]);

  React.useEffect(() => {
    scan();
    loadRuntime();
  }, [scan, loadRuntime]);

  /**
   * toggleRunning starts the agent or ends every process of it. A desktop app
   * takes a moment to show up in the process table, so the run state is read
   * again shortly after the call rather than only once.
   */
  const toggleRunning = React.useCallback(
    (agent: Agent, running: boolean) => {
      const target = {agentId: agent.agentId, path: agent.path, owner: agent.owner};

      setRunBusyKey(agentKey(agent));
      (running ? AgentBackend.stopAgent(target) : AgentBackend.startAgent(target))
        .then(res => {
          if (res.status === "ok") {
            Setting.showMessage(
              "success",
              `${i18next.t(running ? "agent:Agent stopped" : "agent:Agent started")}: ${agent.name}`,
            );
            setTimeout(() => loadRuntime(true), runtimeSettleMs);
          } else {
            Setting.showMessage(
              "error",
              res.msg ||
                i18next.t(running ? "agent:Failed to stop the agent" : "agent:Failed to start the agent"),
            );
          }
          loadRuntime(true);
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setRunBusyKey(""));
    },
    [loadRuntime],
  );

  const togglePatch = React.useCallback(
    (agent: Agent) => {
      const target = {agentId: agent.agentId, path: agent.path, owner: agent.owner};
      const patched = agent.patched;

      setBusyKey(agentKey(agent));
      (patched ? AgentBackend.unpatchAgent(target) : AgentBackend.patchAgent(target))
        .then(res => {
          if (res.status === "ok") {
            const done = patched
              ? i18next.t("agent:Monitoring disabled")
              : i18next.t("agent:Monitoring enabled");
            const followup = res.data?.followup || agent.followup;
            Setting.showMessage(
              "success",
              followup ? `${done}: ${agent.name}. ${followup}` : `${done}: ${agent.name}`,
            );
            scan();
          } else {
            Setting.showMessage("error", res.msg || i18next.t("agent:Failed to update agent patch"));
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyKey(""));
    },
    [scan],
  );

  /**
   * setRouting saves where an agent's requests go. Only the changed part is
   * passed in; the rest of the routing is carried over from the agent so a
   * fallback list is not dropped by a provider change.
   */
  const setRouting = React.useCallback(
    (agent: Agent, routing: Partial<AgentBackend.AgentRouting>) => {
      const next: AgentBackend.AgentRouting = {
        provider: routing.provider ?? agent.provider,
        fallbacks: routing.fallbacks ?? agent.fallbacks ?? [],
        mode: routing.mode ?? agent.mode ?? gatewayMode,
      };

      setBusyKey(agentKey(agent));
      AgentBackend.updateAgentRouting(agent.agentId, next)
        .then(res => {
          if (res.status === "ok") {
            Setting.showMessage(
              "success",
              next.provider === ""
                ? `${i18next.t("agent:Built-in model restored")}: ${agentBuiltin(agent)}`
                : `${i18next.t("agent:Provider saved")}: ${next.provider}`,
            );
            scan();
          } else {
            Setting.showMessage(
              "error",
              res.msg || i18next.t("agent:Failed to update agent provider"),
            );
            // The routing itself may already be stored: only writing the agent's
            // own configuration file failed, and the page has to show that.
            scan();
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyKey(""));
    },
    [scan],
  );

  /**
   * activateProvider is the one click the home page is built around: it stores
   * the routing and, for an agent whose configuration file Gateway knows how to
   * write, writes it, so the agent is on the new provider with nothing left to
   * press.
   */
  const activateProvider = React.useCallback(
    (agent: Agent, providerId: string) => {
      const target = {agentId: agent.agentId, path: agent.path, owner: agent.owner};
      const routing: AgentBackend.AgentRouting = {
        provider: providerId,
        fallbacks: [],
        mode: agent.mode || gatewayMode,
      };
      const writes = providerId !== "" && (agent.providerConfig?.supported ?? false);

      setBusyKey(agentKey(agent));
      AgentBackend.updateAgentRouting(agent.agentId, routing)
        .then(res => {
          if (res.status !== "ok") {
            throw new Error(res.msg || i18next.t("agent:Failed to update agent provider"));
          }
          return writes ? AgentBackend.applyAgentProvider(target) : res;
        })
        .then(res => {
          if (res.status !== "ok") {
            throw new Error(res.msg || i18next.t("agent:Failed to write the agent configuration"));
          }
          Setting.showMessage(
            "success",
            providerId === ""
              ? `${i18next.t("agent:Built-in model restored")}: ${agentBuiltin(agent)}`
              : `${i18next.t("agent:Provider enabled")}: ${providerId}`,
          );
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        // The routing may be stored even when the file write failed, so the
        // scan runs either way and the page shows what actually happened.
        .then(() => {
          scan();
          setBusyKey("");
        });
    },
    [scan],
  );

  /**
   * writeProvider writes the bound provider into the agent's own config file, or
   * puts back what the file held before Gateway first wrote it.
   */
  const writeProvider = React.useCallback(
    (agent: Agent, restore = false) => {
      const target = {agentId: agent.agentId, path: agent.path, owner: agent.owner};

      setBusyKey(agentKey(agent));
      (restore
        ? AgentBackend.restoreAgentProvider(target)
        : AgentBackend.applyAgentProvider(target))
        .then(res => {
          if (res.status === "ok") {
            Setting.showMessage(
              "success",
              restore
                ? i18next.t("agent:Configuration restored")
                : i18next.t("agent:Configuration written"),
            );
            scan();
          } else {
            Setting.showMessage(
              "error",
              res.msg || i18next.t("agent:Failed to write the agent configuration"),
            );
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyKey(""));
    },
    [scan],
  );

  return {
    agents,
    loading,
    error,
    busyKey,
    scanned,
    inContainer,
    runtime,
    runBusyKey,
    scan,
    loadRuntime,
    toggleRunning,
    togglePatch,
    setRouting,
    activateProvider,
    writeProvider,
  };
}

/**
 * useAgentCatalog lists the agents Gateway knows how to work with that this
 * machine has none of. The catalogue is built into the binary, so it is read
 * once and only the scan it is compared against changes.
 */
export function useAgentCatalog(agents: Agent[], enabled = true) {
  const [catalog, setCatalog] = React.useState<AgentCatalogEntry[]>([]);

  React.useEffect(() => {
    if (!enabled) {
      return;
    }

    AgentBackend.getAgentCatalog()
      .then(res => setCatalog(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined);
  }, [enabled]);

  return React.useMemo(
    () => catalog.filter(item => !agents.some(agent => agent.agentId === item.agentId)),
    [catalog, agents],
  );
}

/** The run state of one installation, before the first listing has landed. */
export function runtimeOf(runtime: Record<string, AgentRuntime>, agent: Agent) {
  return runtime[agentKey(agent)];
}

/** What one agent has been up to, derived from its monitoring sessions. */
export interface AgentActivity {
  sessionCount: number;
  recordCount: number;
  lastTime: string;
}

/**
 * useAgentSessions loads the live session summaries. Each summary already
 * carries its agent, record count and last activity, so the per-agent totals
 * the dashboard shows are folded out of this one request rather than out of the
 * far larger raw record list.
 */
export function useAgentSessions(enabled = true, agent = "", refreshMs = 0) {
  const [sessions, setSessions] = React.useState<AgentSession[]>([]);
  const [error, setError] = React.useState("");

  const load = React.useCallback(() => {
    if (!enabled) {
      return;
    }

    AgentBackend.getAgentSessions(agent)
      .then(res => {
        if (res.status === "ok") {
          setSessions(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("agent:Failed to get agent sessions"));
        }
      })
      .catch(err => setError(err.message || String(err)));
  }, [enabled, agent]);

  React.useEffect(() => {
    load();
    if (!enabled || refreshMs <= 0) {
      return undefined;
    }
    const interval = setInterval(load, refreshMs);
    return () => clearInterval(interval);
  }, [enabled, load, refreshMs]);

  const activity = React.useMemo(() => {
    const totals: Record<string, AgentActivity> = {};
    sessions.forEach(session => {
      const current = totals[session.agent] ?? {sessionCount: 0, recordCount: 0, lastTime: ""};
      totals[session.agent] = {
        sessionCount: current.sessionCount + 1,
        recordCount: current.recordCount + session.recordCount,
        lastTime:
          current.lastTime === "" || session.lastTime > current.lastTime
            ? session.lastTime
            : current.lastTime,
      };
    });
    return totals;
  }, [sessions]);

  const recordCount = React.useMemo(
    () => sessions.reduce((total, session) => total + session.recordCount, 0),
    [sessions],
  );

  return {sessions, activity, recordCount, error, reload: load};
}

/** The activity of the installation, looked up under the id the monitor uses. */
export function activityOf(activity: Record<string, AgentActivity>, agent: Agent) {
  return activity[monitorAgentId(agent.agentId)];
}

/**
 * Transcripts are written per tool rather than per installation: every Codex
 * front end shares one directory, so all of them read the same usage. That is
 * also the limit of what the files can tell apart.
 */
export function usageAgentId(agentId: string) {
  return agentId.startsWith("codex") ? "codex-cli" : agentId;
}

/** What one installation spent, out of the totals read off the transcripts. */
export function usageOf(usage: AgentUsage | undefined, agent: Agent): AgentUsageStat | undefined {
  return usage?.agents.find(item => item.name === usageAgentId(agent.agentId));
}

/** The badge colour every record table paints an outcome with. */
export function getOutcomeVariant(outcome: string | undefined): BadgeVariant {
  return (
    {
      attempted: "info",
      denied: "warning",
      failure: "danger",
      success: "success",
    } as const
  )[outcome as "attempted" | "denied" | "failure" | "success"] ?? "muted";
}
