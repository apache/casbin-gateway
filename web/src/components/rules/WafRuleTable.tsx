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
import type {RuleExpression} from "@/types";

const defaultRules: RuleExpression[] = [
  {
    name: "Enable XML request body parser",
    operator: "match",
    value:
      "SecRule REQUEST_HEADERS:Content-Type \"^(?:application(?:/soap\\+|/)|text/)xml\" \"id:'200000',phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=XML\"",
  },
  {
    name: "Enable JSON request body parser",
    operator: "match",
    value:
      "SecRule REQUEST_HEADERS:Content-Type \"^application/json\" \"id:'200001',phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=JSON\"",
  },
  {
    name: "Verify that we've correctly processed the request body",
    operator: "match",
    value:
      "SecRule &REQUEST_BODY \"@eq 0\" \"id:'200002',phase:2,t:none,deny,status:400,msg:'Failed to parse request body.'\"",
  },
];

export function WafRuleTable({title, table, onUpdateTable}: ExpressionTableProps) {
  const {rows, updateField, addRow, deleteRow, upRow, downRow, restore} = useExpressionTable(
    table,
    onUpdateTable,
    defaultRules,
  );

  const columns: Column<RuleExpression>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "220px",
      render: (text, _record, index) => (
        <Input value={text} onChange={event => updateField(index, "name", event.target.value)} />
      ),
    },
    {
      title: i18next.t("rule:Expression"),
      key: "value",
      dataIndex: "value",
      render: (text, _record, index) => (
        <Input
          className="font-mono text-xs"
          value={text}
          onChange={event => updateField(index, "value", event.target.value)}
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
              addRow({name: `New WAF Rule - ${rows.length}`, operator: "match", value: ""})
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
