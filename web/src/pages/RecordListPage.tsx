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
import {useNavigate} from "react-router-dom";
import {Plus, RefreshCw, ScrollText} from "lucide-react";
import i18next from "i18next";

import * as RecordBackend from "@/backend/RecordBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
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
import type {Account, Record as GatewayRecord} from "@/types";

const METHODS = ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"];

function newRecord(owner: string): Partial<GatewayRecord> {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `record_${randomName}`,
    createdTime: new Date().toISOString(),
    method: "GET",
    host: "door.casdoor.com",
    path: "/",
    userAgent: "",
  };
}

export default function RecordListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<GatewayRecord[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Partial<GatewayRecord>>(() => newRecord(account.name));

  const fetchRecords = React.useCallback(
    (nextPage = page, nextPageSize = pageSize) => {
      setLoading(true);
      RecordBackend.getRecords(account.name, nextPage, nextPageSize)
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
    fetchRecords(1, 10);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  const openAddDialog = () => {
    setForm(newRecord(account.name));
    setAddOpen(true);
  };

  const addRecord = () => {
    setAdding(true);
    RecordBackend.addRecord(form)
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Added successfully"));
          setAddOpen(false);
          fetchRecords();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${error}`);
      });
  };

  const deleteRecord = (record: GatewayRecord) => {
    RecordBackend.deleteRecord(record)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          fetchRecords();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${error}`));
  };

  const columns: Column<GatewayRecord>[] = [
    {
      title: i18next.t("general:ID"),
      key: "id",
      dataIndex: "id",
      width: "80px",
      sorter: (a, b) => a.id - b.id,
    },
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
      width: "110px",
      sorter: (a, b) => a.owner.localeCompare(b.owner),
    },
    {
      title: i18next.t("general:Created time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "180px",
      sorter: (a, b) => a.createdTime.localeCompare(b.createdTime),
      render: (text: string) => Setting.getFormattedDate(text),
    },
    {
      title: i18next.t("general:Method"),
      key: "method",
      dataIndex: "method",
      width: "90px",
      sorter: (a, b) => a.method.localeCompare(b.method),
    },
    {
      title: i18next.t("general:Host"),
      key: "host",
      dataIndex: "host",
      width: "160px",
      sorter: (a, b) => a.host.localeCompare(b.host),
    },
    {
      title: i18next.t("general:Path"),
      key: "path",
      dataIndex: "path",
      sorter: (a, b) => a.path.localeCompare(b.path),
    },
    {
      title: i18next.t("general:Client ip"),
      key: "clientIp",
      dataIndex: "clientIp",
      width: "140px",
      sorter: (a, b) => a.clientIp.localeCompare(b.clientIp),
    },
    {
      title: i18next.t("general:User-Agent"),
      key: "userAgent",
      dataIndex: "userAgent",
      width: "260px",
      sorter: (a, b) => a.userAgent.localeCompare(b.userAgent),
      render: (text: string) => (
        <span className="block truncate text-xs text-muted-foreground" title={text}>
          {text}
        </span>
      ),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "180px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => navigate(`/records/${record.owner}/${record.id}`)}>
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmDialog title={i18next.t("general:Sure to delete {name} ?").replace("{name}", String(record.id))} onConfirm={() => deleteRecord(record)}>
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
        title={i18next.t("general:Records")}
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
        rowKey={record => String(record.id)}
        loading={loading}
        error={error}
        onRetry={() => fetchRecords()}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchRecords(nextPage, nextPageSize),
        }}
        title={i18next.t("general:Records")}
        description={`${total} ${i18next.t("general:Records")}`}
        emptyIcon={ScrollText}
        toolbar={
          <Button variant="outline" size="sm" onClick={() => fetchRecords()} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("general:New Record")}
        submitting={adding}
        onSubmit={addRecord}
      >
        <Field label={i18next.t("general:Method")}>
          <Select value={form.method} onValueChange={value => setForm({...form, method: value})}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {METHODS.map(method => (
                <SelectItem key={method} value={method}>
                  {method}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </Field>
        <Field label={i18next.t("general:Host")} htmlFor="record-host">
          <Input
            id="record-host"
            value={form.host}
            onChange={event => setForm({...form, host: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("general:Path")} htmlFor="record-path">
          <Input
            id="record-path"
            value={form.path}
            onChange={event => setForm({...form, path: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("general:User-Agent")} htmlFor="record-user-agent">
          <Input
            id="record-user-agent"
            value={form.userAgent}
            onChange={event => setForm({...form, userAgent: event.target.value})}
          />
        </Field>
      </FormDialog>
    </PageContainer>
  );
}
