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
import {Bot, CircleX, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/DataTable";
import {PageHeader} from "@/components/FormRow";
import {UnauthorizedResult} from "@/components/Result";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import type {Account, AgentSession} from "@/types";

export default function AgentSessionsPage({account}: {account: Account}) {
  const [sessions, setSessions] = React.useState<AgentSession[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const isAdmin = Setting.isAdminUser(account);

  const load = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }

    setLoading(true);
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
      .then(() => setLoading(false));
  }, [isAdmin]);

  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }

    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, [isAdmin, load]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<AgentSession>[] = [
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
      title: i18next.t("agent:Agent"),
      key: "agent",
      dataIndex: "agent",
      width: "180px",
      render: (value: string) => (
        <Badge variant="blue">
          <AgentIcon agent={value} fallback={<Bot className="h-4 w-4" />} size={16} />
          {value}
        </Badge>
      ),
    },
    {
      title: i18next.t("agent:Records"),
      key: "recordCount",
      dataIndex: "recordCount",
      width: "120px",
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
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("agent:Agent Sessions")}>
        <Button variant="outline" onClick={load} disabled={loading}>
          <RefreshCw className={loading ? "animate-spin" : undefined} />
          {i18next.t("general:Refresh")}
        </Button>
      </PageHeader>

      {error && (
        <Alert variant="destructive" className="mb-4">
          <CircleX />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <DataTable
        columns={columns}
        data={sessions}
        rowKey={session => `${session.agent}:${session.sessionKey}`}
        loading={loading}
        pageSize={20}
        emptyText={i18next.t("agent:No agent sessions yet - patch an agent to start collecting them")}
      />
    </div>
  );
}
