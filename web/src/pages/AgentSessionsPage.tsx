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
import {Link} from "react-router-dom";
import {Activity, Bot, FileText, Folder, Logs, MessageSquare, Radio, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/shared/data-table";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {agentOption} from "@/components/shared/brand-options";
import {SimpleSelect} from "@/components/shared/simple-select";
import {StatCard} from "@/components/shared/stat-card";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import type {Account, AgentSession} from "@/types";

type SourceFilter = "all" | "monitored" | "historical";

const dayMs = 24 * 60 * 60 * 1000;

const relativeUnits: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 86400],
  ["month", 30 * 86400],
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
];

const relativeFormatters = new Map<string, Intl.RelativeTimeFormat>();

function relativeFormatter() {
  const language = i18next.language || "en";
  let formatter = relativeFormatters.get(language);
  if (!formatter) {
    formatter = new Intl.RelativeTimeFormat(language, {numeric: "auto"});
    relativeFormatters.set(language, formatter);
  }
  return formatter;
}

/** "3 minutes ago" - a wall of timestamps is unreadable at a glance. */
function relativeTime(value?: string) {
  const time = value ? new Date(value).getTime() : NaN;
  if (!Number.isFinite(time)) {
    return "";
  }

  const seconds = (time - Date.now()) / 1000;
  for (const [unit, size] of relativeUnits) {
    if (Math.abs(seconds) >= size) {
      return relativeFormatter().format(Math.round(seconds / size), unit);
    }
  }
  return relativeFormatter().format(Math.round(seconds), "second");
}

