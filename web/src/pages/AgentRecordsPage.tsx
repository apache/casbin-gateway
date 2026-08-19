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
import {Bot, CircleX, Info, RefreshCw} from "lucide-react";
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
import {Input} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Switch} from "@/components/ui/switch";
import type {Account, AgentRecord} from "@/types";

function monitorAgentId(agentId: string) {
  return agentId === "codex_vscode" || agentId === "codex-vscode" ? "codex-cli" : agentId;
}

// The server keeps a bounded in-memory window; these are the slices of it the
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

function getOutcomeVariant(outcome: string | undefined) {
  return (
    {
      attempted: "processing",
      denied: "warning",
      failure: "error",
      success: "success",
    } as const
  )[outcome as "attempted" | "denied" | "failure" | "success"];
}

function DetailField({label, children}: {label: string; children: React.ReactNode}) {
  return (
    <div className="grid grid-cols-[160px_minmax(0,1fr)] gap-2 border-b py-1.5 last:border-0">
      <div className="text-xs font-medium text-muted-foreground">{label}</div>
      <div className="min-w-0 break-words text-sm">{children}</div>
    </div>
  );
}

function RecordDetail({record}: {record: AgentRecord}) {
  const mcpTarget =
    record.mcpServer && `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`;

  return (
    <div className="px-4 py-2">
      <DetailField label={i18next.t("general:ID")}>{record.id}</DetailField>
      <DetailField label={i18next.t("agent:Time")}>
        {new Date(record.createdTime).toLocaleString()}
      </DetailField>
      {record.agentPath && (
        <DetailField label={i18next.t("agent:Agent path")}>
          <code className="text-xs">{record.agentPath}</code>
        </DetailField>
      )}
      {record.user && <DetailField label={i18next.t("agent:User")}>{record.user}</DetailField>}
      {record.sessionKey && (
        <DetailField label={i18next.t("agent:Session")}>
          <code className="text-xs">{record.sessionKey}</code>
        </DetailField>
      )}
      {record.title && (
        <DetailField label={i18next.t("agent:Session title")}>{record.title}</DetailField>
      )}
      {record.promptId && (
        <DetailField label={i18next.t("agent:Prompt ID")}>
          <code className="text-xs">{record.promptId}</code>
        </DetailField>
      )}
      {record.toolUseId && (
        <DetailField label={i18next.t("agent:Tool use ID")}>
          <code className="text-xs">{record.toolUseId}</code>
        </DetailField>
      )}
      {record.toolName && (
        <DetailField label={i18next.t("agent:Tool")}>
          <code className="text-xs">{record.toolName}</code>
        </DetailField>
      )}
      {mcpTarget && (
        <DetailField label={i18next.t("agent:MCP target")}>
          <code className="text-xs">{mcpTarget}</code>
        </DetailField>
      )}
      {record.model && (
        <DetailField label={i18next.t("agent:Model")}>
          <code className="text-xs">{record.model}</code>
        </DetailField>
      )}
      {record.durationMs !== undefined && (
        <DetailField label={i18next.t("agent:Duration")}>
          {record.durationMs.toLocaleString()} ms
        </DetailField>
      )}
      {record.clientIp && (
        <DetailField label={i18next.t("agent:Reported from")}>
          <code className="text-xs">{record.clientIp}</code>
        </DetailField>
      )}
      {record.detail && <DetailField label={i18next.t("agent:Detail")}>{record.detail}</DetailField>}
      {record.object ? (
        <DetailField label={i18next.t("agent:Payload")}>
          <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-all rounded bg-muted p-2 font-mono text-xs">
            {formatPayload(record.object)}
          </pre>
        </DetailField>
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
        <Badge variant="blue">
          <AgentIcon agent={value} fallback={<Bot className="h-4 w-4" />} size={16} />
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
      width: "200px",
      render: (_value, record) => {
        const target = record.mcpServer
          ? `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`
          : record.toolName;
        return (
          <div className="flex flex-col">
            {target && <code className="truncate text-xs">{target}</code>}
            {record.model && <span className="truncate text-xs text-muted-foreground">{record.model}</span>}
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
              className="truncate text-xs text-primary hover:underline"
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
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("agent:Agent Records")}>
        <label className="flex items-center gap-2 text-sm">
          <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
          {i18next.t("agent:Auto refresh")}
        </label>
        <Button variant="outline" onClick={() => load(true)} disabled={loading}>
          <RefreshCw className={loading ? "animate-spin" : undefined} />
          {i18next.t("general:Refresh")}
        </Button>
      </PageHeader>

      <Alert variant="info" className="mb-4">
        <Info />
        <AlertDescription>
          {i18next.t("agent:Records are kept in memory only and are lost when Gateway restarts")}
        </AlertDescription>
      </Alert>

      <div className="mb-4 flex flex-wrap gap-2">
        <Select value={agent || ALL} onValueChange={value => setFilter("agent", value)}>
          <SelectTrigger className="w-[190px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{i18next.t("agent:All agents")}</SelectItem>
            {agentOptions.map(value => (
              <SelectItem key={value} value={value}>
                {value}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={eventType || ALL} onValueChange={value => setFilter("eventType", value)}>
          <SelectTrigger className="w-[180px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{i18next.t("agent:All event types")}</SelectItem>
            {["session", "prompt", "llm", "tool", "mcp", "permission", "subagent", "compact"].map(
              value => (
                <SelectItem key={value} value={value}>
                  {value}
                </SelectItem>
              ),
            )}
          </SelectContent>
        </Select>

        <Select value={outcome || ALL} onValueChange={value => setFilter("outcome", value)}>
          <SelectTrigger className="w-[170px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{i18next.t("agent:All outcomes")}</SelectItem>
            {["attempted", "success", "failure", "denied"].map(value => (
              <SelectItem key={value} value={value}>
                {value}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select
          value={String(limit)}
          onValueChange={value =>
            setFilter("limit", Number(value) === defaultLimit ? "" : value)
          }
        >
          <SelectTrigger className="w-[190px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {limitOptions.map(value => (
              <SelectItem key={value} value={String(value)}>
                {i18next.t("agent:Last {count} records").replace("{count}", String(value))}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

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

      {error && (
        <Alert variant="destructive" className="mb-4">
          <CircleX />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <DataTable
        columns={columns}
        data={records}
        rowKey={record => record.id}
        loading={loading}
        pageSize={20}
        emptyText={i18next.t("agent:No agent records yet - patch an agent to start collecting them")}
        expandedRowRender={record => <RecordDetail record={record} />}
      />
    </div>
  );
}
