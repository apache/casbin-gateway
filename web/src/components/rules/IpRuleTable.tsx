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
import {TagsInput} from "@/components/ui/tags-input";
import type {RuleExpression} from "@/types";

const defaultRules: RuleExpression[] = [
  {name: "loopback", operator: "is in", value: "127.0.0.1"},
  {name: "lan cidr", operator: "is in", value: "10.0.0.0/8,192.168.0.0/16"},
];

export function IpRuleTable({title, table, onUpdateTable}: ExpressionTableProps) {
  const {rows, updateField, addRow, deleteRow, upRow, downRow, restore} = useExpressionTable(
    table,
    onUpdateTable,
    defaultRules,
  );

  const operators = [
    {value: "is in", label: i18next.t("rule:is in")},
    {value: "is not in", label: i18next.t("rule:is not in")},
    {value: "is abroad", label: i18next.t("rule:is abroad")},
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
      title: i18next.t("rule:IP List"),
      key: "value",
      dataIndex: "value",
      // The server stores the list as one comma-joined string, so the chips are
      // split apart for editing and joined back on every change.
      render: (text: string, _record, index) => (
        <TagsInput
          placeholder="Input IP Addresses"
          value={text ? text.split(",") : []}
          onChange={value => updateField(index, "value", value.map(item => item.trim()).join(","))}
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
              addRow({name: `New IP Rule - ${rows.length}`, operator: "is in", value: "127.0.0.1"})
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
