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
import i18next from "i18next";

import * as RecordBackend from "@/backend/RecordBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import type {Account, Record as GatewayRecord} from "@/types";

export default function RecordListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<GatewayRecord[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);

  const fetchRecords = React.useCallback(
    (nextPage = page, nextPageSize = pageSize) => {
      setLoading(true);
      RecordBackend.getRecords(account.name, nextPage, nextPageSize).then(res => {
        setLoading(false);
        if (res.status === "ok") {
          setData(res.data ?? []);
          setTotal(res.data2 ?? 0);
          setPage(nextPage);
          setPageSize(nextPageSize);
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get records")}: ${res.msg}`);
        }
      });
    },
    [account.name, page, pageSize],
  );

  React.useEffect(() => {
    fetchRecords(1, 10);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  const addRecord = () => {
    const randomName = Setting.getRandomName();
    RecordBackend.addRecord({
      owner: account.name,
      name: `record_${randomName}`,
      createdTime: new Date().toISOString(),
      method: "GET",
      host: "door.casdoor.com",
      path: "/",
      userAgent: "",
    })
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Record added successfully"));
          fetchRecords();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${error}`));
  };

  const deleteRecord = (record: GatewayRecord) => {
    RecordBackend.deleteRecord(record)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Record deleted successfully"));
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
          <ConfirmButton title={i18next.t("general:Sure to delete record?")} onConfirm={() => deleteRecord(record)}>
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
        rowKey={record => String(record.id)}
        loading={loading}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchRecords(nextPage, nextPageSize),
        }}
        title={i18next.t("general:Records")}
        toolbar={
          <Button size="sm" onClick={addRecord}>
            {i18next.t("general:Add")}
          </Button>
        }
      />
    </div>
  );
}
