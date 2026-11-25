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
import {CalendarDays} from "lucide-react";
import i18next from "i18next";

import {EmptyState} from "@/components/shared/empty-state";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {Skeleton} from "@/components/ui/skeleton";
import {formatCost, formatTokens, type UsagePoint} from "@/lib/usage";
import {cn} from "@/lib/utils";

type Metric = "requests" | "totalTokens" | "cost";

const metrics = {requests: "llm:Requests", totalTokens: "llm:Tokens", cost: "llm:Cost"} as const;
const shades = ["bg-muted", "bg-primary/10", "bg-primary/20", "bg-primary/30", "bg-primary/40"];

function daysOf(points: UsagePoint[]) {
  const days = new Map<string, Record<Metric, number>>();
  for (const point of points) {
    // Hourly relay buckets carry the server's calendar date in their prefix.
    const key = point.bucket.slice(0, 10);
    const day = days.get(key) ?? {requests: 0, totalTokens: 0, cost: 0};
    day.requests += point.requests;
    day.totalTokens += point.totalTokens;
    day.cost += point.cost;
    days.set(key, day);
  }
  return days;
}

export function UsageCalendar({points, loading}: {points: UsagePoint[]; loading: boolean}) {
  const [chosenMonth, setMonth] = React.useState("");
  const [chosenDay, setDay] = React.useState("");
  const [metric, setMetric] = React.useState<Metric>("requests");
  const days = React.useMemo(() => daysOf(points), [points]);
  const months = Array.from(new Set(Array.from(days.keys(), day => day.slice(0, 7)))).sort().reverse();
  const month = months.includes(chosenMonth) ? chosenMonth : months[0];
  const active = chosenDay.startsWith(month + "-") ? days.get(chosenDay) : undefined;
  const locale = i18next.language;
  const monthFormat = new Intl.DateTimeFormat(locale, {year: "numeric", month: "long", timeZone: "UTC"});
  const weekdayFormat = new Intl.DateTimeFormat(locale, {weekday: "short", timeZone: "UTC"});
  const start = month ? new Date(month + "-01T00:00:00Z") : undefined;
  const length = start ? new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth() + 1, 0)).getUTCDate() : 0;
  const dates = Array.from({length}, (_, index) => month + "-" + String(index + 1).padStart(2, "0"));
  const maximum = Math.max(0, ...dates.map(date => days.get(date)?.[metric] ?? 0));

  return (
    <Card>
      <CardHeader className="flex flex-wrap items-center justify-between gap-2">
        <CardTitle className="text-[15px]">{i18next.t("usage:Daily activity")}</CardTitle>
        {month && !loading ? (
          <div className="flex flex-wrap gap-2">
            <SimpleSelect
              aria-label={i18next.t("usage:Month")} className="w-auto" value={month} onChange={setMonth}
              options={months.map(value => ({value, label: monthFormat.format(new Date(value + "-01T00:00:00Z"))}))}
            />
            <SimpleSelect
              aria-label={i18next.t("usage:Metric")} className="w-auto" value={metric} onChange={value => setMetric(value as Metric)}
              options={Object.entries(metrics).map(([value, key]) => ({value, label: i18next.t(key)}))}
            />
          </div>
        ) : null}
      </CardHeader>
      <CardContent>
        {loading ? <Skeleton className="h-52 w-full" /> : !month ? (
          <EmptyState icon={CalendarDays} title={i18next.t("usage:Nothing was spent in this window")} />
        ) : (
          <>
            <div className="grid grid-cols-7 gap-1 text-xs">
              {Array.from({length: 7}, (_, index) => (
                <span className="text-muted-foreground text-center" key={index}>
                  {weekdayFormat.format(Date.UTC(2024, 0, 1 + index))}
                </span>
              ))}
              {dates.map((date, index) => {
                const day = days.get(date);
                const style = index === 0 && start ? {gridColumnStart: (start.getUTCDay() + 6) % 7 + 1} : undefined;
                if (!day) {
                  return <span key={date} style={style} className="p-2 text-muted-foreground">{index + 1}</span>;
                }
                const value = day[metric];
                const shade = maximum > 0 ? Math.ceil(4 * value / maximum) : 0;
                const exact = metric === "cost" ? formatCost(value) : value.toLocaleString(locale);
                return (
                  <button
                    type="button" key={date} style={style} aria-pressed={chosenDay === date}
                    aria-label={`${date} · ${i18next.t(metrics[metric])} ${exact}`}
                    onClick={() => setDay(date)}
                    className={cn("flex flex-col rounded p-1.5 text-start tabular-nums hover:ring-1 focus-visible:ring-2 sm:p-2", shades[shade], chosenDay === date && "ring-1")}
                  >
                    <span>{index + 1}</span>
                    <span className="max-w-full truncate font-medium">{metric === "cost" ? formatCost(value) : formatTokens(value)}</span>
                  </button>
                );
              })}
            </div>
            {active ? (
              <p aria-live="polite" className="mt-3 text-sm tabular-nums">
                {chosenDay} · {i18next.t(metrics.requests)} {active.requests.toLocaleString(locale)} · {i18next.t(metrics.totalTokens)} {active.totalTokens.toLocaleString(locale)} · {i18next.t(metrics.cost)} {formatCost(active.cost)}
              </p>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  );
}
