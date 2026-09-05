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
import {Link, useParams, useSearchParams} from "react-router-dom";
import {ArrowLeft, Bot, ChevronRight, CircleX, Copy, Eye, EyeOff, RefreshCw} from "lucide-react";
import copy from "copy-to-clipboard";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {AgentAccounts} from "@/components/AgentAccounts";
import {AgentInstances} from "@/components/AgentInstances";
import {AgentUsageTrend, hasUsageTrend} from "@/components/AgentUsageTrend";
import {accountLabel} from "@/components/AgentGridCard";
import {RunBadge, RunButton} from "@/components/AgentRunControl";
import {
  AgentUpdateBadge,
  AgentVersionDialog,
  ToolUninstallConfirmDialog,
} from "@/components/AgentVersionDialog";
import {InstallOutput, ToolUpgradeConfirmDialog} from "@/components/ToolUpgradeConfirmDialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {PermissionCard} from "@/components/PermissionCard";
import {ProviderCard} from "@/components/ProviderCard";
import {ResultScreen, UnauthorizedResult} from "@/components/shared/misc";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {AiDots} from "@/components/shared/loading";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {
  agentKey,
  getOutcomeVariant,
  monitorAgentId,
  runtimeOf,
  updateOf,
  useAgentInstall,
  useAgents,
  useAgentSessions,
  useAgentUpdates,
  usageAgentId,
  usageOf,
} from "@/lib/agents";
import {formatCost, formatTokens} from "@/lib/usage";
import type {
  Account,
  Agent,
  AgentRecord,
  AgentRuntime,
  AgentSession,
  AgentUsage,
  AgentUsageStat,
  Provider,
  ProviderHealth,
  LlmRecordStats,
} from "@/types";

const tabs = ["Agent Sessions", "Agent Records"] as const;
type Tab = (typeof tabs)[number];

const recordLimit = 20;

function InfoRow({label, children}: {label: string; children: React.ReactNode}) {
  return (
    <div className="grid grid-cols-[140px_minmax(0,1fr)] gap-2 border-b py-2 last:border-0">
      <div className="text-sm text-muted-foreground">{label}</div>
      <div className="min-w-0 break-all text-sm">{children}</div>
    </div>
  );
}

