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
import {Link, useNavigate, useSearchParams} from "react-router-dom";
import {Bot, FileSearch, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/shared/data-table";
import {CodeBlock, CodeText, DescriptionList, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {agentOption} from "@/components/shared/brand-options";
import {SimpleSelect} from "@/components/shared/simple-select";
import {getOutcomeVariant, monitorAgentId} from "@/lib/agents";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import type {Account, AgentRecord} from "@/types";

// The server keeps a bounded window of rows; these are the slices of it the
// page can ask for. Without a control here the UI could only ever reach the
// first 200 of the records the backend was holding.
const limitOptions = [200, 500, 1000, 5000];
const defaultLimit = 200;
// A blank string is not a legal Radix Select value, so "all" stands in for the
// unset filter and is translated back to "" when it reaches the query string.
const ALL = "all";

function formatPayload(object: unknown) {
  if (typeof object !== "string") {
    return JSON.stringify(object, null, 2);
  }

  try {
    return JSON.stringify(JSON.parse(object), null, 2);
  } catch {
    return object;
  }
}

function RecordDetail({record}: {record: AgentRecord}) {
  const mcpTarget = record.mcpServer && `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`;

  return (
    <div className="flex flex-col gap-3 px-4 py-3">
      <DescriptionList
        columns={3}
        items={[
          {label: i18next.t("general:ID"), value: record.id},
          {label: i18next.t("agent:Time"), value: new Date(record.createdTime).toLocaleString()},
          record.agentPath && {label: i18next.t("agent:Agent path"), value: <CodeText>{record.agentPath}</CodeText>},
          record.user && {label: i18next.t("agent:User"), value: record.user},
          record.sessionKey && {label: i18next.t("agent:Session"), value: <CodeText>{record.sessionKey}</CodeText>},
          record.title && {label: i18next.t("agent:Session title"), value: record.title},
          record.promptId && {label: i18next.t("agent:Prompt ID"), value: <CodeText>{record.promptId}</CodeText>},
          record.toolUseId && {label: i18next.t("agent:Tool use ID"), value: <CodeText>{record.toolUseId}</CodeText>},
          record.toolName && {label: i18next.t("agent:Tool"), value: <CodeText>{record.toolName}</CodeText>},
          mcpTarget && {label: i18next.t("agent:MCP target"), value: <CodeText>{mcpTarget}</CodeText>},
          record.model && {label: i18next.t("agent:Model"), value: <CodeText>{record.model}</CodeText>},
          record.durationMs !== undefined && {
            label: i18next.t("agent:Duration"),
            value: `${record.durationMs.toLocaleString()} ms`,
          },
          record.clientIp && {label: i18next.t("agent:Reported from"), value: <CodeText>{record.clientIp}</CodeText>},
          record.detail && {label: i18next.t("agent:Detail"), value: record.detail},
        ]}
      />
      {record.object ? (
        <div className="grid gap-1">
          <span className="text-muted-foreground text-xs">{i18next.t("agent:Payload")}</span>
          <CodeBlock copyable maxHeight="20rem">
            {formatPayload(record.object)}
          </CodeBlock>
        </div>
      ) : null}
    </div>
  );
}

export default function AgentRecordsPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const agent = searchParams.get("agent") || "";
  const eventType = searchParams.get("eventType") || "";
  const outcome = searchParams.get("outcome") || "";
  const session = searchParams.get("session") || "";
  const limit = Number(searchParams.get("limit")) || defaultLimit;
  const isAdmin = Setting.isAdminUser(account);

  const [agents, setAgents] = React.useState<{agentId: string}[]>([]);
  const [records, setRecords] = React.useState<AgentRecord[]>([]);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [autoRefresh, setAutoRefresh] = React.useState(true);
  const [sessionDraft, setSessionDraft] = React.useState(session);

  const load = React.useCallback(
    (foreground = true) => {
      if (!isAdmin) {
        return;
      }

      // Background refreshes must not raise the table's loading state: the poll
      // below runs every few seconds and would otherwise cover the rows the
      // operator is reading, and spin the refresh button permanently.
      if (foreground) {
        setLoading(true);
      }
      AgentBackend.getAgentRecords(agent, eventType, outcome, session, limit)
        .then(res => {
          if (res.status === "ok") {
            setRecords(res.data ?? []);
            setError("");
          } else {
            setError(res.msg || i18next.t("agent:Failed to get agent records"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => {
          if (foreground) {
            setLoading(false);
          }
        });
    },
    [agent, eventType, isAdmin, limit, outcome, session],
  );

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    AgentBackend.getAgents().then(res => {
      if (res.status === "ok") {
        setAgents(res.data ?? []);
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

  React.useEffect(() => {
    setSessionDraft(session);
  }, [session]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const setFilter = (key: string, value: string) => {
    const params = new URLSearchParams(searchParams);
    if (value && value !== ALL) {
      params.set(key, value);
    } else {
      params.delete(key);
    }
    const query = params.toString();
    navigate(query ? `/agent-records?${query}` : "/agent-records");
  };

  const agentOptions = Array.from(
    new Set(agents.map(item => monitorAgentId(item.agentId))),
  ).filter(Boolean);

  const columns: Column<AgentRecord>[] = [
    {
      title: i18next.t("agent:Time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "180px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: i18next.t("agent:Agent"),
      key: "agent",
      dataIndex: "agent",
      width: "160px",
      render: (value: string) => (
        <Badge variant="info">
          <AgentIcon agent={value} fallback={<Bot className="size-3" />} size={12} />
          {value}
        </Badge>
      ),
    },
    {
      title: i18next.t("agent:Event"),
      key: "event",
      width: "200px",
      render: (_value, record) => (
        <div className="flex flex-wrap items-center gap-1">
          <Badge variant="muted">{record.eventType}</Badge>
          {record.action && <CodeText>{record.action}</CodeText>}
          {record.outcome && <Badge variant={getOutcomeVariant(record.outcome)}>{record.outcome}</Badge>}
        </div>
      ),
    },
    {
      title: i18next.t("agent:Target / Model"),
      key: "target",
      width: "200px",
      render: (_value, record) => {
        const target = record.mcpServer
          ? `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`
          : record.toolName;
        return (
          <div className="flex min-w-0 flex-col gap-0.5">
            {target && <CodeText>{target}</CodeText>}
            {record.model && <span className="text-muted-foreground truncate text-xs">{record.model}</span>}
          </div>
        );
      },
    },
    {
      title: i18next.t("agent:Session"),
      key: "sessionKey",
      dataIndex: "sessionKey",
      width: "240px",
      render: (value: string, record) =>
        value ? (
          <div className="flex min-w-0 flex-col">
            {record.title && <span className="truncate font-medium">{record.title}</span>}
            <Link
              to={`/agent-records?agent=${encodeURIComponent(record.agent)}&session=${encodeURIComponent(value)}`}
              className="text-primary truncate text-xs hover:underline"
            >
              {value}
            </Link>
          </div>
        ) : null,
    },
    {
      title: i18next.t("agent:Duration"),
      key: "durationMs",
      dataIndex: "durationMs",
      width: "120px",
      render: (value: number | undefined) =>
        value === undefined ? null : `${value.toLocaleString()} ms`,
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agent Records")}
        description={i18next.t("agent:Records are stored in the database and survive a Gateway restart")}
        actions={
          <>
            <Label className="text-sm font-normal">
              <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
              {i18next.t("agent:Auto refresh")}
            </Label>
            <Button variant="outline" onClick={() => load(true)} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      <div className="flex flex-wrap items-center gap-2">
        <SimpleSelect
          className="w-[190px]"
          value={agent || ALL}
          onChange={value => setFilter("agent", value)}
          options={[
            {label: i18next.t("agent:All agents"), value: ALL},
            ...agentOptions.map(value => agentOption(value)),
          ]}
        />
        <SimpleSelect
          className="w-[180px]"
          value={eventType || ALL}
          onChange={value => setFilter("eventType", value)}
          options={[
            {label: i18next.t("agent:All event types"), value: ALL},
            ...["session", "prompt", "llm", "tool", "mcp", "permission", "subagent", "compact"].map(value => ({
              label: value,
              value,
            })),
          ]}
        />
        <SimpleSelect
          className="w-[170px]"
          value={outcome || ALL}
          onChange={value => setFilter("outcome", value)}
          options={[
            {label: i18next.t("agent:All outcomes"), value: ALL},
            ...["attempted", "success", "failure", "denied"].map(value => ({label: value, value})),
          ]}
        />
        <SimpleSelect
          className="w-[190px]"
          value={String(limit)}
          onChange={value => setFilter("limit", Number(value) === defaultLimit ? "" : value)}
          options={limitOptions.map(value => ({
            label: i18next.t("agent:Last {count} records").replace("{count}", String(value)),
            value: String(value),
          }))}
        />

        <form
          className="flex gap-2"
          onSubmit={event => {
            event.preventDefault();
            setFilter("session", sessionDraft);
          }}
        >
          <Input
            className="w-[260px]"
            placeholder={i18next.t("agent:Session")}
            value={sessionDraft}
            onChange={event => setSessionDraft(event.target.value)}
          />
          <Button type="submit" variant="outline">
            {i18next.t("agent:Filter")}
          </Button>
        </form>
      </div>

      <DataTable
        title={i18next.t("agent:Agent Records")}
        description={`${records.length} ${i18next.t("agent:Records")}`}
        columns={columns}
        dataSource={records}
        rowKey={record => String(record.id)}
        loading={loading}
        pageSize={20}
        searchable
        emptyIcon={FileSearch}
        emptyText={i18next.t("agent:No agent records yet - patch an agent to start collecting them")}
        expandable={{expandedRowRender: record => <RecordDetail record={record} />}}
      />
    </PageContainer>
  );
}
