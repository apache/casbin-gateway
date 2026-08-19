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
import {Link, useNavigate} from "react-router-dom";
import i18next from "i18next";

import * as RuleBackend from "@/backend/RuleBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import type {Account, Rule} from "@/types";

function newRule(owner: string): Rule {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `rule_${randomName}`,
    createdTime: new Date().toISOString(),
    updatedTime: "",
    type: "User-Agent",
    expressions: [],
    action: "Block",
    statusCode: 403,
    reason: "Your request is blocked.",
    isVerbose: false,
  };
}

export default function RuleListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Rule[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);

  const fetchRules = React.useCallback(
    (nextPage = page, nextPageSize = pageSize) => {
      setLoading(true);
      RuleBackend.getRules(account.name, nextPage, nextPageSize).then(res => {
        setLoading(false);
        if (res.status === "ok") {
          setData(res.data ?? []);
          setTotal(res.data2 ?? 0);
          setPage(nextPage);
          setPageSize(nextPageSize);
        } else {
          Setting.showMessage("error", `Failed to get rules: ${res.msg}`);
        }
      });
    },
    [account.name, page, pageSize],
  );

  React.useEffect(() => {
    fetchRules(1, 10);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  const addRule = () => {
    RuleBackend.addRule(newRule(account.name)).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `Failed to add: ${res.msg}`);
      } else {
        Setting.showMessage("success", "Rule added successfully");
        fetchRules();
      }
    });
  };

  const deleteRule = (rule: Rule) => {
    RuleBackend.deleteRule(rule).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `Failed to delete: ${res.msg}`);
      } else {
        Setting.showMessage("success", "Deleted successfully");
        fetchRules();
      }
    });
  };

  const columns: Column<Rule>[] = [
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
      width: "130px",
      sorter: (a, b) => a.owner.localeCompare(b.owner),
    },
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "180px",
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (text: string, record) => (
        <Link to={`/rules/${record.owner}/${text}`} className="text-primary hover:underline">
          {text}
        </Link>
      ),
    },
    {
      title: i18next.t("general:Create time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "180px",
      sorter: (a, b) => a.createdTime.localeCompare(b.createdTime),
      render: (text: string) => Setting.getFormattedDate(text),
    },
    {
      title: i18next.t("general:Update time"),
      key: "updatedTime",
      dataIndex: "updatedTime",
      width: "180px",
      sorter: (a, b) => (a.updatedTime ?? "").localeCompare(b.updatedTime ?? ""),
      render: (text: string) => Setting.getFormattedDate(text),
    },
    {
      title: i18next.t("rule:Type"),
      key: "type",
      dataIndex: "type",
      width: "130px",
      sorter: (a, b) => a.type.localeCompare(b.type),
      render: (text: string) => <Badge variant="blue">{i18next.t(`rule:${text}`)}</Badge>,
    },
    {
      title: i18next.t("rule:Expressions"),
      key: "expressions",
      dataIndex: "expressions",
      render: (_text, record) => (
        <div className="flex flex-wrap gap-1">
          {(record.expressions ?? []).map((expression, index) => (
            <Badge key={index} variant="success">
              {`${expression.operator} ${expression.value.slice(0, 20)}`}
            </Badge>
          ))}
        </div>
      ),
    },
    {
      title: i18next.t("general:Action"),
      key: "ruleAction",
      dataIndex: "action",
      width: "110px",
      sorter: (a, b) => a.action.localeCompare(b.action),
    },
    {
      title: i18next.t("rule:Status code"),
      key: "statusCode",
      dataIndex: "statusCode",
      width: "120px",
      sorter: (a, b) => (a.statusCode ?? 0) - (b.statusCode ?? 0),
    },
    {
      title: i18next.t("rule:Reason"),
      key: "reason",
      dataIndex: "reason",
      width: "260px",
      sorter: (a, b) => (a.reason ?? "").localeCompare(b.reason ?? ""),
    },
    {
      title: i18next.t("general:Action"),
      key: "op",
      width: "180px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => navigate(`/rules/${record.owner}/${record.name}`)}>
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmButton
            title={`Sure to delete rule: ${record.name} ?`}
            onConfirm={() => deleteRule(record)}
          >
            <Button size="sm" variant="destructive">
              {i18next.t("general:Delete")}
            </Button>
          </ConfirmButton>
        </div>
      ),
    },
  ];

  return (
    <div className="p-4 md:p-6">
      <DataTable
        columns={columns}
        data={data}
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchRules(nextPage, nextPageSize),
        }}
        title={i18next.t("general:Rules")}
        toolbar={
          <Button size="sm" onClick={addRule}>
            {i18next.t("general:Add")}
          </Button>
        }
      />
    </div>
  );
}
