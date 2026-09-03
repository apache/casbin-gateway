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
import {Bot, FileText, MessageSquare, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/shared/data-table";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import type {Account, AgentSession} from "@/types";

export default function AgentSessionsPage({account}: {account: Account}) {
  const [sessions, setSessions] = React.useState<AgentSession[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(true);
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

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<AgentSession>[] = [
    {
      title: i18next.t("agent:Session"),
      key: "session",
      render: (_value, session) => (
        <div className="flex min-w-0 flex-col">
          {/* A session read off disk has no monitoring records behind it, so it
              opens its own transcript rather than the Activity page. */}
          <Link
            to={
              session.historical
                ? `/agent-sessions/${encodeURIComponent(session.sessionKey)}?agent=${encodeURIComponent(session.agent)}`
                : `/agent-records?agent=${encodeURIComponent(session.agent)}&session=${encodeURIComponent(session.sessionKey)}`
            }
            className="text-primary truncate font-medium hover:underline"
          >
            {session.title || session.sessionKey}
          </Link>
          <span className="text-muted-foreground truncate text-xs">
            {session.cwd || session.sessionKey}
          </span>
        </div>
      ),
    },
    {
      title: i18next.t("agent:Agent"),
      key: "agent",
      dataIndex: "agent",
      width: "180px",
      render: (value: string) => (
        <Badge variant="info">
          <AgentIcon agent={value} fallback={<Bot className="size-3" />} size={12} />
          {value}
        </Badge>
      ),
    },
    {
      title: i18next.t("agent:Source"),
      key: "historical",
      width: "150px",
      render: (_value, session) =>
        session.historical ? (
          <SimpleTooltip title={session.path}>
            <Badge variant="muted">
              <FileText className="size-3" />
              {i18next.t("agent:From the transcript")}
            </Badge>
          </SimpleTooltip>
        ) : (
          <Badge variant="success">{i18next.t("agent:Monitored")}</Badge>
        ),
    },
    {
      title: i18next.t("agent:Records"),
      key: "recordCount",
      dataIndex: "recordCount",
      width: "110px",
    },
    {
      title: i18next.t("agent:First activity"),
      key: "firstTime",
      dataIndex: "firstTime",
      width: "200px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: i18next.t("agent:Last activity"),
      key: "lastTime",
      dataIndex: "lastTime",
      width: "200px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
  ];

  return (
    <PageContainer>
      <PageHeader title={i18next.t("agent:Agent Sessions")} />

      {error ? <MessageAlert title={error} /> : null}

      <DataTable
        title={i18next.t("agent:Agent Sessions")}
        description={`${sessions.length} ${i18next.t("agent:Agent Sessions")}`}
        columns={columns}
        dataSource={sessions}
        rowKey={session => `${session.agent}:${session.sessionKey}`}
        loading={loading}
        pageSize={20}
        searchable
        emptyIcon={MessageSquare}
        emptyText={i18next.t("agent:No agent sessions yet")}
        toolbar={
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
    </PageContainer>
  );
}
