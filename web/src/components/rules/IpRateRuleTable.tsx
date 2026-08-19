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

import i18next from "i18next";

import {DataTable, type Column} from "@/components/DataTable";
import {useExpressionTable, type ExpressionTableProps} from "@/components/rules/useExpressionTable";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {NumberInput} from "@/components/ui/number-input";
import type {RuleExpression} from "@/types";

// The server keeps both numbers as strings in the generic expression row, so
// `operator` is the rate and `value` the block duration.
const defaultRules: RuleExpression[] = [
  {name: "Default IP Rate", operator: "100", value: "6000"},
];

export function IpRateRuleTable({title, table, onUpdateTable}: ExpressionTableProps) {
  const {rows, updateField, restore} = useExpressionTable(table, onUpdateTable, defaultRules);

  const columns: Column<RuleExpression>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "25%",
      render: (text, _record, index) => (
        <Input value={text} onChange={event => updateField(index, "name", event.target.value)} />
      ),
    },
    {
      title: i18next.t("rule:Rate"),
      key: "operator",
      dataIndex: "operator",
      width: "35%",
      render: (text, _record, index) => (
        <NumberInput
          min={0}
          value={Number(text)}
          addonAfter="requests / ip / s"
          onChange={value => updateField(index, "operator", String(value))}
        />
      ),
    },
    {
      title: i18next.t("rule:Block Duration"),
      key: "value",
      dataIndex: "value",
      render: (text, _record, index) => (
        <NumberInput
          min={0}
          value={Number(text)}
          addonAfter={i18next.t("usage:seconds")}
          onChange={value => updateField(index, "value", String(value))}
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
        <Button size="sm" variant="outline" onClick={restore}>
          {i18next.t("general:Restore")}
        </Button>
      }
    />
  );
}