function shortTime(value?: string) {
  const date = value ? new Date(value) : null;
  if (!date || Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toLocaleString(i18next.language, {month: "short", day: "numeric", hour: "2-digit", minute: "2-digit"});
}

function fullTime(value?: string) {
  const date = value ? new Date(value) : null;
  return !date || Number.isNaN(date.getTime()) ? "" : date.toLocaleString();
}

/** The folder alone: the same repo path repeated down a column says nothing. */
function baseName(path: string) {
  const parts = path.split(/[\\/]/).filter(Boolean);
  return parts[parts.length - 1] || path;
}

export default function AgentSessionsPage({account}: {account: Account}) {
  const [sessions, setSessions] = React.useState<AgentSession[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(true);
  const [source, setSource] = React.useState<SourceFilter>("all");
  const [agent, setAgent] = React.useState("");
  const isAdmin = Setting.isAdminUser(account);

  // The poll below runs every few seconds and must not spin the refresh button
  // or blank the rows the operator is reading.
  const load = React.useCallback((foreground = true) => {
    if (!isAdmin) {
      return;
    }

    if (foreground) {
      setLoading(true);
    }
    AgentBackend.getAgentSessions()
      .then(res => {
        if (res.status === "ok") {
          setSessions(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("agent:Failed to get agent sessions"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => {
        if (foreground) {
          setLoading(false);
        }
      });
  }, [isAdmin]);

  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }

    load(true);
    if (!autoRefresh) {
      return undefined;
    }
    const interval = setInterval(() => load(false), 3000);
    return () => clearInterval(interval);
  }, [autoRefresh, isAdmin, load]);

  const ordered = React.useMemo(
    () => [...sessions].sort((a, b) => new Date(b.lastTime).getTime() - new Date(a.lastTime).getTime()),
    [sessions],
  );

  const agentOptions = React.useMemo(
    () => Array.from(new Set(sessions.map(session => session.agent).filter(Boolean))).sort(),
    [sessions],
  );

  const monitored = ordered.filter(session => !session.historical).length;
  const historical = ordered.length - monitored;
  const records = ordered.reduce((total, session) => total + (session.recordCount || 0), 0);
  const activeToday = ordered.filter(session => Date.now() - new Date(session.lastTime).getTime() < dayMs).length;
  const lastTime = ordered[0]?.lastTime;

  const rows = React.useMemo(
    () =>
      ordered.filter(session => {
        const matchesSource =
          source === "all" || (source === "historical" ? Boolean(session.historical) : !session.historical);
        return matchesSource && (agent === "" || session.agent === agent);
      }),
    [agent, ordered, source],
  );

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<AgentSession>[] = [
    {
      title: i18next.t("agent:Session"),
      key: "session",
      dataIndex: "title",
      ellipsis: true,
      // Without a floor the title column is the one that gives way when the
      // fixed ones do not fit, and the only column worth reading goes first.
      className: "min-w-[240px]",
      sorter: (a, b) => (a.title || a.sessionKey).localeCompare(b.title || b.sessionKey),
      render: (_value, session) => (
        <div className="flex min-w-0 flex-col gap-0.5">
          {/* A session read off disk has no monitoring records behind it, so it
              opens its own transcript rather than the Activity page. */}
          <Link
            to={
              session.historical
                ? `/agent-sessions/${encodeURIComponent(session.sessionKey)}?agent=${encodeURIComponent(session.agent)}`
                : `/agent-records?agent=${encodeURIComponent(session.agent)}&session=${encodeURIComponent(session.sessionKey)}`
            }
            className="hover:text-primary truncate font-medium transition-colors"
          >
            {session.title || session.sessionKey}
          </Link>
          <SimpleTooltip title={session.cwd || session.sessionKey}>
            <span className="text-muted-foreground flex w-fit max-w-full items-center gap-1.5 text-xs">
              {session.cwd ? <Folder className="size-3 shrink-0" /> : null}
              <span className="truncate font-mono">
                {session.cwd ? baseName(session.cwd) : session.sessionKey}
              </span>
            </span>
          </SimpleTooltip>
        </div>
      ),
    },
    {
      title: i18next.t("agent:Agent"),
      key: "agent",
      dataIndex: "agent",
      width: "170px",
      sorter: (a, b) => a.agent.localeCompare(b.agent),
      render: (value: string) => (
        <span className="flex min-w-0 items-center gap-2">
          <span className="bg-muted text-muted-foreground flex size-5 shrink-0 items-center justify-center rounded">
            <AgentIcon agent={value} fallback={<Bot className="size-3" />} size={12} />
          </span>
          <span className="truncate">{value}</span>
        </span>
      ),
    },
    {
      title: i18next.t("agent:Source"),
      key: "historical",
      width: "140px",
      className: "hidden xl:table-cell",
      headerClassName: "hidden xl:table-cell",
      render: (_value, session) =>
        session.historical ? (
          <SimpleTooltip title={session.path}>
            <Badge variant="muted">
              <FileText />
              {i18next.t("agent:From the transcript")}
            </Badge>
          </SimpleTooltip>
        ) : (
          <Badge variant="success">
            <Radio />
            {i18next.t("agent:Monitored")}
          </Badge>
        ),
    },
    {
      title: i18next.t("agent:Records"),
      key: "recordCount",
      dataIndex: "recordCount",
      width: "100px",
      align: "right",
      sorter: (a, b) => a.recordCount - b.recordCount,
      render: (value: number) => (
        <span className={value ? "tabular-nums" : "text-muted-foreground tabular-nums"}>
          {(value || 0).toLocaleString()}
        </span>
      ),
    },
    {
      title: i18next.t("agent:First activity"),
      key: "firstTime",
      dataIndex: "firstTime",
      width: "150px",
      className: "hidden 2xl:table-cell",
      headerClassName: "hidden 2xl:table-cell",
      sorter: (a, b) => new Date(a.firstTime).getTime() - new Date(b.firstTime).getTime(),
      render: (value: string) => (
        <SimpleTooltip title={fullTime(value)}>
          <span className="text-muted-foreground whitespace-nowrap">{shortTime(value)}</span>
        </SimpleTooltip>
      ),
    },
    {
      title: i18next.t("agent:Last activity"),
      key: "lastTime",
      dataIndex: "lastTime",
      width: "150px",
      sorter: (a, b) => new Date(a.lastTime).getTime() - new Date(b.lastTime).getTime(),
      render: (value: string) => (
        <SimpleTooltip title={fullTime(value)}>
          <span className="whitespace-nowrap">{relativeTime(value)}</span>
        </SimpleTooltip>
      ),
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agent Sessions")}
        description={i18next.t("agent:Agent Sessions detail")}
        actions={
          <>
            <Label className="text-sm font-normal">
              <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
              {i18next.t("agent:Auto refresh")}
            </Label>
            <Button variant="outline" size="sm" onClick={() => load(true)} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label={i18next.t("agent:Agent Sessions")}
          value={ordered.length.toLocaleString()}
          icon={MessageSquare}
          hint={i18next.t("agent:Across {count} agents").replace("{count}", String(agentOptions.length))}
        />
        <StatCard
          label={i18next.t("agent:Active today")}
          value={activeToday.toLocaleString()}
          icon={Activity}
          hint={lastTime ? `${i18next.t("agent:Last activity")} · ${relativeTime(lastTime)}` : undefined}
        />
        <StatCard
          label={i18next.t("agent:Monitored")}
          value={monitored.toLocaleString()}
          icon={Radio}
          hint={i18next.t("agent:{count} from transcripts").replace("{count}", historical.toLocaleString())}
        />
        <StatCard label={i18next.t("agent:Records")} value={records.toLocaleString()} icon={Logs} />
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <Tabs value={source} onValueChange={value => setSource(value as SourceFilter)}>
          <TabsList>
            <TabsTrigger value="all">
              {i18next.t("agent:Any source")}
              <Badge variant="muted">{ordered.length}</Badge>
            </TabsTrigger>
            <TabsTrigger value="monitored">
              {i18next.t("agent:Monitored")}
              <Badge variant="muted">{monitored}</Badge>
            </TabsTrigger>
            <TabsTrigger value="historical">
              {i18next.t("agent:From the transcript")}
              <Badge variant="muted">{historical}</Badge>
            </TabsTrigger>
          </TabsList>
        </Tabs>

        <SimpleSelect
          className="w-[200px]"
          value={agent}
          onChange={setAgent}
          aria-label={i18next.t("agent:Agent")}
          options={[
            {label: i18next.t("agent:All agents"), value: ""},
            ...agentOptions.map(name => agentOption(name)),
          ]}
        />
      </div>

      <DataTable
        description={
          rows.length === ordered.length
            ? `${ordered.length.toLocaleString()} ${i18next.t("agent:Agent Sessions")}`
            : `${rows.length.toLocaleString()} / ${ordered.length.toLocaleString()} ${i18next.t("agent:Agent Sessions")}`
        }
        columns={columns}
        dataSource={rows}
        rowKey={session => `${session.agent}:${session.sessionKey}`}
        loading={loading}
        pageSize={20}
        searchable
        emptyIcon={MessageSquare}
        emptyText={i18next.t("agent:No agent sessions yet")}
      />
    </PageContainer>
  );
}
