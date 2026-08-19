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
import {CircleCheck, CircleX, Pencil, Plus, Trash2} from "lucide-react";
import i18next from "i18next";

import * as ChannelBackend from "@/backend/ChannelBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column, type SortOrder} from "@/components/DataTable";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import {Tooltip} from "@/components/ui/tooltip";
import type {Account, Channel} from "@/types";

function newChannel(owner: string): Channel {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `channel_${randomName}`,
    displayName: `New Channel - ${randomName}`,
    type: "openai",
    status: "enabled",
    models: [],
    priority: 0,
    baseUrl: "",
    apiKey: "",
  };
}

export default function ChannelListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Channel[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);
  const [sort, setSort] = React.useState<{field: string; order: SortOrder}>({
    field: "",
    order: undefined,
  });

  const fetchChannels = React.useCallback(
    (nextPage = page, nextPageSize = pageSize, nextSort = sort) => {
      setLoading(true);
      ChannelBackend.getChannels(
        account.name,
        nextPage,
        nextPageSize,
        nextSort.order ? nextSort.field : "",
        nextSort.order ?? "",
      )
        .then(res => {
          setLoading(false);
          if (res.status === "ok") {
            setData(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setSort(nextSort);
          } else {
            Setting.showMessage("error", `${i18next.t("channel:Failed to get channels")}: ${res.msg}`);
          }
        })
        .catch(error => {
          setLoading(false);
          Setting.showMessage("error", `${i18next.t("channel:Failed to get channels")}: ${error}`);
        });
    },
    [account.name, page, pageSize, sort],
  );

  React.useEffect(() => {
    fetchChannels(1, 10, {field: "", order: undefined});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  const addChannel = () => {
    ChannelBackend.addChannel(newChannel(account.name))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("channel:Channel added successfully"));
          fetchChannels();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("channel:Failed to add")}: ${error}`));
  };

  const deleteChannel = (channel: Channel) => {
    ChannelBackend.deleteChannel(channel)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to delete")}: ${res.msg}`);
          return;
        }
        Setting.showMessage("success", i18next.t("channel:Channel deleted successfully"));
        fetchChannels();
      })
      .catch(error =>
        Setting.showMessage("error", `${i18next.t("channel:Failed to delete")}: ${error}`),
      );
  };

  const columns: Column<Channel>[] = [
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "160px",
      // The server sorts and paginates, so sorting locally would only reorder
      // the current page.
      sorter: true,
      render: (text: string, record) => (
        <Tooltip title={text}>
          <Link
            to={`/channels/${record.owner}/${record.name}`}
            className="block truncate text-primary hover:underline"
          >
            {text}
          </Link>
        </Tooltip>
      ),
    },
    {
      title: i18next.t("general:Display name"),
      key: "displayName",
      dataIndex: "displayName",
      width: "200px",
      sorter: true,
      render: (text: string) => (text ? <span className="block truncate">{text}</span> : "-"),
    },
    {
      title: i18next.t("channel:Type"),
      key: "type",
      dataIndex: "type",
      width: "110px",
      render: (text: string) => (
        <Badge variant={text === "openai" ? "success" : "processing"}>{text}</Badge>
      ),
    },
    {
      title: i18next.t("channel:Base URL"),
      key: "baseUrl",
      dataIndex: "baseUrl",
      width: "220px",
      render: (text: string) =>
        text ? (
          <Tooltip title={text}>
            <span className="block truncate font-mono text-xs">{text}</span>
          </Tooltip>
        ) : (
          "-"
        ),
    },
    {
      title: i18next.t("channel:Models"),
      key: "models",
      dataIndex: "models",
      width: "220px",
      render: (models: string[]) =>
        !models || models.length === 0 ? (
          "-"
        ) : (
          <div className="flex flex-wrap gap-1">
            {models.map(model => (
              <Badge key={model} variant="blue">
                {model}
              </Badge>
            ))}
          </div>
        ),
    },
    {
      title: i18next.t("channel:Priority"),
      key: "priority",
      dataIndex: "priority",
      width: "100px",
      sorter: true,
    },
    {
      title: i18next.t("channel:Status"),
      key: "status",
      dataIndex: "status",
      width: "120px",
      render: (text: string) =>
        text === "enabled" ? (
          <Badge variant="success">
            <CircleCheck className="h-3 w-3" />
            {i18next.t("channel:Enabled")}
          </Badge>
        ) : (
          <Badge variant="error">
            <CircleX className="h-3 w-3" />
            {i18next.t("channel:Disabled")}
          </Badge>
        ),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "190px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" onClick={() => navigate(`/channels/${record.owner}/${record.name}`)}>
            <Pencil />
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmButton
            title={`${i18next.t("general:Delete")} channel "${record.name}"?`}
            onConfirm={() => deleteChannel(record)}
          >
            <Button size="sm" variant="destructive">
              <Trash2 />
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
        // An admin sees channels across owners, where names may collide.
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        onSort={(field, order) => fetchChannels(1, pageSize, {field: field, order: order})}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchChannels(nextPage, nextPageSize),
        }}
        title={i18next.t("channel:Channels")}
        toolbar={
          <Button size="sm" onClick={addChannel}>
            <Plus />
            {i18next.t("channel:New Channel")}
          </Button>
        }
      />
    </div>
  );
}
