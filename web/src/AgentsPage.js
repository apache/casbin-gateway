// Copyright 2025 The casbin Authors. All Rights Reserved.
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

import React, {useCallback, useEffect, useState} from "react";
import {Alert, Button, Result, Space, Table, Tag, Typography} from "antd";
import {ReloadOutlined, RobotOutlined} from "@ant-design/icons";
import i18next from "i18next";
import * as AgentBackend from "./backend/AgentBackend";
import * as Setting from "./Setting";
import AgentIcon from "./components/AgentIcon";

const {Text, Title} = Typography;

const rowKey = record => `${record.owner}:${record.path}`;

export default function AgentsPage({account}) {
  const [agents, setAgents] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const isAdmin = Setting.isAdminUser(account);

  const scan = useCallback((forceRefresh = false) => {
    if (!isAdmin) {
      return;
    }

    setLoading(true);
    setError("");
    AgentBackend.getAgents(forceRefresh).then(res => {
      if (res.status === "ok") {
        setAgents(res.data || []);
      } else {
        setError(res.msg || i18next.t("agent:Failed to scan agents"));
      }
    }).catch(err => {
      setError(err.message || String(err));
    }).then(() => {
      setLoading(false);
    });
  }, [isAdmin]);

  useEffect(() => {
    scan();
  }, [scan]);

  if (!isAdmin) {
    return (
      <Result
        status="403"
        title="403 Unauthorized"
        subTitle={i18next.t("general:Sorry, you do not have permission to access this page or logged in status invalid.")}
        extra={<a href="/"><Button type="primary">{i18next.t("general:Back Home")}</Button></a>}
      />
    );
  }

  const columns = [
    {
      title: i18next.t("agent:Agent"),
      dataIndex: "name",
      key: "name",
      render: (value, record) => (
        <Space size={8}>
          <AgentIcon
            agent={record.agentId || value}
            fallback={<RobotOutlined style={{fontSize: 18, color: "#8c8c8c"}} />}
          />
          {value}
        </Space>
      ),
    },
    {
      title: i18next.t("agent:Version"),
      dataIndex: "version",
      key: "version",
      render: value => value || i18next.t("agent:Unknown"),
    },
    {
      title: i18next.t("agent:Install Method"),
      dataIndex: "installMethod",
      key: "installMethod",
      render: value => <Tag>{value || "-"}</Tag>,
    },
    {
      title: i18next.t("general:Owner"),
      dataIndex: "owner",
      key: "owner",
    },
    {
      title: i18next.t("general:Path"),
      dataIndex: "path",
      key: "path",
      render: value => <Text code>{value}</Text>,
    },
  ];

  return (
    <div style={{padding: "24px"}}>
      <Space style={{display: "flex", justifyContent: "space-between", marginBottom: 16}}>
        <Title level={3} style={{margin: 0}}>{i18next.t("agent:Agents")}</Title>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={() => scan(true)}>
          {i18next.t("agent:Scan")}
        </Button>
      </Space>
      {error && <Alert type="error" message={error} style={{marginBottom: 16}} />}
      <Table
        rowKey={rowKey}
        dataSource={agents}
        columns={columns}
        loading={loading}
        pagination={false}
        locale={{emptyText: i18next.t("agent:No supported agents found")}}
        size="small"
        bordered
        scroll={{x: true}}
      />
    </div>
  );
}
