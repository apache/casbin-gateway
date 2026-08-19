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

import * as RuleBackend from "@/backend/RuleBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {RowActions} from "@/components/rules/RowActions";
import {useExpressionTable, type ExpressionTableProps} from "@/components/rules/useExpressionTable";
import {Button} from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {RuleExpression} from "@/types";

const defaultRules: RuleExpression[] = [
  {name: "Start", operator: "begin", value: "rule1"},
  {name: "And", operator: "and", value: "rule2"},
];

/** Combines other rules with and/or, so `value` holds an `owner/name` rule id. */
export function CompoundRule({
  title,
  table,
  onUpdateTable,
  owner,
  ruleName,
}: ExpressionTableProps & {owner: string; ruleName: string}) {
  const {rows, updateField, addRow, deleteRow, upRow, downRow, restore} = useExpressionTable(
    table,
    onUpdateTable,
    defaultRules,
  );
  const [rules, setRules] = React.useState<string[]>([]);

  React.useEffect(() => {
    RuleBackend.getRules(owner).then(res => {
      if (res.status !== "ok") {
        return;
      }
      // A compound rule may not reference itself.
      setRules(
        (res.data ?? [])
          .map(rule => Setting.getItemId(rule))
          .filter(id => id !== `${owner}/${ruleName}`),
      );
    });
  }, [owner, ruleName]);

  const columns: Column<RuleExpression>[] = [
    {
      title: i18next.t("rule:Logic"),
      key: "operator",
      dataIndex: "operator",
      width: "200px",
      render: (text, _record, index) => {
        // The first row opens the expression, so it is the only one that can be
        // "begin" and the only one that cannot be and/or.
        const options =
          index === 0
            ? [{value: "begin", label: i18next.t("rule:begin")}]
            : [
              {value: "and", label: i18next.t("rule:and")},
              {value: "or", label: i18next.t("rule:or")},
            ];
        return (
          <Select value={text} onValueChange={value => updateField(index, "operator", value)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {options.map(item => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        );
      },
    },
    {
      title: i18next.t("rule:Rule"),
      key: "value",
      dataIndex: "value",
      render: (text, _record, index) => (
        <Select value={text} onValueChange={value => updateField(index, "value", value)}>
          <SelectTrigger>
            <SelectValue placeholder={i18next.t("rule:Rule")} />
          </SelectTrigger>
          <SelectContent>
            {rules.map(item => (
              <SelectItem key={item} value={item}>
                {item}
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
          onUp={() => upRow(index)}
          onDown={() => downRow(index)}
          onDelete={() => deleteRow(index)}
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
        <>
          <Button
            size="sm"
            onClick={() => addRow({name: `New Item - ${rows.length}`, operator: "and", value: ""})}
          >
            {i18next.t("general:Add")}
          </Button>
          <Button size="sm" variant="outline" onClick={restore}>
            {i18next.t("general:Restore")}
          </Button>
        </>
      }
    />
  );
}
