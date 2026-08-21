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
import i18next from "i18next";

import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import {BarChartCard, PieChartCard, StatisticCard} from "@/components/charts/ChartCards";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {LlmUsage, MetricPoint} from "@/types";

// ranges maps a picker option to the (rangeType, count, granularity) triple the
// metrics API already speaks, so the window and its bucket size stay in step.
const ranges: Record<string, {rangeType: string; count: number; granularity: string}> = {
  "24h": {rangeType: "hour", count: 24, granularity: "hour"},
  "7d": {rangeType: "day", count: 7, granularity: "day"},
  "30d": {rangeType: "day", count: 30, granularity: "day"},
};

function toPie(points: MetricPoint[] | undefined) {
  return (points ?? []).map(point => ({name: point.data || "-", value: point.count}));
}

// LlmUsagePanel shows aggregated token usage over a selectable window: headline
// totals, tokens over time, and where the tokens go by model, channel and agent.
export function LlmUsagePanel() {
  const [range, setRange] = React.useState("7d");
  const [usage, setUsage] = React.useState<LlmUsage | null>(null);

  React.useEffect(() => {
    const selected = ranges[range];
    LlmRecordBackend.getLlmUsage(selected.rangeType, selected.count, selected.granularity).then(res => {
      if (res.status === "ok") {
        setUsage(res.data);
      }
    });
  }, [range]);

  const totals = usage?.totals;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-medium text-muted-foreground">{i18next.t("llm:Usage")}</h2>
        <Select value={range} onValueChange={setRange}>
          <SelectTrigger className="h-8 w-36">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="24h">{i18next.t("llm:Last 24 hours")}</SelectItem>
            <SelectItem value="7d">{i18next.t("llm:Last 7 days")}</SelectItem>
            <SelectItem value="30d">{i18next.t("llm:Last 30 days")}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatisticCard title={i18next.t("llm:Requests")} value={totals?.requests ?? 0} />
        <StatisticCard title={i18next.t("llm:Total tokens")} value={totals?.totalTokens ?? 0} />
        <StatisticCard title={i18next.t("llm:Prompt tokens")} value={totals?.promptTokens ?? 0} />
        <StatisticCard title={i18next.t("llm:Completion tokens")} value={totals?.completionTokens ?? 0} />
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <BarChartCard title="Tokens Over Time" data={usage?.overTime ?? null} />
        <PieChartCard title="Tokens by Model" data={toPie(usage?.byModel)} />
        <PieChartCard title="Tokens by Channel" data={toPie(usage?.byChannel)} />
        <PieChartCard title="Tokens by Agent" data={toPie(usage?.byAgent)} />
      </div>
    </div>
  );
}
