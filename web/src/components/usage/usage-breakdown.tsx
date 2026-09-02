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

import {DataTable, type Column} from "@/components/shared/data-table";
import {Badge} from "@/components/ui/badge";
import {Progress} from "@/components/ui/progress";
import {formatCost, formatTokens, type UsageRow} from "@/lib/usage";

/**
 * What a window went on, one row per model, provider or agent. The share bar
 * is drawn against the largest row rather than the total, so a list where one
 * name dominates still tells the rest of the rows apart.
 */
export function UsageBreakdown({
  title,
  nameLabel,
  rows,
  loading,
  showFailed = false,
  emptyText,
}: {
  title: React.ReactNode;
  nameLabel: React.ReactNode;
  rows: UsageRow[];
  loading?: boolean;
  /** Set for the relayed source, which is the only one that knows an outcome. */
  showFailed?: boolean;
  emptyText?: React.ReactNode;
}) {
  const peak = rows.reduce((largest, row) => Math.max(largest, row.tokens), 0);

  const columns: Column<UsageRow>[] = [
    {
      key: "name",
      title: nameLabel,
      render: (_value, row) => (
        <div className="flex min-w-0 flex-col gap-1">
          <span className="truncate font-mono text-xs">{row.name}</span>
          {row.detail ? <span className="text-muted-foreground truncate text-xs">{row.detail}</span> : null}
          <Progress value={peak === 0 ? 0 : (row.tokens / peak) * 100} className="h-1" />
        </div>
      ),
    },
    {
      key: "requests",
      title: i18next.t("llm:Requests"),
      align: "right",
      width: "120px",
      sorter: (left, right) => left.requests - right.requests,
      render: (_value, row) =>
        showFailed && row.failed ? (
          <div className="flex items-center justify-end gap-1.5">
            <span className="tabular-nums">{row.requests.toLocaleString()}</span>
            <Badge variant="warning">{row.failed.toLocaleString()}</Badge>
          </div>
        ) : (
          <span className="tabular-nums">{row.requests.toLocaleString()}</span>
        ),
    },
    {
      key: "tokens",
      title: i18next.t("llm:Tokens"),
      align: "right",
      width: "100px",
      sorter: (left, right) => left.tokens - right.tokens,
      render: (_value, row) => <span className="tabular-nums">{formatTokens(row.tokens)}</span>,
    },
    {
      key: "cost",
      title: i18next.t("llm:Cost"),
      align: "right",
      width: "100px",
      sorter: (left, right) => left.cost - right.cost,
      render: (_value, row) => <span className="tabular-nums">{formatCost(row.cost)}</span>,
    },
  ];

  return (
    <DataTable
      dense
      title={title}
      columns={columns}
      dataSource={rows}
      rowKey={row => row.name}
      loading={loading}
      pageSize={10}
      emptyText={emptyText}
    />
  );
}
