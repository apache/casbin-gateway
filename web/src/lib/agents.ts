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
  AgentInstallJob,
  AgentInstance,
  AgentRuntime,
  AgentSession,
  AgentUpdate,
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

/** The monitor keys front ends that share a configuration under one agent id. */
export function monitorAgentId(agentId: string) {
  switch (agentId) {
  case "codex_vscode":
  case "codex-vscode":
    return "codex-cli";
  case "opencode-desktop":
    return "opencode";
  case "cursor-agent":
    return "cursor";
  default:
    return agentId;
  }
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
  case "qwen-code":
    return "agent:Qwen Code config hint";
  case "iflow-cli":
    return "agent:iFlow CLI config hint";
  case "cline":
    return "agent:Cline config hint";
  case "continue":
    return "agent:Continue config hint";
  case "aider":
    return "agent:Aider config hint";
  case "goose":
    return "agent:goose config hint";
  case "droid":
    return "agent:Droid config hint";
  case "zed":
    return "agent:Zed config hint";
  case "pi":
    return "agent:Pi config hint";
  case "kimi-code":
    return "agent:Kimi Code config hint";
  case "codebuddy":
    return "agent:CodeBuddy config hint";
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
 * The agent id the configuration on disk routes to, empty when it names
 * anything other than a gateway agent endpoint. Two front ends can share one
 * configuration file - the opencode CLI and its desktop app do - so the id
 * written there is not always the agent being looked at.
 */
export function routedAgentId(agent: Agent) {
  const marker = "/v1/agents/";
  const current = agent.providerConfig?.current ?? "";
  const index = current.indexOf(marker);
  if (index === -1) {
    return "";
  }
  return decodeURIComponent(current.slice(index + marker.length).split("/")[0]);
}

/**
 * Whether what this agent calls is its own gateway endpoint. Only the requests
 * that arrive there are held to its permissions, so an agent this is false for
 * is not restricted by them at all, whatever its switches say.
 */
export function agentRoutedHere(agent: Agent) {
  return routedAgentId(agent) === agent.agentId;
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

/** How often a running install is asked about while the manager works. */
const installPollMs = 1500;

/**
 * useAgentInstall drives the installs and upgrades Gateway runs with the host's
 * own package managers. A package manager takes minutes, so the request that
 * starts one returns at once and the job is polled until it ends: the button
 * that started it keeps showing what is happening, and a reload does too.
 *
 * `onFinished` is called after every job that ends, which is when a rescan will
 * see the agent that was just installed.
 */
export function useAgentInstall(enabled = true, onFinished?: () => void) {
  const [jobs, setJobs] = React.useState<Record<string, AgentInstallJob>>({});
  const [busyId, setBusyId] = React.useState("");
  // The jobs this page is waiting on, so the one that finishes is reported
  // once - and one left running by an earlier page is picked up on load.
  const watched = React.useRef<Record<string, boolean>>({});
  const finished = React.useRef(onFinished);
  finished.current = onFinished;

  const load = React.useCallback(() => {
    if (!enabled) {
      return;
    }

    AgentBackend.getAgentInstallJobs()
      .then(res => {
        const list = res.status === "ok" ? (res.data ?? []) : [];
        setJobs(Object.fromEntries(list.map(job => [job.agentId, job])));
      })
      .catch(() => undefined);
  }, [enabled]);

  React.useEffect(() => load(), [load]);

  const running = Object.values(jobs).some(job => job.running);

  React.useEffect(() => {
    if (!running) {
      return;
    }

    const timer = window.setInterval(load, installPollMs);
    return () => window.clearInterval(timer);
  }, [running, load]);

  React.useEffect(() => {
    Object.values(jobs).forEach(job => {
      if (job.running) {
        watched.current[job.agentId] = true;
        return;
      }
      if (!watched.current[job.agentId]) {
        return;
      }
      delete watched.current[job.agentId];
      Setting.showMessage(
        job.ok ? "success" : "error",
        job.ok
          ? `${i18next.t(installOutcomeKey(job.action))}: ${job.name}${job.version ? ` ${job.version}` : ""}`
          : `${i18next.t("agent:Failed to install the agent")}: ${job.error || job.name}`,
      );
      finished.current?.();
    });
  }, [jobs]);

  const start = React.useCallback(
    (agentId: string, call: () => Promise<{status: string; msg?: string; data?: AgentInstallJob}>) => {
      setBusyId(agentId);
      call()
        .then(res => {
          if (res.status === "ok" && res.data) {
            watched.current[agentId] = true;
            setJobs(current => ({...current, [agentId]: res.data as AgentInstallJob}));
          } else {
            Setting.showMessage("error", res.msg || i18next.t("agent:Failed to install the agent"));
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyId(""));
    },
    [],
  );

  const install = React.useCallback(
    (agentId: string, version = "") =>
      start(agentId, () => AgentBackend.installAgent(agentId, version)),
    [start],
  );

  const upgrade = React.useCallback(
    (agent: Agent) => start(agent.agentId, () => AgentBackend.upgradeAgent(installTarget(agent))),
    [start],
  );

  const setVersion = React.useCallback(
    (agent: Agent, version: string) =>
      start(agent.agentId, () => AgentBackend.setAgentVersion(installTarget(agent), version)),
    [start],
  );

  const uninstall = React.useCallback(
    (agent: Agent) => start(agent.agentId, () => AgentBackend.uninstallAgent(installTarget(agent))),
    [start],
  );

  return {jobs, busyId, install, upgrade, setVersion, uninstall, reload: load};
}

function installTarget(agent: Agent) {
  return {agentId: agent.agentId, path: agent.path, owner: agent.owner};
}

/** What a finished job is reported as, which is not the same for all four. */
function installOutcomeKey(action: string) {
  switch (action) {
  case "upgrade":
    return "agent:Agent upgraded";
  case "downgrade":
    return "agent:Agent moved to an older version";
  case "uninstall":
    return "agent:Agent uninstalled";
  default:
    return "agent:Agent installed";
  }
}

/** How often the registries are asked again while a page stays open. */
const updatePollMs = 10 * 60 * 1000;

/**
 * useAgentUpdates asks which installations have a newer release waiting. The
 * answer comes from a registry rather than from disk, so it is loaded on its
 * own: a page shows its agents at once and marks the outdated ones when the
 * lookups land.
 *
 * `scope` of "all" also names the release a first install would land on for the
 * agents this machine has none of, which is what a page listing every agent
 * needs and what one listing only installations does not.
 */
export function useAgentUpdates(enabled = true, scope: "installed" | "all" = "installed") {
  const [list, setList] = React.useState<AgentUpdate[]>([]);
  const [checking, setChecking] = React.useState(false);

  const load = React.useCallback(
    (forceRefresh = false) => {
      if (!enabled) {
        return;
      }

      setChecking(true);
      AgentBackend.getAgentUpdates(forceRefresh, scope)
        .then(res => setList(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => undefined)
        .then(() => setChecking(false));
    },
    [enabled, scope],
  );

  React.useEffect(() => load(), [load]);

  React.useEffect(() => {
    if (!enabled) {
      return;
    }

    const timer = window.setInterval(() => load(true), updatePollMs);
    return () => window.clearInterval(timer);
  }, [enabled, load]);

  // An installation is keyed by where it is, an agent that has none by its id:
  // the rows for the missing ones carry no path to tell them apart by.
  const updates = React.useMemo(
    () => Object.fromEntries(list.filter(item => item.path !== "").map(item => [agentKey(item), item])),
    [list],
  );
  const missing = React.useMemo(
    () => Object.fromEntries(list.filter(item => item.path === "").map(item => [item.agentId, item])),
    [list],
  );
  const outdated = list.filter(item => item.available).length;

  return {updates, missing, outdated, checking, reload: load};
}

/** The release check for one installation, before the lookups have landed. */
export function updateOf(updates: Record<string, AgentUpdate>, agent: Pick<Agent, "owner" | "path">) {
  return updates[agentKey(agent)];
}

/** The run state of one installation, before the first listing has landed. */
export function runtimeOf(runtime: Record<string, AgentRuntime>, agent: Agent) {
  return runtime[agentKey(agent)];
}

/**
 * useAgentSessions loads the live session summaries. Each summary already
 * carries its agent, record count and last activity, so the totals a page shows
 * are folded out of this one request rather than out of the far larger raw
 * record list.
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

  const recordCount = React.useMemo(
    () => sessions.reduce((total, session) => total + session.recordCount, 0),
    [sessions],
  );

  return {sessions, recordCount, error, reload: load};
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

/**
 * useAgentInstances owns the extra copies of one agent: the ones stored, what
 * each is signed in to, and whether each is running. Each has a state directory
 * of its own, so they start, stop and sign in independently of one another.
 */
export function useAgentInstances(agentId = "", enabled = true) {
  const [instances, setInstances] = React.useState<AgentInstance[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [busyName, setBusyName] = React.useState("");

  const load = React.useCallback(
    (forceRefresh = false) => {
      if (!enabled) {
        return;
      }

      setLoading(true);
      AgentBackend.getAgentInstances(agentId, forceRefresh)
        .then(res => {
          if (res.status === "ok") {
            setInstances(res.data ?? []);
          } else {
            Setting.showMessage("error", res.msg);
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setLoading(false));
    },
    [agentId, enabled],
  );

  React.useEffect(() => {
    load();
  }, [load]);

  /**
   * Adds one copy. The server names and lays it out, so there is nothing to ask
   * first; the name is edited afterwards, once an account has signed in to it.
   * The agent id stands in for the row that does not exist yet while it runs.
   */
  const add = React.useCallback(
    (agent: Agent) => {
      setBusyName(agent.agentId);
      AgentBackend.addAgentInstance({agentId: agent.agentId, path: agent.path, owner: agent.owner})
        .then(res => {
          if (res.status === "ok") {
            Setting.showMessage(
              "success",
              `${i18next.t("agent:Instance added")}: ${res.data?.instance ?? ""}`,
            );
            load(true);
          } else {
            Setting.showMessage("error", res.msg || i18next.t("agent:Failed to add the instance"));
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyName(""));
    },
    [load],
  );

  const rename = React.useCallback(
    (instance: AgentInstance, displayName: string) => {
      AgentBackend.renameAgentInstance(instance.name, displayName)
        .then(res => {
          if (res.status !== "ok") {
            Setting.showMessage("error", res.msg);
          }
          load();
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)));
    },
    [load],
  );

  const remove = React.useCallback(
    (instance: AgentInstance) => {
      setBusyName(instance.name);
      AgentBackend.deleteAgentInstance(instance.name)
        .then(res => {
          if (res.status === "ok") {
            Setting.showMessage("success", `${i18next.t("agent:Instance removed")}: ${instance.instance}`);
            load(true);
          } else {
            Setting.showMessage("error", res.msg);
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyName(""));
    },
    [load],
  );

  /** Starts or stops one copy, leaving whatever else is running alone. */
  const toggleRunning = React.useCallback(
    (instance: AgentInstance) => {
      const running = instance.running;

      setBusyName(instance.name);
      (running
        ? AgentBackend.stopAgentInstance(instance.name)
        : AgentBackend.startAgentInstance(instance.name))
        .then(res => {
          if (res.status === "ok") {
            Setting.showMessage(
              "success",
              `${i18next.t(running ? "agent:Agent stopped" : "agent:Agent started")}: ${
                instance.displayName || instance.instance
              }`,
            );
            // A desktop app takes a moment to show up in the process table, so
            // the run state is read again shortly after the call.
            setTimeout(() => load(true), runtimeSettleMs);
          } else {
            Setting.showMessage(
              "error",
              res.msg ||
                i18next.t(running ? "agent:Failed to stop the agent" : "agent:Failed to start the agent"),
            );
          }
          load(true);
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyName(""));
    },
    [load],
  );

  /**
   * Takes the URL scheme of the agent for one copy, or gives it back. A copy
   * started without an account takes it on its own; this is for one that is
   * already running, or that is signing in again.
   */
  const toggleCapture = React.useCallback(
    (instance: AgentInstance) => {
      const capture = !instance.capturing;

      setBusyName(instance.name);
      AgentBackend.captureAgentInstanceLink(instance.name, capture)
        .then(res => {
          if (res.status === "ok") {
            if (capture) {
              Setting.showMessage("success", i18next.t("agent:Waiting for the sign-in link"));
            }
          } else {
            Setting.showMessage("error", res.msg);
          }
          load();
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyName(""));
    },
    [load],
  );

  return {
    instances,
    loading,
    busyName,
    reload: load,
    add,
    rename,
    remove,
    toggleRunning,
    toggleCapture,
  };
}

/** What one hook call returns, as the cards and the tables take it. */
export type AgentInstanceControls = ReturnType<typeof useAgentInstances>;

/** The instances belonging to one installation, out of a listing of them all. */
export function instancesOf(instances: AgentInstance[], agent: Agent) {
  return instances.filter(instance => instance.agentId === agent.agentId);
}
