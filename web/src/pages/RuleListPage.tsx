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
import {ListFilter, Plus, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as RuleBackend from "@/backend/RuleBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Input} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {getRuleActions, getRuleTypes} from "@/lib/rules";
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
  const [error, setError] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Rule>(() => newRule(account.name));
  const [nameError, setNameError] = React.useState("");

  const fetchRules = React.useCallback(
    (nextPage = page, nextPageSize = pageSize) => {
      setLoading(true);
      RuleBackend.getRules(account.name, nextPage, nextPageSize)
        .then(res => {
          if (res.status === "ok") {
            setData(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setError("");
          } else {
            setError(res.msg || i18next.t("general:Failed to get data"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => setLoading(false));
    },
    [account.name, page, pageSize],
  );

  React.useEffect(() => {
    fetchRules(1, 10);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  const openAddDialog = () => {
    setForm(newRule(account.name));
    setNameError("");
    setAddOpen(true);
  };

  const addRule = () => {
    const name = form.name.trim();
    if (name === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    setAdding(true);
    RuleBackend.addRule({...form, name: name}).then(res => {
      setAdding(false);
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
      } else {
        Setting.showMessage("success", i18next.t("general:Added successfully"));
        setAddOpen(false);
        fetchRules();
      }
    });
  };

  const deleteRule = (rule: Rule) => {
    RuleBackend.deleteRule(rule).then(res => {
      if (res.status === "error") {
        Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
      } else {
        Setting.showMessage("success", i18next.t("general:Deleted successfully"));
        fetchRules();
      }
    });
  };

  const types = getRuleTypes();
  const actions = getRuleActions();

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
      render: (text: string) => <Badge variant="info">{i18next.t(`rule:${text}`)}</Badge>,
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
          <ConfirmDialog
            title={i18next.t("general:Sure to delete {name} ?").replace("{name}", record.name)}
            onConfirm={() => deleteRule(record)}
          >
            <Button size="sm" variant="destructive">
              {i18next.t("general:Delete")}
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("general:Rules")}
        actions={
          <Button onClick={openAddDialog}>
            <Plus />
            {i18next.t("general:Add")}
          </Button>
        }
      />

      <DataTable
        columns={columns}
        dataSource={data}
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        error={error}
        onRetry={() => fetchRules()}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchRules(nextPage, nextPageSize),
        }}
        title={i18next.t("general:Rules")}
        description={`${total} ${i18next.t("general:Rules")}`}
        emptyIcon={ListFilter}
        toolbar={
          <Button variant="outline" size="sm" onClick={() => fetchRules()} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("rule:New Rule")}
        submitting={adding}
        onSubmit={addRule}
      >
        <Field label={i18next.t("general:Name")} htmlFor="rule-name" required error={nameError}>
          <Input
            id="rule-name"
            value={form.name}
            onChange={event => {
              setForm({...form, name: event.target.value});
              setNameError("");
            }}
          />
        </Field>
        <Field label={i18next.t("rule:Type")}>
          <Select value={form.type} onValueChange={value => setForm({...form, type: value})}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {types.map(type => (
                <SelectItem key={type.value} value={type.value}>
                  {type.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        {form.type !== "WAF" && (
          <Field label={i18next.t("general:Action")}>
            <Select value={form.action} onValueChange={value => setForm({...form, action: value})}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {actions.map(action => (
                  <SelectItem key={action.value} value={action.value}>
                    {action.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
        )}
      </FormDialog>
    </PageContainer>
  );
}
