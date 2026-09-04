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
import {Bar, BarChart, XAxis, YAxis} from "recharts";
import i18next from "i18next";

import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {fillDays, formatCost, formatTokens, type UsagePoint} from "@/lib/usage";
import type {AgentUsageStat} from "@/types";

/** How far back a card's trend reaches. The server sends at most this many. */
export const trendDays = 30;

/** Whether an agent's transcripts hold enough of a recent history to draw. */
export function hasUsageTrend(stat?: AgentUsageStat) {
  return (stat?.days?.length ?? 0) > 0;
}

/** One number of the summary line: what it is, then what it reads. */
function Stat({
  label,
  value,
  hint,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  hint?: React.ReactNode;
}) {
  const shown = (
    <span className="min-w-0 truncate">
      <span className="text-muted-foreground text-[11px]">{label} </span>
      <span className="text-[13px] font-medium tabular-nums">{value}</span>
    </span>
  );

  return hint ? <SimpleTooltip title={hint}>{shown}</SimpleTooltip> : shown;
}

/** One line of the tooltip, which is a whole day rather than a single series. */
function TipRow({label, value}: {label: React.ReactNode; value: React.ReactNode}) {
  return (
    <div className="flex w-full items-center justify-between gap-3">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-mono tabular-nums">{value}</span>
    </div>
  );
}

/**
 * What one agent spent, as the shape of it: a bar per day over the last month,
 * with the totals it adds up to on either side. A card is too narrow for axes,
 * so the days are named in the tooltip and the window in the caption.
 */
export function AgentUsageTrend({
  stat,
  hints,
}: {
  stat: AgentUsageStat;
  /** The same notes the totals carry elsewhere: where they were read from,
   *  and how much of the cost is missing a list price. */
  hints?: {tokens?: React.ReactNode; cost?: React.ReactNode; requests?: React.ReactNode};
}) {
  const config: ChartConfig = {
    tokens: {label: i18next.t("llm:Tokens"), color: "var(--chart-1)"},
  };
  // The quiet days are filled in here rather than by the server: a week nobody
  // worked is a gap in the bars, not a chart that is a week shorter.
  const points: UsagePoint[] = fillDays(
    (stat.days ?? []).map(day => ({
      bucket: day.name,
      requests: day.requests,
      promptTokens: day.promptTokens,
      completionTokens: day.completionTokens,
      cacheReadTokens: day.cacheReadTokens,
      cacheWriteTokens: day.cacheWriteTokens,
      cost: day.cost,
    })),
    trendDays,
  );
  const data = points.map(point => ({
    ...point,
    tokens:
      point.promptTokens + point.completionTokens + point.cacheReadTokens + point.cacheWriteTokens,
  }));
  const window = i18next.t("usage:Last {count} days").replace("{count}", String(trendDays));

  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between gap-2">
        <Stat
          label={i18next.t("llm:Tokens")}
          value={formatTokens(stat.totalTokens)}
          hint={hints?.tokens}
        />
        <Stat label={i18next.t("llm:Cost")} value={formatCost(stat.cost)} hint={hints?.cost} />
      </div>

      <ChartContainer config={config} className="aspect-auto h-[52px] w-full">
        <BarChart data={data} margin={{top: 2, right: 0, left: 0, bottom: 0}}>
          <XAxis dataKey="bucket" hide />
          <YAxis hide domain={[0, "dataMax"]} />
          <ChartTooltip
            cursor={false}
            content={
              <ChartTooltipContent
                hideIndicator
                labelFormatter={(_, payload) => String(payload?.[0]?.payload?.bucket ?? "")}
                formatter={(_value, _name, item) => {
                  const day = item.payload as (typeof data)[number];
                  return (
                    <div className="grid w-full gap-1">
                      <TipRow
                        label={i18next.t("llm:Requests")}
                        value={day.requests.toLocaleString()}
                      />
                      <TipRow label={i18next.t("llm:Tokens")} value={formatTokens(day.tokens)} />
                      <TipRow label={i18next.t("llm:Cost")} value={formatCost(day.cost)} />
                    </div>
                  );
                }}
              />
            }
          />
          {/* Redrawn every time the totals are polled, so it does not animate:
              the bars would slide up from zero once a minute. */}
          <Bar dataKey="tokens" fill="var(--color-tokens)" radius={1} isAnimationActive={false} />
        </BarChart>
      </ChartContainer>

      <div className="text-muted-foreground flex items-center justify-between gap-2 text-[11px]">
        <Stat
          label={i18next.t("llm:Requests")}
          value={stat.requests.toLocaleString()}
          hint={hints?.requests}
        />
        <span className="shrink-0">{window}</span>
      </div>
    </div>
  );
}
