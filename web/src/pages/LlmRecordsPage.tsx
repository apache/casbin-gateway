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
import {Activity, CircleDollarSign, Database, Hash, Logs, Play, RefreshCw, Square, Trash2} from "lucide-react";
import i18next from "i18next";

import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as SettingBackend from "@/backend/SettingBackend";
import * as Setting from "@/Setting";
import {RequestInspector} from "@/components/llm/request-inspector";
import {DataTable, type Column} from "@/components/shared/data-table";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Loading} from "@/components/shared/loading";
import {CodeBlock, CodeText, DescriptionList, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {StatCard} from "@/components/shared/stat-card";
import {MessageAlert} from "@/components/ui/alert";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import {formatCost, formatTokens} from "@/lib/usage";
import type {Account, LlmPrice, LlmRecord, LlmRecordStats, LlmRecordStatus} from "@/types";

/** How often the totals are recomputed while the live feed is open. */
const STATS_INTERVAL = 10000;

function statusVariant(status: number): BadgeVariant {
  if (status >= 200 && status < 300) {
    return "success";
  }
  if (status >= 400 && status < 500) {
    return "warning";
  }
  return "danger";
}

/** Share of the input tokens served out of the prompt cache. */
function cacheHitRate(stats: LlmRecordStats) {
  const input = stats.promptTokens + stats.cacheReadTokens + stats.cacheWriteTokens;
  return input === 0 ? 0 : (stats.cacheReadTokens / input) * 100;
}

function tokenBreakdown(record: LlmRecord) {
  return [
    `${i18next.t("llm:Fresh input")}: ${record.promptTokens.toLocaleString()}`,
    `${i18next.t("llm:Cache read")}: ${record.cacheReadTokens.toLocaleString()}`,
    `${i18next.t("llm:Cache write")}: ${record.cacheWriteTokens.toLocaleString()}`,
    `${i18next.t("llm:Output")}: ${record.completionTokens.toLocaleString()}`,
  ].join("\n");
}

function isFailed(record: LlmRecord) {
  return record.status < 200 || record.status >= 300;
}

/** The upstream error is stored as it arrived, so JSON is laid out to be read. */
function formatErrorBody(body: string) {
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
}

/** What went wrong, which is the part a status code on its own never says. A
 *  request that failed over and then succeeded still lists what it failed over
 *  from. */
function RecordFailure({record, errorBody}: {record: LlmRecord; errorBody: string}) {
  return (
    <div className="flex flex-col gap-2">
      {isFailed(record) || record.error ? (
        <MessageAlert
          variant="destructive"
          title={i18next.t("llm:Request failed")}
          description={record.error || i18next.t("llm:No error message")}
        />
      ) : null}
      {record.failures?.length ? (
        <div className="flex flex-col gap-1">
          <span className="text-muted-foreground text-xs">{i18next.t("llm:Providers tried first")}</span>
          {record.failures.map((failure, index) => (
            <div key={index} className="flex min-w-0 flex-wrap items-center gap-2 text-xs">
              <Badge variant={statusVariant(failure.status)}>{failure.status || i18next.t("llm:No response")}</Badge>
              <CodeText>{failure.provider}</CodeText>
              <span className="text-muted-foreground min-w-0 break-all">{failure.error}</span>
            </div>
          ))}
        </div>
      ) : null}
      {errorBody ? (
        <div className="flex flex-col gap-1">
          <span className="text-muted-foreground text-xs">{i18next.t("llm:Upstream response")}</span>
          <CodeBlock copyable maxHeight="16rem">
            {formatErrorBody(errorBody)}
          </CodeBlock>
        </div>
      ) : null}
    </div>
  );
}

/** Mounted only once a row is expanded, which is when the body is fetched. */
function RecordDetail({record, onDelete}: {record: LlmRecord; onDelete: () => void}) {
  const [detail, setDetail] = React.useState<LlmRecord | null>(null);
  const [price, setPrice] = React.useState<LlmPrice | null>(null);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    LlmRecordBackend.getLlmRecord(record.id)
      .then(res => {
        if (res.status === "ok") {
          setDetail(res.data ?? null);
          setPrice(res.data2 ?? null);
        } else {
          setError(res.msg);
        }
      })
      .catch(err => setError(err.message || String(err)));
  }, [record.id]);

  const payload = detail?.payload ?? "";
  const rate = price
    ? `${formatCost(price.input)} / ${formatCost(price.output)} ${i18next.t("llm:per million")}`
    : i18next.t("llm:No price for this model");

  return (
    <div className="flex flex-col gap-4 px-4 py-3">
      <DescriptionList
        columns={3}
        items={[
          {label: i18next.t("general:ID"), value: record.id},
          {label: i18next.t("general:Created time"), value: Setting.getFormattedDate(record.createdTime)},
          {label: i18next.t("llm:Endpoint"), value: <CodeText>{`${record.protocol} ${record.endpoint}`}</CodeText>},
          record.provider && {label: i18next.t("llm:Provider"), value: <CodeText>{record.provider}</CodeText>},
          record.agent && {label: i18next.t("agent:Agent"), value: <CodeText>{record.agent}</CodeText>},
          {label: i18next.t("llm:Attempts"), value: record.attempts},
          {
            label: i18next.t("llm:Input tokens"),
            value: `${record.promptTokens.toLocaleString()} ${i18next.t("llm:fresh")}`,
          },
          {
            label: i18next.t("llm:Cached input"),
            value: `${record.cacheReadTokens.toLocaleString()} ${i18next.t("llm:read")} · ${record.cacheWriteTokens.toLocaleString()} ${i18next.t("llm:written")}`,
          },
          {
            label: i18next.t("llm:Output tokens"),
            value:
              record.reasoningTokens > 0
                ? `${record.completionTokens.toLocaleString()} · ${record.reasoningTokens.toLocaleString()} ${i18next.t("llm:reasoning")}`
                : record.completionTokens.toLocaleString(),
          },
          {
            label: i18next.t("llm:Cost"),
            value: record.priced ? `${formatCost(record.cost)} · ${rate}` : i18next.t("llm:No price for this model"),
          },
          {label: i18next.t("llm:Request size"), value: `${record.bytes.toLocaleString()} B`},
          {label: i18next.t("llm:Redacted values"), value: record.redactions},
        ]}
      />

      {isFailed(record) || record.error || record.failures?.length ? (
        <RecordFailure record={record} errorBody={detail?.errorBody ?? ""} />
      ) : null}

      {error ? <MessageAlert title={error} /> : null}
      {record.truncated ? <MessageAlert variant="warning" title={i18next.t("llm:Body was shortened")} /> : null}

      {payload ? <RequestInspector payload={payload} /> : null}
      {!payload && detail === null && error === "" ? <Loading type="small" /> : null}
      {!payload && detail !== null ? (
        <span className="text-muted-foreground text-xs">{i18next.t("llm:Body not stored")}</span>
      ) : null}

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
  const [changingMode, setChangingMode] = React.useState(false);
  const [stats, setStats] = React.useState<LlmRecordStats | null>(null);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(25);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [live, setLive] = React.useState(false);
  const [liveOpen, setLiveOpen] = React.useState(false);
  const [pending, setPending] = React.useState(0);

  const [outcome, setOutcome] = React.useState("");
  const [windowHours, setWindowHours] = React.useState("24");
  const [modelDraft, setModelDraft] = React.useState("");
  const [clientIpDraft, setClientIpDraft] = React.useState("");
  const [filter, setFilter] = React.useState<LlmRecordBackend.LlmRecordFilter>({});

  const activeFilter = React.useMemo(
    () => ({...filter, outcome: outcome, windowHours: Number(windowHours)}),
    [filter, outcome, windowHours],
  );

  const load = React.useCallback(
    (nextPage = page, nextPageSize = pageSize, foreground = true) => {
      if (!isAdmin) {
        return;
      }
      // A background reload keeps the loading state down, or it would cover the
      // rows being read.
      if (foreground) {
        setLoading(true);
      }

      LlmRecordBackend.getLlmRecords(nextPage, nextPageSize, activeFilter)
        .then(res => {
          if (res.status === "ok") {
            setRecords(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setPending(0);
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
    [activeFilter, isAdmin, page, pageSize],
  );

  const loadStats = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    LlmRecordBackend.getLlmRecordStats(activeFilter).then(res => {
      if (res.status === "ok") {
        setStats(res.data ?? null);
      }
    });
  }, [activeFilter, isAdmin]);

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

  // Recording is on by default, so this is mostly the way it gets turned down.
  // The Settings page holds the same field; changing it here is the shortcut,
  // and it applies to the next request rather than the next restart.
  const setRecordMode = (mode: string) => {
    setChangingMode(true);
    SettingBackend.getSetting()
      .then(res => {
        if (res.status !== "ok") {
          throw new Error(res.msg);
        }
        return SettingBackend.updateSetting({...res.data, llmRecordMode: mode as LlmRecordStatus["mode"]});
      })
      .then(res => {
        setChangingMode(false);
        if (res.status === "ok") {
          loadStatus();
          Setting.showMessage("success", mode === "off" ? i18next.t("llm:Recording is off") : i18next.t("llm:Recording is on"));
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(error => {
        setChangingMode(false);
        Setting.showMessage("error", `${error}`);
      });
  };

  React.useEffect(() => {
    load(1, pageSize);
    loadStats();
    loadStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeFilter]);

  // The feed carries every record Gateway writes, so the page drops the ones
  // its own filter would not have asked for.
  const matchesFilter = React.useCallback(
    (record: LlmRecord) => {
      if (activeFilter.model && !record.model.includes(activeFilter.model)) {
        return false;
      }
      if (activeFilter.clientIp && record.clientIp !== activeFilter.clientIp) {
        return false;
      }
      const succeeded = record.status >= 200 && record.status < 300;
      if (outcome === "ok" && !succeeded) {
        return false;
      }
      if (outcome === "error" && succeeded) {
        return false;
      }
      return true;
    },
    [activeFilter, outcome],
  );

  React.useEffect(() => {
    if (!live || !isAdmin) {
      setLiveOpen(false);
      return undefined;
    }

    const close = LlmRecordBackend.streamLlmRecords({
      onRecord: record => {
        if (!matchesFilter(record)) {
          return;
        }
        setTotal(current => current + 1);
        // A page further down counts them instead of reshuffling what is
        // being read.
        setPage(currentPage => {
          if (currentPage === 1) {
            setRecords(current => [record, ...current].slice(0, pageSize));
          } else {
            setPending(current => current + 1);
          }
          return currentPage;
        });
      },
      onOpen: () => setLiveOpen(true),
      onError: () => setLiveOpen(false),
    });
    const interval = setInterval(loadStats, STATS_INTERVAL);

    return () => {
      close();
      clearInterval(interval);
      setLiveOpen(false);
    };
  }, [isAdmin, live, loadStats, matchesFilter, pageSize]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const refresh = () => {
    load(page, pageSize);
    loadStats();
    loadStatus();
  };

  const deleteRecord = (id: number) => {
    LlmRecordBackend.deleteLlmRecord(id)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          refresh();
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
          loadStats();
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
          <span className="text-muted-foreground truncate text-xs">{record.agent || record.provider}</span>
        </div>
      ),
    },
    {
      title: i18next.t("general:Client ip"),
      key: "clientIp",
      dataIndex: "clientIp",
      width: "120px",
    },
    {
      title: i18next.t("llm:Status"),
      key: "status",
      width: "120px",
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
      width: "100px",
      render: (value: number) => `${value.toLocaleString()} ms`,
    },
    {
      title: i18next.t("llm:Tokens"),
      key: "totalTokens",
      width: "150px",
      render: (_value, record) =>
        record.totalTokens > 0 ? (
          <div className="flex min-w-0 flex-col gap-0.5" title={tokenBreakdown(record)}>
            <span>{record.totalTokens.toLocaleString()}</span>
            <span className="text-muted-foreground truncate text-xs">
              {formatTokens(record.promptTokens)} ↑ · {formatTokens(record.completionTokens)} ↓
              {record.cacheReadTokens > 0 ? ` · ${formatTokens(record.cacheReadTokens)} ⚡` : ""}
            </span>
          </div>
        ) : null,
    },
    {
      title: i18next.t("llm:Cost"),
      key: "cost",
      width: "90px",
      align: "right",
      render: (_value, record) =>
        record.priced ? (
          formatCost(record.cost)
        ) : (
          <span className="text-muted-foreground" title={i18next.t("llm:No price for this model")}>
            —
          </span>
        ),
    },
    {
      title: i18next.t("llm:Request"),
      key: "summary",
      render: (_value, record) => (
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="truncate text-xs" title={record.summary || record.error}>
            {record.summary || record.error}
          </span>
          {record.summary && record.error ? (
            <span className="text-destructive truncate text-xs" title={record.error}>
              {record.error}
            </span>
          ) : record.messageCount > 0 || record.toolCount > 0 ? (
            <span className="text-muted-foreground truncate text-xs">
              {record.messageCount} {i18next.t("llm:messages")} · {record.toolCount} {i18next.t("llm:tools")}
              {record.systemBytes > 0
                ? ` · ${i18next.t("llm:system")} ${(record.systemBytes / 1000).toFixed(1)}k`
                : ""}
            </span>
          ) : null}
        </div>
      ),
    },
  ];

  const modeOff = status !== null && status.mode === "off";
  const modeMetadata = status !== null && status.mode === "metadata";
  const modeFull = status !== null && status.mode === "full";
  const stopRecording = (
    <Button size="sm" variant="outline" onClick={() => setRecordMode("off")} loading={changingMode}>
      <Square />
      {i18next.t("llm:Recording off")}
    </Button>
  );
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
            {isAdmin && status ? (
              <SimpleSelect
                className="w-[220px]"
                value={status.mode}
                disabled={changingMode}
                onChange={setRecordMode}
                options={[
                  {label: i18next.t("llm:Recording off"), value: "off"},
                  {label: i18next.t("llm:Record metadata"), value: "metadata"},
                  {label: i18next.t("llm:Record metadata and bodies"), value: "full"},
                ]}
              />
            ) : null}
            <SimpleSelect
              className="w-[150px]"
              value={windowHours}
              onChange={setWindowHours}
              options={[
                {label: i18next.t("llm:Last hour"), value: "1"},
                {label: i18next.t("llm:Last 24 hours"), value: "24"},
                {label: i18next.t("llm:Last 7 days"), value: "168"},
                {label: i18next.t("llm:All time"), value: "0"},
              ]}
            />
            <Label className="text-sm font-normal">
              <Switch checked={live} onCheckedChange={setLive} />
              <span className="flex items-center gap-1.5">
                {i18next.t("llm:Live")}
                {live ? (
                  <span
                    className={liveOpen ? "bg-success size-2 animate-pulse rounded-full" : "bg-muted-foreground size-2 rounded-full"}
                    title={liveOpen ? i18next.t("llm:Feed connected") : i18next.t("llm:Feed connecting")}
                  />
                ) : null}
              </span>
            </Label>
            <Button variant="outline" onClick={refresh} loading={loading}>
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

      {modeOff ? (
        <MessageAlert
          variant="info"
          title={i18next.t("llm:Recording is off")}
          description={i18next.t("llm:Recording is off detail")}
          action={
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="outline" onClick={() => setRecordMode("metadata")} loading={changingMode}>
                <Play />
                {i18next.t("llm:Record metadata")}
              </Button>
              <Button size="sm" variant="outline" onClick={() => setRecordMode("full")} loading={changingMode}>
                <Play />
                {i18next.t("llm:Record metadata and bodies")}
              </Button>
            </div>
          }
        />
      ) : null}
      {modeMetadata ? (
        <MessageAlert
          variant="warning"
          title={i18next.t("llm:Bodies are not stored")}
          description={i18next.t("llm:Bodies are not stored detail")}
          action={
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="outline" onClick={() => setRecordMode("full")} loading={changingMode}>
                <Play />
                {i18next.t("llm:Record metadata and bodies")}
              </Button>
              {stopRecording}
            </div>
          }
        />
      ) : null}
      {modeFull ? (
        <MessageAlert
          variant="warning"
          title={i18next.t("llm:Bodies are stored")}
          description={i18next.t("llm:Bodies are stored detail")}
          action={
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="outline" onClick={() => setRecordMode("metadata")} loading={changingMode}>
                <Logs />
                {i18next.t("llm:Record metadata")}
              </Button>
              {stopRecording}
            </div>
          }
        />
      ) : null}
      {status && status.dropped > 0 ? (
        <MessageAlert
          variant="warning"
          title={i18next.t("llm:Records were dropped").replace("{count}", status.dropped.toLocaleString())}
        />
      ) : null}
      {error ? <MessageAlert title={error} /> : null}

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard
          label={i18next.t("llm:Requests")}
          value={(stats?.requests ?? 0).toLocaleString()}
          icon={Activity}
          hint={
            stats && stats.failed > 0
              ? `${stats.failed.toLocaleString()} ${i18next.t("llm:failed")}`
              : i18next.t("llm:all succeeded")
          }
          tone={stats && stats.failed > 0 ? "warning" : "default"}
        />
        <StatCard
          label={i18next.t("llm:Tokens")}
          value={formatTokens(stats?.totalTokens ?? 0)}
          icon={Hash}
          hint={`${formatTokens(stats?.promptTokens ?? 0)} ${i18next.t("llm:in")} · ${formatTokens(stats?.completionTokens ?? 0)} ${i18next.t("llm:out")}`}
        />
        <StatCard
          label={i18next.t("llm:Cache hit rate")}
          value={`${(stats ? cacheHitRate(stats) : 0).toFixed(0)}%`}
          icon={Database}
          percent={stats ? cacheHitRate(stats) : 0}
          hint={`${formatTokens(stats?.cacheReadTokens ?? 0)} ${i18next.t("llm:read")} · ${formatTokens(stats?.cacheWriteTokens ?? 0)} ${i18next.t("llm:written")}`}
        />
        <StatCard
          label={i18next.t("llm:Cost")}
          value={formatCost(stats?.cost ?? 0)}
          icon={CircleDollarSign}
          hint={
            stats && stats.unpriced > 0
              ? i18next.t("llm:{count} records have no price").replace("{count}", stats.unpriced.toLocaleString())
              : i18next.t("llm:List prices, see llmPricingFile")
          }
        />
      </div>

      {stats && stats.models.length > 1 ? (
        <div className="flex flex-wrap items-center gap-2">
          {stats.models.map(model => (
            <Badge key={model.model} variant="muted" className="gap-2">
              <span className="font-mono">{model.model}</span>
              <span>{model.requests.toLocaleString()}</span>
              <span className="text-muted-foreground">
                {formatTokens(model.tokens)} · {formatCost(model.cost)}
              </span>
            </Badge>
          ))}
        </div>
      ) : null}

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
        {pending > 0 ? (
          <Button variant="ghost" onClick={() => load(1, pageSize)}>
            {i18next.t("llm:{count} new records").replace("{count}", pending.toLocaleString())}
          </Button>
        ) : null}
      </form>

      <DataTable
        title={i18next.t("llm:LLM Records")}
        description={total === 1 ? `${total} ${i18next.t("general:Record")}` : `${total.toLocaleString()} ${i18next.t("general:Records")}`}
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
