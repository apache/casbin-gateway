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
import {Link, useSearchParams} from "react-router-dom";
import {Logs, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as Setting from "@/Setting";
import {UsageBreakdown} from "@/components/usage/usage-breakdown";
import {UsageHero} from "@/components/usage/usage-hero";
import {UsageTrendChart} from "@/components/usage/usage-trend-chart";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {UnauthorizedResult} from "@/components/shared/misc";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {
  emptyUsageTotals,
  fillDays,
  type UsagePoint,
  type UsageRow,
  type UsageTotals,
} from "@/lib/usage";
import type {Account} from "@/types";

/**
 * The two accounts of what the agents cost, which do not measure the same
 * thing: "agents" is what they wrote in their own transcripts, and is the only
 * record of a request that never went through Gateway; "relayed" is what
 * Gateway itself passed on, and is the only one that knows which provider
 * answered and whether it failed.
 */
type UsageSource = "agents" | "relayed";

/** The windows each source offers, in days. 0 is everything it has. */
const RANGES: Record<UsageSource, number[]> = {
  agents: [7, 30, 90, 0],
  relayed: [1, 7, 30, 0],
};

const REFRESH_OPTIONS = [0, 10000, 30000, 60000];

/** One window's worth of either source, normalised so the page draws one way. */
interface UsageView {
  totals: UsageTotals;
  points: UsagePoint[];
  models: UsageRow[];
  providers: UsageRow[];
  agents: UsageRow[];
}

const emptyView: UsageView = {
  totals: emptyUsageTotals,
  points: [],
  models: [],
  providers: [],
  agents: [],
};

function rangeLabel(days: number) {
  if (days === 0) {
    return i18next.t("llm:All time");
  }
  if (days === 1) {
    return i18next.t("llm:Last 24 hours");
  }
  return i18next.t("usage:Last {count} days").replace("{count}", String(days));
}

export default function UsagePage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  // The tab lives in the URL so the rail can link straight to one of them.
  const [searchParams, setSearchParams] = useSearchParams();
  const source: UsageSource = searchParams.get("tab") === "relayed" ? "relayed" : "agents";
  const [days, setDays] = React.useState(7);
  const [agent, setAgent] = React.useState("");
  const [refreshMs, setRefreshMs] = React.useState(30000);
  const [view, setView] = React.useState<UsageView>(emptyView);
  // False until the first answer lands, so an empty window never flashes as
  // "nothing was spent" before anything has been counted.
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [recording, setRecording] = React.useState(true);
  // Every agent seen in either source, so the filter keeps its options when the
  // window is narrowed down to one that spent nothing.
  const [knownAgents, setKnownAgents] = React.useState<string[]>([]);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    LlmRecordBackend.getLlmRecordStatus()
      .then(res => setRecording(res.status !== "ok" || (res.data?.mode ?? "off") !== "off"))
      .catch(() => undefined);
  }, [isAdmin]);

  const loadAgents = React.useCallback(async(): Promise<UsageView> => {
    const res = await AgentBackend.getAgentUsage(agent, days);
    if (res.status !== "ok" || !res.data) {
      throw new Error(res.msg || i18next.t("general:Failed to get data"));
    }

    const usage = res.data;
    const rowOf = (stat: (typeof usage.agents)[number], detail?: string): UsageRow => ({
      name: stat.name,
      requests: stat.requests,
      tokens: stat.totalTokens,
      cost: stat.cost,
      detail: detail,
    });

    return {
      totals: {...usage.totals, failed: undefined},
      points: fillDays(
        usage.days.map(day => ({
          bucket: day.name,
          requests: day.requests,
          promptTokens: day.promptTokens,
          completionTokens: day.completionTokens,
          cacheReadTokens: day.cacheReadTokens,
          cacheWriteTokens: day.cacheWriteTokens,
          cost: day.cost,
        })),
        days,
      ),
      models: usage.models.map(model => rowOf(model)),
      providers: [],
      agents: usage.agents.map(stat => rowOf(stat, stat.lastModel)),
    };
  }, [agent, days]);

  const loadRelayed = React.useCallback(async(): Promise<UsageView> => {
    const filter = {agent: agent, windowHours: days * 24, top: 100};
    const [statsRes, trendRes, agentsRes] = await Promise.all([
      LlmRecordBackend.getLlmRecordStats(filter),
      LlmRecordBackend.getLlmUsageTrend(filter, days === 1 ? "hour" : "day"),
      LlmRecordBackend.getLlmAgentStats(filter),
    ]);
    if (statsRes.status !== "ok" || !statsRes.data) {
      throw new Error(statsRes.msg || i18next.t("general:Failed to get data"));
    }

    const stats = statsRes.data;
    return {
      totals: {
        requests: stats.requests,
        failed: stats.failed,
        promptTokens: stats.promptTokens,
        completionTokens: stats.completionTokens,
        cacheReadTokens: stats.cacheReadTokens,
        cacheWriteTokens: stats.cacheWriteTokens,
        totalTokens: stats.totalTokens,
        cost: stats.cost,
        unpriced: stats.unpriced,
      },
      points: (trendRes.status === "ok" ? (trendRes.data ?? []) : []).map(point => ({
        bucket: point.bucket,
        requests: point.requests,
        promptTokens: point.promptTokens,
        completionTokens: point.completionTokens,
        cacheReadTokens: point.cacheReadTokens,
        cacheWriteTokens: point.cacheWriteTokens,
        cost: point.cost,
      })),
      models: stats.models.map(model => ({
        name: model.model,
        requests: model.requests,
        tokens: model.tokens,
        cost: model.cost,
      })),
      providers: stats.providers.map(provider => ({
        name: provider.provider,
        requests: provider.requests,
        failed: provider.failed,
        tokens: provider.tokens,
        cost: provider.cost,
      })),
      agents: (agentsRes.status === "ok" ? (agentsRes.data ?? []) : []).map(stat => ({
        name: stat.agent,
        requests: stat.requests,
        failed: stat.failed,
        tokens: stat.tokens,
        cost: stat.cost,
        detail: stat.lastModel,
      })),
    };
  }, [agent, days]);

  const load = React.useCallback(
    (spinner: boolean) => {
      if (!isAdmin) {
        return;
      }
      if (spinner) {
        setLoading(true);
      }
      (source === "agents" ? loadAgents() : loadRelayed())
        .then(next => {
          setView(next);
          setError("");
          // The agents a window happens to cover are the only names there are
          // to filter by, so the list only ever grows.
          setKnownAgents(known => {
            const merged = new Set(known);
            next.agents.forEach(row => merged.add(row.name));
            return [...merged].sort();
          });
        })
        .catch(failure => {
          setError(failure.message || String(failure));
          setView(emptyView);
        })
        .then(() => setLoading(false));
    },
    [isAdmin, source, loadAgents, loadRelayed],
  );

  React.useEffect(() => {
    load(true);
  }, [load]);

  React.useEffect(() => {
    if (refreshMs === 0) {
      return undefined;
    }
    // Refreshed without the spinner, so a page left open does not blink every
    // time it catches up.
    const interval = setInterval(() => load(false), refreshMs);
    return () => clearInterval(interval);
  }, [refreshMs, load]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const changeSource = (next: UsageSource) => {
    setSearchParams(
      previous => {
        previous.set("tab", next);
        return previous;
      },
      {replace: true},
    );
    // Each source offers the windows its own granularity can draw, so a window
    // the new one does not have falls back to its default.
    if (!RANGES[next].includes(days)) {
      setDays(7);
    }
  };

  const window = rangeLabel(days);
  const relayed = source === "relayed";
  // A provider that speaks the OpenAI protocol reports cache hits but never
  // cache writes, so a zero there is the protocol, not the truth.
  const cacheWriteHint =
    relayed && view.totals.cacheWriteTokens === 0 && view.totals.cacheReadTokens > 0
      ? i18next.t("usage:Cache writes are not reported by every protocol")
      : undefined;

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("usage:Usage")}
        description={i18next.t(relayed ? "usage:Relayed description" : "usage:Agents description")}
        actions={
          <>
            <SimpleSelect
              className="w-[150px]"
              value={agent}
              onChange={setAgent}
              options={[
                {label: i18next.t("usage:Every agent"), value: ""},
                ...knownAgents.map(name => ({label: name, value: name})),
              ]}
            />
            <SimpleSelect
              className="w-[150px]"
              value={String(days)}
              onChange={value => setDays(Number(value))}
              options={RANGES[source].map(option => ({label: rangeLabel(option), value: String(option)}))}
            />
            <SimpleSelect
              className="w-[150px]"
              value={String(refreshMs)}
              onChange={value => setRefreshMs(Number(value))}
              options={REFRESH_OPTIONS.map(option => ({
                label:
                  option === 0
                    ? i18next.t("usage:No auto refresh")
                    : i18next.t("usage:Every {count}s").replace("{count}", String(option / 1000)),
                value: String(option),
              }))}
            />
            <Button variant="outline" onClick={() => load(true)} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      <Tabs value={source} onValueChange={value => changeSource(value as UsageSource)}>
        <TabsList>
          <TabsTrigger value="agents">{i18next.t("usage:Agent spend")}</TabsTrigger>
          <TabsTrigger value="relayed">{i18next.t("usage:Relayed spend")}</TabsTrigger>
        </TabsList>
      </Tabs>

      {error ? <MessageAlert title={error} /> : null}

      {relayed && !recording ? (
        <MessageAlert
          variant="warning"
          title={i18next.t("llm:Recording is off")}
          description={i18next.t("usage:Recording is off detail")}
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/llm-records">
                <Logs />
                {i18next.t("llm:LLM Records")}
              </Link>
            </Button>
          }
        />
      ) : null}

      <UsageHero
        totals={view.totals}
        loading={loading}
        windowLabel={window}
        cacheWriteHint={cacheWriteHint}
      />

      <UsageTrendChart points={view.points} loading={loading} description={window} />

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <UsageBreakdown
          title={i18next.t("usage:By model")}
          nameLabel={i18next.t("agent:Model")}
          rows={view.models}
          loading={loading}
          emptyText={i18next.t("usage:Nothing was spent in this window")}
        />
        <UsageBreakdown
          title={i18next.t("usage:By agent")}
          nameLabel={i18next.t("agent:Agent")}
          rows={view.agents}
          loading={loading}
          showFailed={relayed}
          emptyText={i18next.t("usage:Nothing was spent in this window")}
        />
        {relayed ? (
          <UsageBreakdown
            title={i18next.t("usage:By provider")}
            nameLabel={i18next.t("provider:Provider")}
            rows={view.providers}
            loading={loading}
            showFailed
            emptyText={i18next.t("usage:Nothing was spent in this window")}
          />
        ) : null}
      </div>
    </PageContainer>
  );
}
