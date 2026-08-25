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
import {Plus, RefreshCw, Server} from "lucide-react";
import i18next from "i18next";

import * as NodeBackend from "@/backend/NodeBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {UnauthorizedResult} from "@/components/shared/misc";
import {Button} from "@/components/ui/button";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Input} from "@/components/ui/input";
import type {Account, Node} from "@/types";

function newNode(owner: string): Node {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `node_${randomName}`,
    createdTime: new Date().toISOString(),
    displayName: `New Node - ${randomName}`,
    tag: "",
    clientIp: "",
    upgradeMode: "At Any Time",
  };
}

export default function NodeListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Node[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [authorized, setAuthorized] = React.useState(true);
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Node>(() => newNode(account.name));
  const [nameError, setNameError] = React.useState("");

  const fetchNodes = React.useCallback(() => {
    setLoading(true);
    NodeBackend.getNodes(account.name)
      .then(res => {
        if (res.status === "ok") {
          setData(res.data ?? []);
          setError("");
        } else if (Setting.isResponseDenied(res)) {
          setAuthorized(false);
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [account.name]);

  React.useEffect(() => {
    fetchNodes();
  }, [fetchNodes]);

  const openAddDialog = () => {
    setForm(newNode(account.name));
    setNameError("");
    setAddOpen(true);
  };

  const addNode = () => {
    const name = form.name.trim();
    if (name === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    setAdding(true);
    NodeBackend.addNode({...form, name: name})
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Added successfully"));
          setAddOpen(false);
          fetchNodes();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${error}`);
      });
  };

  const deleteNode = (node: Node) => {
    NodeBackend.deleteNode(node)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          fetchNodes();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${error}`));
  };

  if (!authorized) {
    return <UnauthorizedResult />;
  }

  const columns: Column<Node>[] = [
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
      width: "110px",
      sorter: (a, b) => a.owner.localeCompare(b.owner),
    },
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "170px",
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (text: string, record) => (
        <Link to={`/nodes/${record.owner}/${record.name}`} className="text-primary hover:underline">
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
      title: i18next.t("general:Display name"),
      key: "displayName",
      dataIndex: "displayName",
      sorter: (a, b) => a.displayName.localeCompare(b.displayName),
    },
    {
      title: i18next.t("general:Tag"),
      key: "tag",
      dataIndex: "tag",
      width: "140px",
      sorter: (a, b) => (a.tag ?? "").localeCompare(b.tag ?? ""),
    },
    {
      title: i18next.t("general:Client IP"),
      key: "clientIp",
      dataIndex: "clientIp",
      width: "150px",
      sorter: (a, b) => a.clientIp.localeCompare(b.clientIp),
    },
    {
      title: i18next.t("general:Upgrade mode"),
      key: "upgradeMode",
      dataIndex: "upgradeMode",
      width: "160px",
      sorter: (a, b) => a.upgradeMode.localeCompare(b.upgradeMode),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "180px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => navigate(`/nodes/${record.owner}/${record.name}`)}>
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmDialog
            title={i18next.t("general:Sure to delete {name} ?").replace("{name}", record.name)}
            onConfirm={() => deleteNode(record)}
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
        title={i18next.t("general:Nodes")}
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
        onRetry={() => fetchNodes()}
        pageSize={20}
        searchable
        title={i18next.t("general:Nodes")}
        description={`${data.length} ${i18next.t("general:Nodes")}`}
        emptyIcon={Server}
        toolbar={
          <Button variant="outline" size="sm" onClick={() => fetchNodes()} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("node:New Node")}
        submitting={adding}
        onSubmit={addNode}
      >
        <Field label={i18next.t("general:Name")} htmlFor="node-name" required error={nameError}>
          <Input
            id="node-name"
            value={form.name}
            onChange={event => {
              setForm({...form, name: event.target.value});
              setNameError("");
            }}
          />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="node-display-name">
          <Input
            id="node-display-name"
            value={form.displayName}
            onChange={event => setForm({...form, displayName: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("general:Tag")} htmlFor="node-tag">
          <Input
            id="node-tag"
            value={form.tag}
            onChange={event => setForm({...form, tag: event.target.value})}
          />
        </Field>
      </FormDialog>
    </PageContainer>
  );
}
