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

import * as MiscBackend from "@/backend/MiscBackend";
import * as Setting from "@/Setting";
import {cn} from "@/lib/utils";
import {DataTable, type Column} from "@/components/DataTable";
import {BarChartCard, PieChartCard, StatisticCard} from "@/components/charts/ChartCards";
import type {MetricPoint} from "@/types";

const ranges = ["All", "Hour", "Day", "Week", "Month"] as const;
type Range = (typeof ranges)[number];

// How far back each range reaches, in units of the range itself.
function getRangeValue(rangeType: string) {
  switch (rangeType) {
  case "hour":
    return 72;
  case "day":
    return 7;
  case "week":
    return 12;
  case "month":
    return 12;
  default:
    return 7;
  }
}

function getGranularity(rangeType: string) {
  switch (rangeType) {
  case "hour":
    return "hour";
  case "day":
    return "hour";
  case "week":
    return "day";
  case "month":
    return "month";
  default:
    return "day";
  }
}

export default function DashboardPage() {
  const [rangeType, setRangeType] = React.useState<Range>("All");
  const [userAgents, setUserAgents] = React.useState<MetricPoint[]>([]);
  const [ipAddresses, setIpAddresses] = React.useState<MetricPoint[]>([]);
  const [sites, setSites] = React.useState<MetricPoint[]>([]);
  const [requestCountOverTime, setRequestCountOverTime] = React.useState<MetricPoint[]>([]);
  const [uniqueIpCount, setUniqueIpCount] = React.useState(0);
  const [totalRequestCount, setTotalRequestCount] = React.useState(0);

  const getAllData = React.useCallback((range: Range) => {
    const metricRange = range === "All" ? "month" : range.toLowerCase();
    const count = getRangeValue(metricRange);

    const loadMetric = (type: string, top?: number) =>
      MiscBackend.getMetric(type, metricRange, count, top).then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg);
          return null;
        }
        return res.data ?? [];
      });

    loadMetric("userAgent", 10).then(data => data && setUserAgents(data));
    loadMetric("ip").then(data => {
      if (!data) {
        return;
      }
      setIpAddresses(data.slice(0, 10));
      setUniqueIpCount(data.length);
    });
    loadMetric("site").then(data => data && setSites(data));

    const overTimeRange = range === "All" ? "week" : range.toLowerCase();
    MiscBackend.getMetricOverTime(
      overTimeRange,
      getRangeValue(overTimeRange),
      getGranularity(overTimeRange),
    ).then(res => {
      if (res.status !== "ok") {
        Setting.showMessage("error", res.msg);
        return;
      }
      setRequestCountOverTime(res.data ?? []);
      setTotalRequestCount(res.data2 ?? 0);
    });
  }, []);

  React.useEffect(() => {
    getAllData(rangeType);
  }, [getAllData, rangeType]);

  const countColumn: Column<MetricPoint> = {
    title: i18next.t("general:Count"),
    key: "count",
    dataIndex: "count",
    width: "100px",
    sorter: (a, b) => a.count - b.count,
  };

  return (
    <div className="space-y-4 p-4 md:p-6">
      <div className="flex justify-end">
        <div className="inline-flex rounded-md border p-0.5">
          {ranges.map(range => (
            <button
              key={range}
              onClick={() => setRangeType(range)}
              className={cn(
                "rounded px-3 py-1 text-sm transition-colors",
                rangeType === range
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-accent",
              )}
            >
              {i18next.t(`usage:${range}`)}
            </button>
          ))}
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-4">
        <StatisticCard title={i18next.t("general:Total Request Count")} value={totalRequestCount} />
        <div className="lg:col-span-3">
          <BarChartCard title="Request Count Over Time" data={requestCountOverTime} />
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-4">
        <div className="lg:col-span-3">
          <PieChartCard
            title="Sites"
            data={sites.map(item => ({name: item.data, value: item.count}))}
          />
        </div>
        <StatisticCard title={i18next.t("general:Unique IP Count")} value={uniqueIpCount} />
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <DataTable
          columns={[
            {
              title: i18next.t("general:IP Address"),
              key: "data",
              dataIndex: "data",
            },
            countColumn,
          ]}
          data={ipAddresses}
          rowKey={(record, index) => `${record.data}-${index}`}
          pageSize={0}
          title={i18next.t("general:Top 10 IP Addresses")}
        />
        <div className="lg:col-span-2">
          <DataTable
            columns={[
              {
                title: i18next.t("general:User-Agent"),
                key: "data",
                dataIndex: "data",
                render: (text: string) => (
                  <span className="block truncate text-xs" title={text}>
                    {text}
                  </span>
                ),
              },
              countColumn,
            ]}
            data={userAgents}
            rowKey={(record, index) => `${record.data}-${index}`}
            pageSize={0}
            title={i18next.t("general:Top 10 User-Agents")}
          />
        </div>
      </div>
    </div>
  );
}
