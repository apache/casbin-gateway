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
import {Area, CartesianGrid, ComposedChart, Line, XAxis, YAxis} from "recharts";
import {TrendingUp} from "lucide-react";
import i18next from "i18next";

import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Skeleton} from "@/components/ui/skeleton";
import {EmptyState} from "@/components/shared/empty-state";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {bucketLabel, formatCost, formatTokens, type UsagePoint} from "@/lib/usage";

/** The series, in the order they stack. Built per render so a language switch
 *  relabels the legend without a reload. */
function buildConfig(): ChartConfig {
  return {
    promptTokens: {label: i18next.t("llm:Fresh input"), color: "var(--chart-1)"},
    completionTokens: {label: i18next.t("llm:Output"), color: "var(--chart-2)"},
    cacheWriteTokens: {label: i18next.t("llm:Cache write"), color: "var(--chart-3)"},
    cacheReadTokens: {label: i18next.t("llm:Cache read"), color: "var(--chart-4)"},
    cost: {label: i18next.t("llm:Cost"), color: "var(--chart-5)"},
  };
}

/**
 * When the window was spent, which is the one thing its totals cannot say. The
 * token counts stack, because together they are the total; the cost rides a
 * second axis as a line, because dollars and tokens share no scale and one
 * axis would flatten whichever of them is smaller.
 */
export function UsageTrendChart({
  points,
  loading,
  description,
}: {
  points: UsagePoint[];
  loading?: boolean;
  description?: React.ReactNode;
}) {
  const config = buildConfig();
  // Labels are cut out of the bucket key rather than parsed as a date: the
  // server cut that key from a timestamp in its own zone, and reading it as a
  // Date here would move every point into the browser's.
  const data = points.map(point => ({...point, label: bucketLabel(point.bucket)}));
  // A tick per bucket is unreadable past a few weeks, so only some are drawn.
  const tickInterval = Math.max(0, Math.ceil(data.length / 12) - 1);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-[15px]">{i18next.t("usage:Trend")}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-[300px] w-full" />
        ) : data.length === 0 ? (
          <EmptyState icon={TrendingUp} title={i18next.t("usage:Nothing was spent in this window")} />
        ) : (
          <ChartContainer config={config} className="aspect-auto h-[300px] w-full">
            <ComposedChart data={data} margin={{top: 8, right: 8, left: 0, bottom: 0}}>
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="label"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                interval={tickInterval}
                minTickGap={16}
              />
              <YAxis
                yAxisId="tokens"
                tickLine={false}
                axisLine={false}
                width={52}
                tickFormatter={value => formatTokens(Number(value))}
              />
              <YAxis
                yAxisId="cost"
                orientation="right"
                tickLine={false}
                axisLine={false}
                width={52}
                tickFormatter={value => formatCost(Number(value))}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(value, name) => (
                      <div className="flex w-full items-center justify-between gap-3">
                        <span className="text-muted-foreground">{config[name as string]?.label ?? name}</span>
                        <span className="font-mono tabular-nums">
                          {name === "cost" ? formatCost(Number(value)) : Number(value).toLocaleString()}
                        </span>
                      </div>
                    )}
                  />
                }
              />
              <ChartLegend content={<ChartLegendContent />} />
              {/* Straight segments, not a curve: these are discrete totals for a
                  day or an hour, and a spline between them would draw a bulge
                  over the neighbours that were spent on. */}
              <Area
                yAxisId="tokens"
                stackId="tokens"
                type="linear"
                dataKey="promptTokens"
                stroke="var(--color-promptTokens)"
                fill="var(--color-promptTokens)"
                fillOpacity={0.2}
              />
              <Area
                yAxisId="tokens"
                stackId="tokens"
                type="linear"
                dataKey="completionTokens"
                stroke="var(--color-completionTokens)"
                fill="var(--color-completionTokens)"
                fillOpacity={0.2}
              />
              <Area
                yAxisId="tokens"
                stackId="tokens"
                type="linear"
                dataKey="cacheWriteTokens"
                stroke="var(--color-cacheWriteTokens)"
                fill="var(--color-cacheWriteTokens)"
                fillOpacity={0.2}
              />
              <Area
                yAxisId="tokens"
                stackId="tokens"
                type="linear"
                dataKey="cacheReadTokens"
                stroke="var(--color-cacheReadTokens)"
                fill="var(--color-cacheReadTokens)"
                fillOpacity={0.2}
              />
              <Line
                yAxisId="cost"
                type="linear"
                dataKey="cost"
                stroke="var(--color-cost)"
                strokeWidth={2}
                strokeDasharray="4 4"
                dot={false}
              />
            </ComposedChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
