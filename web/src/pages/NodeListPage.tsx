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

import * as NodeBackend from "@/backend/NodeBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {UnauthorizedResult} from "@/components/Result";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
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
  const [authorized, setAuthorized] = React.useState(true);

  const fetchNodes = React.useCallback(() => {
    setLoading(true);
    NodeBackend.getNodes(account.name).then(res => {
      setLoading(false);
      if (res.status === "ok") {
        setData(res.data ?? []);
      } else if (Setting.isResponseDenied(res)) {
        setAuthorized(false);
      } else {
        Setting.showMessage("error", res.msg);
      }
    });
  }, [account.name]);

  React.useEffect(() => {
    fetchNodes();
  }, [fetchNodes]);

  const addNode = () => {
    NodeBackend.addNode(newNode(account.name))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("node:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("node:Node added successfully"));
          fetchNodes();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("node:Failed to add")}: ${error}`));
  };

  const deleteNode = (node: Node) => {
    NodeBackend.deleteNode(node)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("node:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("node:Node deleted successfully"));
          fetchNodes();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("node:Failed to delete")}: ${error}`));
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
          <ConfirmButton
            title={i18next.t("node:Sure to delete node: {name} ?").replace("{name}", record.name)}
            onConfirm={() => deleteNode(record)}
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
        pageSize={20}
        title={i18next.t("general:Nodes")}
        toolbar={
          <Button size="sm" onClick={addNode}>
            {i18next.t("general:Add")}
          </Button>
        }
      />
    </div>
  );
}
