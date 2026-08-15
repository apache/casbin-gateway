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
import {Link} from "react-router-dom";
import * as AgentBackend from "./backend/AgentBackend";
import * as Setting from "./Setting";
import AgentIcon from "./components/AgentIcon";

const {Text, Title} = Typography;

export default function AgentSessionsPage({account}) {
  const [sessions, setSessions] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const isAdmin = Setting.isAdminUser(account);

  const load = useCallback(() => {
    if (!isAdmin) {
      return;
    }

    setLoading(true);
    AgentBackend.getAgentSessions().then(res => {
      if (res.status === "ok") {
        setSessions(res.data || []);
        setError("");
      } else {
        setError(res.msg || i18next.t("agent:Failed to get agent sessions"));
      }
    }).catch(err => {
      setError(err.message || String(err));
    }).then(() => {
      setLoading(false);
    });
  }, [isAdmin]);

  useEffect(() => {
    if (!isAdmin) {
      return;
    }

    load();
    const interval = setInterval(load, 3000);
    return () => clearInterval(interval);
  }, [isAdmin, load]);

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
      title: i18next.t("agent:Session"),
      key: "session",
      render: (_, session) => (
        <Space direction="vertical" size={0}>
          <Link to={`/agent-records?agent=${encodeURIComponent(session.agent)}&session=${encodeURIComponent(session.sessionKey)}`}>
            {session.title || session.sessionKey}
          </Link>
          <Text type="secondary" style={{fontSize: 12}} ellipsis>{session.sessionKey}</Text>
        </Space>
      ),
    },
    {
      title: i18next.t("agent:Agent"),
      dataIndex: "agent",
      key: "agent",
      render: value => (
        <Tag color="blue" style={{alignItems: "center", display: "inline-flex", gap: 6}}>
          <AgentIcon agent={value} fallback={<RobotOutlined />} size={16} />
          {value}
        </Tag>
      ),
    },
    {
      title: i18next.t("agent:Records"),
      dataIndex: "recordCount",
      key: "recordCount",
    },
    {
      title: i18next.t("agent:First activity"),
      dataIndex: "firstTime",
      key: "firstTime",
      render: value => new Date(value).toLocaleString(),
    },
    {
      title: i18next.t("agent:Last activity"),
      dataIndex: "lastTime",
      key: "lastTime",
      render: value => new Date(value).toLocaleString(),
    },
  ];

  return (
    <div style={{padding: "24px"}}>
      <Space style={{display: "flex", justifyContent: "space-between", marginBottom: 16}}>
        <Title level={3} style={{margin: 0}}>{i18next.t("agent:Agent Sessions")}</Title>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>{i18next.t("general:Refresh")}</Button>
      </Space>
      {error && <Alert type="error" message={error} style={{marginBottom: 16}} />}
      <Table
        rowKey={session => `${session.agent}:${session.sessionKey}`}
        dataSource={sessions}
        columns={columns}
        loading={loading}
        pagination={{pageSize: 20}}
        size="small"
        locale={{emptyText: i18next.t("agent:No agent sessions yet - patch an agent to start collecting them")}}
      />
    </div>
  );
}
