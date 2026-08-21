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
import {Logs, RefreshCw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as Setting from "@/Setting";
import {LlmUsagePanel} from "@/components/LlmUsagePanel";
import {DataTable, type Column} from "@/components/shared/data-table";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {CodeBlock, CodeText, DescriptionList, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import type {Account, LlmRecord, LlmRecordStatus} from "@/types";

function statusVariant(status: number): BadgeVariant {
  if (status >= 200 && status < 300) {
    return "success";
  }
  if (status >= 400 && status < 500) {
    return "warning";
  }
  return "danger";
}

function formatPayload(payload: string) {
  try {
    return JSON.stringify(JSON.parse(payload), null, 2);
  } catch {
    return payload;
  }
}

/** Mounted only once a row is expanded, which is when the body is fetched. */
function RecordDetail({record, onDelete}: {record: LlmRecord; onDelete: () => void}) {
  const [detail, setDetail] = React.useState<LlmRecord | null>(null);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    LlmRecordBackend.getLlmRecord(record.id)
      .then(res => (res.status === "ok" ? setDetail(res.data ?? null) : setError(res.msg)))
      .catch(err => setError(err.message || String(err)));
  }, [record.id]);

  const payload = detail?.payload ?? "";

  return (
    <div className="flex flex-col gap-3 px-4 py-3">
      <DescriptionList
        columns={3}
        items={[
          {label: i18next.t("general:ID"), value: record.id},
          {label: i18next.t("general:Created time"), value: Setting.getFormattedDate(record.createdTime)},
          {label: i18next.t("llm:Endpoint"), value: <CodeText>{`${record.protocol} ${record.endpoint}`}</CodeText>},
          record.channel && {label: i18next.t("llm:Channel"), value: <CodeText>{record.channel}</CodeText>},
          record.agent && {label: i18next.t("agent:Agent"), value: <CodeText>{record.agent}</CodeText>},
          {label: i18next.t("llm:Attempts"), value: record.attempts},
          {label: i18next.t("llm:Tokens"), value: `${record.promptTokens} + ${record.completionTokens} = ${record.totalTokens}`},
          {label: i18next.t("llm:Request size"), value: `${record.bytes.toLocaleString()} B`},
          {label: i18next.t("llm:Redacted values"), value: record.redactions},
          record.error && {label: i18next.t("llm:Error"), value: record.error},
        ]}
      />

      {error ? <MessageAlert title={error} /> : null}

      {payload ? (
        <div className="grid gap-1">
          <span className="text-muted-foreground text-xs">{i18next.t("agent:Payload")}</span>
          <CodeBlock copyable maxHeight="24rem">
            {formatPayload(payload)}
          </CodeBlock>
        </div>
      ) : (
        <span className="text-muted-foreground text-xs">
          {record.truncated ? i18next.t("llm:Body too large to store") : i18next.t("llm:Body not stored")}
        </span>
      )}

      <div>
        <ConfirmDialog
          title={i18next.t("general:Sure to delete {name} ?").replace("{name}", String(record.id))}
          onConfirm={onDelete}
        >
          <Button size="sm" variant="destructive">
            <Trash2 />
            {i18next.t("general:Delete")}
          </Button>
        </ConfirmDialog>
      </div>
    </div>
  );
}

