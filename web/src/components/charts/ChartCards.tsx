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
import {useTranslation} from "react-i18next";
import {Bar, BarChart, CartesianGrid, Cell, Pie, PieChart, XAxis, YAxis} from "recharts";

import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import type {MetricPoint} from "@/types";

// The five categorical slots defined in index.css, used in this fixed order and
// never cycled: anything past the fifth slice folds into "Other".
const SERIES_COLORS = [
  "hsl(var(--chart-1))",
  "hsl(var(--chart-2))",
  "hsl(var(--chart-3))",
  "hsl(var(--chart-4))",
  "hsl(var(--chart-5))",
];
const OTHER_COLOR = "hsl(var(--muted-foreground))";

export function BarChartCard({title, data}: {title: string; data: MetricPoint[] | null}) {
  const points = data ?? [];
  const config = {
    count: {
      label: i18next.t("general:Count"),
      color: "hsl(var(--chart-1))",
    },
  } satisfies ChartConfig;

  return (
    <Card className="h-full">
      <CardHeader className="p-4 pb-0">
        <CardTitle className="text-sm">{i18next.t(`general:${title}`)}</CardTitle>
      </CardHeader>
      <CardContent className="p-2">
        <ChartContainer config={config} className="aspect-auto h-[300px] w-full">
          <BarChart accessibilityLayer data={points} margin={{top: 8, right: 8, left: 0}}>
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="data"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={16}
            />
            <YAxis tickLine={false} axisLine={false} tickMargin={8} width={48} />
            <ChartTooltip cursor={false} content={<ChartTooltipContent />} />
            <Bar dataKey="count" fill="var(--color-count)" radius={[4, 4, 0, 0]} maxBarSize={48} />
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

export function PieChartCard({
  title,
  data,
}: {
  title: string;
  data: {name: string; value: number}[] | null;
}) {
  // Subscribe to language changes so the memoised "Other" label below
  // re-translates in place on a switch (changeLanguage no longer reloads).
  const {t} = useTranslation();
  // Rank the slices, keep the five that get their own color, and sum the tail
  // into a single neutral "Other" slice so no color is ever reused.
  const {slices, config} = React.useMemo(() => {
    const sorted = [...(data ?? [])].sort((a, b) => b.value - a.value);
    const head = sorted.slice(0, SERIES_COLORS.length).map((item, index) => ({
      ...item,
      fill: SERIES_COLORS[index],
    }));
    const tail = sorted.slice(SERIES_COLORS.length);

    if (tail.length > 0) {
      head.push({
        name: t("general:Other"),
        value: tail.reduce((sum, item) => sum + item.value, 0),
        fill: OTHER_COLOR,
      });
    }

    const chartConfig: ChartConfig = {};
    head.forEach(item => {
      chartConfig[item.name] = {label: item.name};
    });

    return {slices: head, config: chartConfig};
  }, [data, t]);

  return (
    <Card className="h-full">
      <CardHeader className="p-4 pb-0">
        <CardTitle className="text-sm">{i18next.t(`general:${title}`)}</CardTitle>
      </CardHeader>
      <CardContent className="p-2">
        <ChartContainer config={config} className="aspect-auto h-[300px] w-full">
          <PieChart>
            <ChartTooltip content={<ChartTooltipContent nameKey="name" hideLabel />} />
            <Pie data={slices} dataKey="value" nameKey="name" innerRadius="45%" outerRadius="70%">
              {slices.map(item => (
                // A 2px surface-colored ring keeps neighbouring slices apart.
                <Cell
                  key={item.name}
                  fill={item.fill}
                  stroke="hsl(var(--card))"
                  strokeWidth={2}
                />
              ))}
            </Pie>
            <ChartLegend content={<ChartLegendContent nameKey="name" className="flex-wrap" />} />
          </PieChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}

/**
 * StatisticCard shows one big number. The antd build drew it with an echarts
 * scatter plot; plain markup renders the same thing without a chart runtime.
 */
export function StatisticCard({title, value}: {title: string; value: number}) {
  return (
    <Card className="flex h-full flex-col">
      <CardHeader className="p-4 pb-0">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-1 items-center justify-center p-4">
        <span className="text-4xl font-semibold tabular-nums">{value.toLocaleString()}</span>
      </CardContent>
    </Card>
  );
}
