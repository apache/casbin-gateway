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

import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {RowActions} from "@/components/rules/RowActions";
import {Button} from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {Account, Rule} from "@/types";

interface RuleRef {
  owner: string;
  name: string;
}

/**
 * The rule picker on the site edit page. A site stores its rules as `owner/name`
 * strings, so the rows are split for editing and joined back on every change.
 */
export function SiteRuleTable({
  title,
  account,
  sources,
  rules,
  onUpdateRules,
}: {
  title: React.ReactNode;
  account: Account;
  sources: Rule[];
  rules: string[] | null | undefined;
  onUpdateRules: (rules: string[]) => void;
}) {
  const rows: RuleRef[] = React.useMemo(
    () =>
      (rules ?? []).map(item => {
        const values = item.split("/");
        return {owner: values[0], name: values[1]};
      }),
    [rules],
  );

  const update = (table: RuleRef[]) => {
    onUpdateRules(table.map(row => `${row.owner}/${row.name}`));
  };

  const columns: Column<RuleRef>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      render: (text, _record, index) => (
        <Select
          value={text || undefined}
          onValueChange={value =>
            update(rows.map((row, i) => (i === index ? {...row, name: value} : row)))
          }
        >
          <SelectTrigger>
            <SelectValue placeholder={i18next.t("general:Rules")} />
          </SelectTrigger>
          <SelectContent>
            {/* Already-picked rules are hidden so the same one is not added twice. */}
            {Setting.getDeduplicatedArray(sources, rows, "name")
              .concat(sources.filter(source => source.name === text))
              .map(rule => (
                <SelectItem key={rule.name} value={rule.name}>
                  {rule.name}
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
      ),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "130px",
      render: (_text, _record, index) => (
        <RowActions
          index={index}
          length={rows.length}
          onUp={() => update(Setting.swapRow(rows, index - 1, index))}
          onDown={() => update(Setting.swapRow(rows, index, index + 1))}
          onDelete={() => update(Setting.deleteRow(rows, index))}
        />
      ),
    },
  ];

  return (
    <DataTable
      columns={columns}
      data={rows}
      rowKey={(_record, index) => String(index)}
      pageSize={0}
      title={title}
      toolbar={
        <Button
          size="sm"
          onClick={() => update(Setting.addRow(rows, {owner: account.name, name: ""}))}
        >
          {i18next.t("general:Add")}
        </Button>
      }
    />
  );
}