export default function LlmRecordsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [records, setRecords] = React.useState<LlmRecord[]>([]);
  const [status, setStatus] = React.useState<LlmRecordStatus | null>(null);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(25);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [autoRefresh, setAutoRefresh] = React.useState(false);

  const [outcome, setOutcome] = React.useState("");
  const [modelDraft, setModelDraft] = React.useState("");
  const [clientIpDraft, setClientIpDraft] = React.useState("");
  const [filter, setFilter] = React.useState<LlmRecordBackend.LlmRecordFilter>({});

  const load = React.useCallback(
    (nextPage = page, nextPageSize = pageSize, foreground = true) => {
      if (!isAdmin) {
        return;
      }
      // A background poll must not raise the loading state, or it would cover
      // the rows the operator is reading every few seconds.
      if (foreground) {
        setLoading(true);
      }

      LlmRecordBackend.getLlmRecords(nextPage, nextPageSize, {...filter, outcome: outcome})
        .then(res => {
          if (res.status === "ok") {
            setRecords(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setError("");
          } else {
            setError(res.msg || i18next.t("general:Failed to get data"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => {
          if (foreground) {
            setLoading(false);
          }
        });
    },
    [filter, isAdmin, outcome, page, pageSize],
  );

  const loadStatus = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    LlmRecordBackend.getLlmRecordStatus().then(res => {
      if (res.status === "ok") {
        setStatus(res.data ?? null);
      }
    });
  }, [isAdmin]);

  React.useEffect(() => {
    load(1, pageSize);
    loadStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filter, outcome]);

  React.useEffect(() => {
    if (!autoRefresh) {
      return undefined;
    }
    const interval = setInterval(() => load(page, pageSize, false), 5000);
    return () => clearInterval(interval);
  }, [autoRefresh, load, page, pageSize]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const deleteRecord = (id: number) => {
    LlmRecordBackend.deleteLlmRecord(id)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          load(page, pageSize);
        }
      })
      .catch(err => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${err}`));
  };

  const clearRecords = () => {
    LlmRecordBackend.clearLlmRecords()
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          load(1, pageSize);
          loadStatus();
        }
      })
      .catch(err => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${err}`));
  };

  const applyFilter = () => setFilter({model: modelDraft, clientIp: clientIpDraft});

  const columns: Column<LlmRecord>[] = [
    {
      title: i18next.t("agent:Time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "170px",
      render: (value: string) => Setting.getFormattedDate(value),
    },
    {
      title: i18next.t("agent:Model"),
      key: "model",
      dataIndex: "model",
      width: "190px",
      render: (value: string, record) => (
        <div className="flex min-w-0 flex-col gap-0.5">
          <CodeText>{value}</CodeText>
          <span className="text-muted-foreground truncate text-xs">{record.agent || record.channel}</span>
        </div>
      ),
    },
    {
      title: i18next.t("general:Client ip"),
      key: "clientIp",
      dataIndex: "clientIp",
      width: "130px",
    },
    {
      title: i18next.t("llm:Status"),
      key: "status",
      width: "130px",
      render: (_value, record) => (
        <div className="flex flex-wrap items-center gap-1">
          <Badge variant={statusVariant(record.status)}>{record.status || i18next.t("llm:No response")}</Badge>
          {record.stream ? <Badge variant="muted">SSE</Badge> : null}
        </div>
      ),
    },
    {
      title: i18next.t("agent:Duration"),
      key: "durationMs",
      dataIndex: "durationMs",
      width: "110px",
      render: (value: number) => `${value.toLocaleString()} ms`,
    },
    {
      title: i18next.t("llm:Tokens"),
      key: "totalTokens",
      width: "120px",
      render: (_value, record) =>
        record.totalTokens > 0 ? (
          <div className="flex min-w-0 flex-col gap-0.5">
            <span>{record.totalTokens.toLocaleString()}</span>
            <span className="text-muted-foreground truncate text-xs">
              {record.promptTokens.toLocaleString()} + {record.completionTokens.toLocaleString()}
            </span>
          </div>
        ) : null,
    },
    {
      title: i18next.t("llm:Request"),
      key: "summary",
      render: (_value, record) => (
        <span className="text-muted-foreground block truncate text-xs" title={record.summary || record.error}>
          {record.summary || record.error}
        </span>
      ),
    },
  ];

  const modeOff = status !== null && status.mode === "off";
  const description = status
    ? i18next
      .t("llm:Mode {mode}, kept for {days} days, up to {max} records")
      .replace("{mode}", i18next.t(`llm:Mode ${status.mode}`))
      .replace("{days}", String(status.retentionDays))
      .replace("{max}", status.maxRecords.toLocaleString())
    : undefined;

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("llm:LLM Records")}
        description={description}
        actions={
          <>
            <Label className="text-sm font-normal">
              <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
              {i18next.t("agent:Auto refresh")}
            </Label>
            <Button variant="outline" onClick={() => load(page, pageSize)} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
            <ConfirmDialog title={i18next.t("llm:Delete every stored record?")} onConfirm={clearRecords}>
              <Button variant="outline" disabled={total === 0}>
                <Trash2 />
                {i18next.t("llm:Clear")}
              </Button>
            </ConfirmDialog>
          </>
        }
      />

      {modeOff ? <MessageAlert variant="info" title={i18next.t("llm:Recording is off")} description={i18next.t("llm:Recording is off detail")} /> : null}
      {status && status.dropped > 0 ? (
        <MessageAlert
          variant="warning"
          title={i18next.t("llm:Records were dropped").replace("{count}", status.dropped.toLocaleString())}
        />
      ) : null}
      {error ? <MessageAlert title={error} /> : null}

      {modeOff ? null : <LlmUsagePanel />}

      <form
        className="flex flex-wrap items-center gap-2"
        onSubmit={event => {
          event.preventDefault();
          applyFilter();
        }}
      >
        <Input
          className="w-[220px]"
          placeholder={i18next.t("agent:Model")}
          value={modelDraft}
          onChange={event => setModelDraft(event.target.value)}
        />
        <Input
          className="w-[190px]"
          placeholder={i18next.t("general:Client ip")}
          value={clientIpDraft}
          onChange={event => setClientIpDraft(event.target.value)}
        />
        <SimpleSelect
          className="w-[170px]"
          value={outcome}
          onChange={setOutcome}
          options={[
            {label: i18next.t("llm:Any outcome"), value: ""},
            {label: i18next.t("llm:Succeeded"), value: "ok"},
            {label: i18next.t("llm:Failed"), value: "error"},
          ]}
        />
        <Button type="submit" variant="outline">
          {i18next.t("agent:Filter")}
        </Button>
      </form>

      <DataTable
        title={i18next.t("llm:LLM Records")}
        description={`${total.toLocaleString()} ${i18next.t("general:Records")}`}
        columns={columns}
        dataSource={records}
        rowKey={record => String(record.id)}
        loading={loading}
        emptyIcon={Logs}
        emptyText={modeOff ? i18next.t("llm:Recording is off detail") : i18next.t("llm:No records yet")}
        expandable={{
          expandedRowRender: record => <RecordDetail record={record} onDelete={() => deleteRecord(record.id)} />,
        }}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => load(nextPage, nextPageSize),
        }}
      />
    </PageContainer>
  );
}
