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
import {Activity, ArrowDownToLine, ArrowUpFromLine, CircleDollarSign, Database, Sparkles} from "lucide-react";
import i18next from "i18next";

import {cn} from "@/lib/utils";
import {Card, CardContent} from "@/components/ui/card";
import {Progress} from "@/components/ui/progress";
import {Skeleton} from "@/components/ui/skeleton";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cacheHitRate, formatCost, formatTokens, type UsageTotals} from "@/lib/usage";

/** One of the four token counts under the headline number. */
function MiniStat({
  icon: Icon,
  label,
  value,
  accent,
  hint,
}: {
  icon: React.ComponentType<{className?: string}>;
  label: React.ReactNode;
  value: React.ReactNode;
  accent: string;
  hint?: React.ReactNode;
}) {
  // SimpleTooltip renders its child alone when there is no title, so the hint
  // is optional without a branch here.
  return (
    <SimpleTooltip title={hint}>
      <div className="bg-background/60 flex flex-col gap-1 rounded-lg border p-3">
        <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
          <Icon className={cn("size-3.5 shrink-0", accent)} />
          <span className="truncate">{label}</span>
        </div>
        <span className="text-sm font-semibold tabular-nums">{value}</span>
      </div>
    </SimpleTooltip>
  );
}

/**
 * The window's headline: what it cost, in tokens and in dollars, with the four
 * counts those tokens are made of. Everything here is one window of one source,
 * so the totals and the tables below it always describe the same requests.
 */
export function UsageHero({
  totals,
  loading,
  windowLabel,
  cacheWriteHint,
}: {
  totals: UsageTotals;
  loading?: boolean;
  windowLabel: React.ReactNode;
  /** Set where the source cannot report cache writes, so a zero is explained. */
  cacheWriteHint?: React.ReactNode;
}) {
  const hitRate = cacheHitRate(totals);

  if (loading) {
    return (
      <Card>
        <CardContent className="flex flex-col gap-4">
          <Skeleton className="h-12 w-64" />
          <Skeleton className="h-16 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardContent className="flex flex-col gap-4">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div className="min-w-0">
            <div className="text-muted-foreground text-xs font-medium">
              {i18next.t("usage:Tokens spent")} · {windowLabel}
            </div>
            <div className="mt-1 flex flex-wrap items-baseline gap-2">
              <span className="text-3xl font-semibold tracking-tight tabular-nums">
                {totals.totalTokens.toLocaleString()}
              </span>
              <span className="text-muted-foreground bg-muted rounded-md px-1.5 py-0.5 text-xs">
                ≈ {formatTokens(totals.totalTokens)}
              </span>
            </div>
          </div>

          <div className="bg-background/60 flex items-center gap-5 rounded-lg border px-4 py-2.5">
            <div className="flex flex-col">
              <span className="text-muted-foreground text-[11px] font-medium">
                {i18next.t("llm:Requests")}
              </span>
              <span className="flex items-center gap-1.5 text-sm font-semibold tabular-nums">
                <Activity className="text-info size-3.5" />
                {totals.requests.toLocaleString()}
                {totals.failed ? (
                  <span className="text-warning text-xs font-normal">
                    {totals.failed.toLocaleString()} {i18next.t("llm:failed")}
                  </span>
                ) : null}
              </span>
            </div>
            <div className="bg-border h-8 w-px" />
            <div className="flex flex-col">
              <span className="text-muted-foreground text-[11px] font-medium">{i18next.t("llm:Cost")}</span>
              <span className="text-success flex items-center gap-1.5 text-sm font-semibold tabular-nums">
                <CircleDollarSign className="size-3.5" />
                {formatCost(totals.cost)}
              </span>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 lg:grid-cols-5">
          <MiniStat
            icon={ArrowDownToLine}
            label={i18next.t("llm:Fresh input")}
            value={formatTokens(totals.promptTokens)}
            accent="text-info"
          />
          <MiniStat
            icon={ArrowUpFromLine}
            label={i18next.t("llm:Output")}
            value={formatTokens(totals.completionTokens)}
            accent="text-chart-4"
          />
          <MiniStat
            icon={Database}
            label={i18next.t("llm:Cache write")}
            value={formatTokens(totals.cacheWriteTokens)}
            accent="text-warning"
            hint={cacheWriteHint}
          />
          <MiniStat
            icon={Sparkles}
            label={i18next.t("llm:Cache read")}
            value={formatTokens(totals.cacheReadTokens)}
            accent="text-success"
          />
          <div className="bg-background/60 col-span-2 flex flex-col justify-center gap-2 rounded-lg border p-3 lg:col-span-1">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground font-medium">{i18next.t("llm:Cache hit rate")}</span>
              <span className="text-success font-semibold tabular-nums">{hitRate.toFixed(hitRate >= 99.95 ? 0 : 1)}%</span>
            </div>
            <Progress value={hitRate} tone="success" className="h-1.5" />
          </div>
        </div>

        {totals.unpriced > 0 ? (
          <span className="text-muted-foreground text-xs">
            {i18next.t("llm:{count} records have no price").replace("{count}", totals.unpriced.toLocaleString())}
          </span>
        ) : null}
      </CardContent>
    </Card>
  );
}
