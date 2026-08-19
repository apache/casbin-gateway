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
import {RowActions} from "@/components/rules/RowActions";
import {useExpressionTable, type ExpressionTableProps} from "@/components/rules/useExpressionTable";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {RuleExpression} from "@/types";

/** The same table serves the User-Agent and the URL Path rule types. */
export function UaRuleTable({
  title,
  table,
  onUpdateTable,
  kind = "userAgent",
}: ExpressionTableProps & {kind?: "userAgent" | "urlPath"}) {
  const isPath = kind === "urlPath";
  const defaultRules: RuleExpression[] = isPath
    ? [{name: "Example", operator: "contains", value: "/.git/config"}]
    : [{name: "Current User-Agent", operator: "equals", value: window.navigator.userAgent}];

  const {rows, updateField, addRow, deleteRow, upRow, downRow, restore} = useExpressionTable(
    table,
    onUpdateTable,
    defaultRules,
  );

  const operators = [
    {value: "equals", label: i18next.t("rule:equals")},
    {value: "does not equal", label: i18next.t("rule:does not equal")},
    {value: "contains", label: i18next.t("rule:contains")},
    {value: "does not contain", label: i18next.t("rule:does not contain")},
    {value: "match", label: i18next.t("rule:regex match")},
  ];

  const columns: Column<RuleExpression>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "200px",
      render: (text, _record, index) => (
        <Input value={text} onChange={event => updateField(index, "name", event.target.value)} />
      ),
    },
    {
      title: i18next.t("rule:Operator"),
      key: "operator",
      dataIndex: "operator",
      width: "180px",
      render: (text, _record, index) => (
        <Select value={text} onValueChange={value => updateField(index, "operator", value)}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {operators.map(item => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ),
    },
    {
      title: i18next.t("rule:Value"),
      key: "value",
      dataIndex: "value",
      render: (text, _record, index) => (
        <Input
          value={text}
          onChange={event => updateField(index, "value", event.target.value)}
          onBlur={event => updateField(index, "value", event.target.value.replace(/\s+/g, " ").trim())}
        />
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
            onClick={() =>
              addRow({
                name: isPath
                  ? `New URL Path Rule - ${rows.length}`
                  : `New UA Rule - ${rows.length}`,
                operator: isPath ? "contains" : "equals",
                value: "",
              })
            }
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