/** The monitoring card: what the patcher reported, plus the toggle itself. */
function MonitoringCard({
  agent,
  busy,
  onToggle,
}: {
  agent: Agent;
  busy: boolean;
  onToggle: () => void;
}) {
  const action = i18next.t(`agent:${agent.patched ? "Unpatch" : "Patch"}`);
  const Icon = agent.patched ? EyeOff : Eye;
  const note = [agent.notice, agent.followup].filter(Boolean).join(" ");

  return (
    <Card>
      <CardHeader className="p-4 pb-2">
        <CardTitle className="text-base">{i18next.t("agent:Patch Status")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4 pt-0">
        <div>
          {!agent.supported ? (
            <Badge variant="secondary">{i18next.t("agent:Not supported")}</Badge>
          ) : agent.patched ? (
            <Badge variant="success">{i18next.t("agent:Patched")}</Badge>
          ) : (
            <Badge variant="secondary">{i18next.t("agent:Not patched")}</Badge>
          )}
        </div>

        {agent.detail ? <p className="text-sm text-muted-foreground">{agent.detail}</p> : null}
        {note ? <p className="text-sm text-muted-foreground">{note}</p> : null}

        {agent.supported ? (
          <ConfirmDialog
            title={`${action} ${agent.name}?`}
            description={note || undefined}
            confirmText={action}
            variant={agent.patched ? "destructive" : "default"}
            onConfirm={onToggle}
          >
            <Button
              size="sm"
              variant={agent.patched ? "outline" : "default"}
              loading={busy}
            >
              <Icon />
              {action}
            </Button>
          </ConfirmDialog>
        ) : (
          <Button size="sm" variant="outline" disabled>
            <Eye />
            {i18next.t("agent:Patch")}
          </Button>
        )}
      </CardContent>
    </Card>
  );
}

/** The run state of the installation, with the control that changes it. */
function RuntimeCard({
  agent,
  status,
  busy,
  onLocated,
  onToggle,
}: {
  agent: Agent;
  status?: AgentRuntime;
  busy: boolean;
  onLocated: () => void;
  onToggle: (agent: Agent, running: boolean) => void;
}) {
  return (
    <Card>
      <CardHeader className="p-4 pb-2">
        <CardTitle className="text-base">{i18next.t("agent:Run Status")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4 pt-0">
        <div>
          <RunBadge status={status} />
        </div>

        {status?.running ? (
          <p className="text-sm text-muted-foreground">
            {`${i18next.t("agent:Processes")}: ${status.pids.join(", ")}`}
          </p>
        ) : (
          <p className="text-sm text-muted-foreground">
            {i18next.t(status?.desktop ? "agent:Start app hint" : "agent:Start console hint")}
          </p>
        )}

        <RunButton agent={agent} status={status} busy={busy} onLocated={onLocated} onToggle={onToggle} />
      </CardContent>
    </Card>
  );
}

/**
 * What this agent has spent through the proxy, and on which provider - with the
 * shape of what its own transcripts say above it, which is the fuller of the two
 * accounts and the only one that sees a request Gateway never relayed.
 */
function UsageCard({stats, spent}: {stats: LlmRecordStats | null; spent?: AgentUsageStat}) {
  return (
    <Card>
      <CardHeader className="p-4 pb-2">
        <CardTitle className="text-base">{i18next.t("agent:Usage")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3 p-4 pt-0">
        {spent && hasUsageTrend(spent) ? (
          <AgentUsageTrend
            stat={spent}
            hints={{
              tokens: i18next.t("agent:Read from the agent's own logs"),
              requests: i18next.t("agent:Read from the agent's own logs"),
              cost: i18next.t("agent:Read from the agent's own logs"),
            }}
          />
        ) : null}

        <div className="grid grid-cols-3 gap-2">
          <div>
            <div className="text-xs text-muted-foreground">{i18next.t("llm:Requests")}</div>
            <div className="text-lg font-semibold">{(stats?.requests ?? 0).toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{i18next.t("llm:Tokens")}</div>
            <div className="text-lg font-semibold">{formatTokens(stats?.totalTokens ?? 0)}</div>
          </div>
          <div>
            <div className="text-xs text-muted-foreground">{i18next.t("llm:Cost")}</div>
            <div className="text-lg font-semibold">{formatCost(stats?.cost ?? 0)}</div>
          </div>
        </div>

        {stats && stats.failed > 0 ? (
          <p className="text-sm text-warning">
            {`${stats.failed.toLocaleString()} ${i18next.t("llm:failed")}`}
          </p>
        ) : null}

        {/* The per-provider split is what says whether the fallbacks are carrying
            traffic the bound provider could not. */}
        {stats && stats.providers.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            {stats.providers.map(provider => (
              <Badge key={provider.provider} variant="muted" className="gap-2">
                <span className="font-mono">{provider.provider}</span>
                <span>{provider.requests.toLocaleString()}</span>
                <span className="text-muted-foreground">
                  {formatTokens(provider.tokens)} · {formatCost(provider.cost)}
                </span>
              </Badge>
            ))}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground">{i18next.t("agent:No usage yet")}</p>
        )}
      </CardContent>
    </Card>
  );
}

export default function AgentDetailPage({account}: {account: Account}) {
  const params = useParams();
  const [searchParams] = useSearchParams();
  const isAdmin = Setting.isAdminUser(account);
  const {
    agents,
    error,
    busyKey,
    scanned,
    runtime,
    runBusyKey,
    scan,
    loadRuntime,
    toggleRunning,
    togglePatch,
    setRouting,
    writeProvider,
  } = useAgents(isAdmin);
  const updates = useAgentUpdates(isAdmin);
  // The upgrade rescans when it ends, so the version above is the new one.
  const installer = useAgentInstall(isAdmin, () => {
    scan(true);
    updates.reload(true);
  });
  const [tab, setTab] = React.useState<Tab>("Agent Sessions");
  const [records, setRecords] = React.useState<AgentRecord[]>([]);
  const [recordError, setRecordError] = React.useState("");
  const [providers, setProviders] = React.useState<Provider[]>([]);
  const [health, setHealth] = React.useState<ProviderHealth[]>([]);
  const [stats, setStats] = React.useState<LlmRecordStats | null>(null);
  const [usage, setUsage] = React.useState<AgentUsage>();

  const agentId = params.agentId ?? "";
  const path = searchParams.get("path") ?? "";

  // The same agent can be installed more than once, so the path from the link
  // picks the installation; without one, the first match is close enough.
  const agent = agents.find(
    candidate =>
      candidate.agentId === agentId && (path === "" || candidate.path === path),
  );

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    ProviderBackend.getProviders(account.name)
      .then(res => {
        if (res.status === "ok") {
          setProviders(res.data ?? []);
        }
      })
      .catch(() => setProviders([]));

    ProviderBackend.getProviderHealth()
      .then(res => {
        if (res.status === "ok") {
          setHealth(res.data ?? []);
        }
      })
      .catch(() => setHealth([]));
  }, [isAdmin, account.name]);

  React.useEffect(() => {
    if (!isAdmin || agentId === "") {
      return;
    }
    LlmRecordBackend.getLlmRecordStats({agent: agentId})
      .then(res => setStats(res.status === "ok" ? (res.data ?? null) : null))
      .catch(() => setStats(null));

    // What the agent wrote down itself, which counts the requests that never
    // came through Gateway. Read once: walking the transcripts is not cheap.
    AgentBackend.getAgentUsage(usageAgentId(agentId))
      .then(res => setUsage(res.status === "ok" ? (res.data ?? undefined) : undefined))
      .catch(() => undefined);
  }, [isAdmin, agentId]);

  const spent = agent ? usageOf(usage, agent) : undefined;
  const monitorId = agent ? monitorAgentId(agent.agentId) : "";
  const watching = Boolean(agent?.patched);
  const {sessions} = useAgentSessions(isAdmin && watching, monitorId, 5000);

  React.useEffect(() => {
    if (!isAdmin || !watching || monitorId === "") {
      setRecords([]);
      return undefined;
    }

    const load = () => {
      AgentBackend.getAgentRecords(monitorId, "", "", "", recordLimit)
        .then(res => {
          if (res.status === "ok") {
            setRecords(res.data ?? []);
            setRecordError("");
          } else {
            setRecordError(res.msg || i18next.t("agent:Failed to get agent records"));
          }
        })
        .catch(err => setRecordError(err.message || String(err)));
    };

    load();
    const interval = setInterval(load, 5000);
    return () => clearInterval(interval);
  }, [isAdmin, watching, monitorId]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  if (!scanned) {
    return (
      <div className="flex justify-center p-10">
        <AiDots size="small" />
      </div>
    );
  }

  if (!agent) {
    return (
      <ResultScreen
        status="404"
        title={i18next.t("agent:Agent installation not found")}
        subTitle={error || path || agentId}
        extra={
          <Button asChild>
            <Link to="/">{i18next.t("agent:Agents")}</Link>
          </Button>
        }
      />
    );
  }

  const sessionColumns: Column<AgentSession>[] = [
    {
      title: i18next.t("agent:Session"),
      key: "session",
      render: (_value, session) => (
        <div className="flex min-w-0 flex-col">
          <Link
            to={`/agent-records?agent=${encodeURIComponent(session.agent)}&session=${encodeURIComponent(session.sessionKey)}`}
            className="truncate text-primary hover:underline"
          >
            {session.title || session.sessionKey}
          </Link>
          <span className="truncate text-xs text-muted-foreground">{session.sessionKey}</span>
        </div>
      ),
    },
    {
      title: i18next.t("agent:Records"),
      key: "recordCount",
      dataIndex: "recordCount",
      width: "100px",
    },
    {
      title: i18next.t("agent:Last activity"),
      key: "lastTime",
      dataIndex: "lastTime",
      width: "200px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
  ];

  const recordColumns: Column<AgentRecord>[] = [
    {
      title: i18next.t("agent:Time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "180px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: i18next.t("agent:Event"),
      key: "event",
      width: "220px",
      render: (_value, record) => (
        <div className="flex flex-wrap items-center gap-1">
          <Badge variant="secondary">{record.eventType}</Badge>
          {record.action && <code className="text-xs">{record.action}</code>}
          {record.outcome && (
            <Badge variant={getOutcomeVariant(record.outcome)}>{record.outcome}</Badge>
          )}
        </div>
      ),
    },
    {
      title: i18next.t("agent:Target / Model"),
      key: "target",
      render: (_value, record) => {
        const target = record.mcpServer
          ? `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`
          : record.toolName;
        return (
          <div className="flex min-w-0 flex-col">
            {target && <code className="truncate text-xs">{target}</code>}
            {record.model && (
              <span className="truncate text-xs text-muted-foreground">{record.model}</span>
            )}
          </div>
        );
      },
    },
  ];

  return (
    <div className="space-y-4 p-4 md:p-6">
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" />
        {i18next.t("agent:Agents")}
      </Link>

      <div className="flex flex-wrap items-center gap-3">
        <AgentIcon
          agent={agent.agentId || agent.name}
          size={40}
          fallback={<Bot className="h-10 w-10 text-muted-foreground" />}
        />
        <h1 className="text-xl font-semibold tracking-tight">{agent.name}</h1>
        <Badge variant="secondary">{agent.version || i18next.t("agent:Unknown")}</Badge>
        <AgentUpdateBadge update={updateOf(updates.updates, agent)} />
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={() => {
            scan(true);
            loadRuntime();
          }}
        >
          <RefreshCw />
          {i18next.t("agent:Scan")}
        </Button>
      </div>

      {error && (
        <Alert variant="destructive">
          <CircleX />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <RuntimeCard
          agent={agent}
          status={runtimeOf(runtime, agent)}
          busy={runBusyKey === agentKey(agent)}
          onLocated={() => scan(true)}
          onToggle={toggleRunning}
        />

        <MonitoringCard
          agent={agent}
          busy={busyKey === agentKey(agent)}
          onToggle={() => togglePatch(agent)}
        />

        <ProviderCard
          agent={agent}
          providers={providers}
          health={health}
          busy={busyKey === agentKey(agent)}
          onRouting={routing => setRouting(agent, routing)}
          onWrite={restore => writeProvider(agent, restore)}
        />

        <UsageCard stats={stats} spent={spent} />

        <Card>
          <CardHeader className="p-4 pb-2">
            <CardTitle className="text-base">{i18next.t("agent:Installation")}</CardTitle>
          </CardHeader>
          <CardContent className="p-4 pt-0">
            <InfoRow label={i18next.t("agent:Agent")}>
              <code className="text-xs">{agent.agentId}</code>
            </InfoRow>
            <InfoRow label={i18next.t("agent:Version")}>
              <span className="flex flex-wrap items-center gap-2">
                <span className="tabular-nums">{agent.version || i18next.t("agent:Unknown")}</span>
                <AgentUpdateBadge update={updateOf(updates.updates, agent)} />
              </span>
            </InfoRow>
            <InfoRow label={i18next.t("agent:Install Method")}>
              <span className="flex flex-wrap items-center gap-2">
                {agent.installMethod || "-"}
                <ToolUpgradeConfirmDialog
                  agent={agent}
                  job={installer.jobs[agent.agentId]}
                  busy={installer.busyId === agent.agentId}
                  onConfirm={() => installer.upgrade(agent)}
                />
                <AgentVersionDialog
                  agentId={agent.agentId}
                  name={agent.name}
                  installMethod={agent.installMethod}
                  installedVersion={agent.version}
                  update={updateOf(updates.updates, agent)}
                  busy={installer.busyId === agent.agentId}
                  fallbackDetail={agent.upgrade?.detail}
                  onSelect={version => installer.setVersion(agent, version)}
                />
                <ToolUninstallConfirmDialog
                  agent={agent}
                  job={installer.jobs[agent.agentId]}
                  busy={installer.busyId === agent.agentId}
                  onConfirm={() => installer.uninstall(agent)}
                />
              </span>
            </InfoRow>
            <InfoRow label={i18next.t("general:Owner")}>{agent.owner}</InfoRow>
            {agent.account && accountLabel(agent.account) ? (
              <InfoRow label={i18next.t("agent:Account")}>
                <span className="flex flex-wrap items-center gap-2">
                  <span>{accountLabel(agent.account)}</span>
                  {agent.account.email && agent.account.email !== accountLabel(agent.account) ? (
                    <code className="text-muted-foreground text-xs">{agent.account.email}</code>
                  ) : null}
                  {agent.account.plan ? (
                    <Badge variant="muted" className="capitalize">
                      {agent.account.plan}
                    </Badge>
                  ) : null}
                </span>
              </InfoRow>
            ) : null}
            <InfoRow label={i18next.t("general:Path")}>
              <span className="flex items-start gap-1">
                <code className="min-w-0 flex-1 text-xs">{agent.path}</code>
                <SimpleTooltip title={i18next.t("agent:Copy path")}>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="h-6 w-6 shrink-0"
                    aria-label={i18next.t("agent:Copy path")}
                    onClick={() => {
                      copy(agent.path);
                      Setting.showMessage("success", i18next.t("agent:Path copied to clipboard"));
                    }}
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </Button>
                </SimpleTooltip>
              </span>
            </InfoRow>
            <InstallOutput job={installer.jobs[agent.agentId]} />
          </CardContent>
        </Card>
      </div>

      <PermissionCard agent={agent} providers={providers} />

      <AgentAccounts agent={agent} enabled={isAdmin} />

      <AgentInstances agent={agent} enabled={isAdmin} />

      {!watching ? (
        <Alert variant="info">
          <AlertDescription>
            {i18next.t("agent:Turn on monitoring to collect activity")}
          </AlertDescription>
        </Alert>
      ) : (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="inline-flex rounded-md border p-0.5">
              {tabs.map(name => (
                <button
                  key={name}
                  onClick={() => setTab(name)}
                  className={cn(
                    "rounded px-3 py-1 text-sm transition-colors",
                    tab === name
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:bg-accent",
                  )}
                >
                  {i18next.t(`agent:${name}`)}
                </button>
              ))}
            </div>
            {/* Only the records page filters by agent; the sessions page lists
                every agent, so it is linked without one. */}
            <Link
              to={
                tab === "Agent Sessions"
                  ? "/agent-sessions"
                  : `/agent-records?agent=${encodeURIComponent(monitorId)}`
              }
              className="inline-flex items-center text-sm text-primary hover:underline"
            >
              {i18next.t("agent:View all")}
              <ChevronRight className="h-4 w-4" />
            </Link>
          </div>

          {recordError && tab === "Agent Records" ? (
            <Alert variant="destructive">
              <CircleX />
              <AlertDescription>{recordError}</AlertDescription>
            </Alert>
          ) : null}

          {tab === "Agent Sessions" ? (
            <DataTable
              columns={sessionColumns}
              dataSource={sessions}
              rowKey={session => session.sessionKey}
              pageSize={0}
              emptyText={i18next.t("agent:Monitoring, no activity yet")}
            />
          ) : (
            <DataTable
              columns={recordColumns}
              dataSource={records}
              rowKey={record => String(record.id)}
              pageSize={0}
              emptyText={i18next.t("agent:Monitoring, no activity yet")}
            />
          )}
        </div>
      )}
    </div>
  );
}
