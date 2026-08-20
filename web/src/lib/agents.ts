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
import type {Agent, AgentSession} from "@/types";

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

/** The detail page route for one installation, disambiguated by its path. */
export function agentDetailPath(agent: Agent) {
  return `/agents/${encodeURIComponent(agent.agentId)}?path=${encodeURIComponent(agent.path)}`;
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

  React.useEffect(() => {
    scan();
  }, [scan]);

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

  const switchProvider = React.useCallback(
    (agent: Agent, channelId: string) => {
      const target = {agentId: agent.agentId, path: agent.path, owner: agent.owner};

      setBusyKey(agentKey(agent));
      AgentBackend.switchAgentProvider(target, channelId)
        .then(res => {
          if (res.status === "ok") {
            const done = res.data?.changed
              ? i18next.t("agent:Provider switched")
              : i18next.t("agent:Provider already selected");
            const notes = [
              res.data?.backedUp ? i18next.t("agent:Original config backed up") : "",
              res.data?.configPath,
            ]
              .filter(Boolean)
              .join(". ");
            Setting.showMessage("success", notes ? `${done}: ${agent.name}. ${notes}` : `${done}: ${agent.name}`);
            scan();
          } else {
            Setting.showMessage("error", res.msg || i18next.t("agent:Failed to switch provider"));
          }
        })
        .catch(err => Setting.showMessage("error", err.message || String(err)))
        .then(() => setBusyKey(""));
    },
    [scan],
  );

  return {agents, loading, error, busyKey, scanned, scan, togglePatch, switchProvider};
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

/** The badge colour every record table paints an outcome with. */
export function getOutcomeVariant(outcome: string | undefined) {
  return (
    {
      attempted: "processing",
      denied: "warning",
      failure: "error",
      success: "success",
    } as const
  )[outcome as "attempted" | "denied" | "failure" | "success"];
}
