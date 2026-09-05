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
import {Link} from "react-router-dom";
import {ChevronRight} from "lucide-react";
import i18next from "i18next";

import {authenticityOf, useProviderProbes} from "@/components/AuthenticityOverview";
import {Card} from "@/components/ui/card";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {formatScore, gradeLetter, gradeStyleOf} from "@/lib/authenticity";
import {cn} from "@/lib/utils";
import {formatCost} from "@/lib/usage";
import type {AgentSpend} from "@/lib/agents";
import type {Provider} from "@/types";

/** Fills the {name} placeholders these strings carry. */
function fill(key: string, values: Record<string, string | number>) {
  return Object.entries(values).reduce(
    (text, [name, value]) => text.split(`{${name}}`).join(String(value)),
    i18next.t(key),
  );
}

interface Cell {
  key: string;
  label: React.ReactNode;
  value: React.ReactNode;
  note?: React.ReactNode;
  tone?: string;
  to?: string;
  hint?: React.ReactNode;
}

/** One figure of the strip: what it reads, then what it counts. */
function Figure({cell, className}: {cell: Cell; className?: string}) {
  const body = (
    <>
      <span
        className={cn(
          "flex items-baseline gap-1 truncate text-xl font-semibold tabular-nums",
          cell.tone,
        )}
      >
        {cell.value}
        {cell.to ? <ChevronRight className="text-muted-foreground size-4 shrink-0 self-center" /> : null}
      </span>
      <span className="text-muted-foreground truncate text-xs">
        {cell.label}
        {cell.note ? <span className="opacity-70"> · {cell.note}</span> : null}
      </span>
    </>
  );

  const inner = cell.to ? (
    <Link to={cell.to} className="flex min-w-0 flex-col gap-0.5">
      {body}
    </Link>
  ) : (
    <div className="flex min-w-0 flex-col gap-0.5">{body}</div>
  );

  return (
    <div className={cn("min-w-0 px-4 py-3", cell.to && "hover:bg-accent/40 transition-colors", className)}>
      {cell.hint ? <SimpleTooltip title={cell.hint}>{inner}</SimpleTooltip> : inner}
    </div>
  );
}

/**
 * The whole machine in one line: what the agents on it have spent, how many of
 * them there are, and whether the keys behind them are what they were sold as.
 * Everything here is otherwise only readable by adding up the cards below it,
 * which is the reading the home screen should not ask anyone to do.
 */
export function HomeSummary({
  total,
  agents,
  running,
  providers,
  loaded,
}: {
  /** The spend of every agent added up, by the same rule each card uses. */
  total: Pick<AgentSpend, "requests" | "cost">;
  agents: number;
  running: number;
  providers: Provider[];
  /** False until the first listing lands, when a zero would be a guess. */
  loaded: boolean;
}) {
  const {probes} = useProviderProbes();
  const {configured, summary, worst} = authenticityOf(providers, probes);
  const style = gradeStyleOf(worst?.grade);
  const dash = <span className="text-muted-foreground">-</span>;

  const cells: Cell[] = [
    {
      key: "cost",
      label: i18next.t("llm:Cost"),
      note: i18next.t("llm:All time"),
      value: loaded ? formatCost(total.cost) : dash,
      hint: i18next.t("agent:Read from the agent's own logs"),
    },
    {
      key: "requests",
      label: i18next.t("llm:Requests"),
      value: loaded ? total.requests.toLocaleString() : dash,
      to: "/usage",
    },
    {
      key: "agents",
      label: i18next.t("agent:Agents"),
      note: fill("agent:{count} running", {count: running}),
      value: agents,
    },
  ];

  if (configured.length > 0) {
    cells.push({
      key: "authenticity",
      label: i18next.t("audit:Authenticity"),
      note:
        summary.alerting > 0
          ? fill("audit:Providers alerting", {count: summary.alerting})
          : fill("audit:Graded providers", {graded: summary.graded, total: configured.length}),
      value: (
        <span className="tabular-nums">
          {gradeLetter(worst?.grade)}
          {worst ? ` · ${formatScore(worst)}` : ""}
        </span>
      ),
      tone: style.text,
      to: "/authenticity",
      hint: i18next.t(style.verdict),
    });
  }

  return (
    <Card className="gap-0 overflow-hidden py-0 shadow-xs">
      <div className={cn("grid grid-cols-2", cells.length > 3 ? "sm:grid-cols-4" : "sm:grid-cols-3")}>
        {cells.map((cell, index) => (
          <Figure
            key={cell.key}
            cell={cell}
            // Two columns on a phone rule a cross through the middle; one row on
            // a wide screen rules only between the figures.
            className={cn(
              index % 2 === 1 && "border-l",
              index >= 2 && "border-t",
              "sm:border-t-0",
              index > 0 && "sm:border-l",
            )}
          />
        ))}
      </div>
    </Card>
  );
}
