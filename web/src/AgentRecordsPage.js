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
import {Alert, Button, Descriptions, Input, Result, Select, Space, Table, Tag, Typography} from "antd";
import {ReloadOutlined, RobotOutlined} from "@ant-design/icons";
import i18next from "i18next";
import {Link, useHistory, useLocation} from "react-router-dom";
import * as AgentBackend from "./backend/AgentBackend";
import * as Setting from "./Setting";
import AgentIcon from "./components/AgentIcon";

const {Text, Title} = Typography;

function monitorAgentId(agentId) {
  return agentId === "codex_vscode" || agentId === "codex-vscode" ? "codex-cli" : agentId;
}

const payloadStyle = {
  background: "#f5f5f5",
  borderRadius: 4,
  fontFamily: "monospace",
  fontSize: 12,
  margin: 0,
  maxHeight: 300,
  overflow: "auto",
  padding: 8,
  whiteSpace: "pre-wrap",
  wordBreak: "break-all",
};

function formatPayload(object) {
  if (typeof object !== "string") {
    return JSON.stringify(object, null, 2);
  }

  try {
    return JSON.stringify(JSON.parse(object), null, 2);
  } catch {
    return object;
  }
}

function getOutcomeColor(outcome) {
  return {
    attempted: "processing",
    denied: "warning",
    failure: "error",
    success: "success",
  }[outcome];
}

function RecordDetail({record}) {
  const mcpTarget = record.mcpServer && `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`;

  return (
    <Descriptions bordered column={1} size="small" style={{margin: 8}}>
      <Descriptions.Item label={i18next.t("general:ID")}>{record.id}</Descriptions.Item>
      <Descriptions.Item label={i18next.t("agent:Time")}>{new Date(record.createdTime).toLocaleString()}</Descriptions.Item>
      {record.agentPath && <Descriptions.Item label={i18next.t("agent:Agent path")}><Text code>{record.agentPath}</Text></Descriptions.Item>}
      {record.user && <Descriptions.Item label={i18next.t("agent:User")}>{record.user}</Descriptions.Item>}
      {record.sessionKey && <Descriptions.Item label={i18next.t("agent:Session")}><Text code>{record.sessionKey}</Text></Descriptions.Item>}
      {record.title && <Descriptions.Item label={i18next.t("agent:Session title")}>{record.title}</Descriptions.Item>}
      {record.promptId && <Descriptions.Item label={i18next.t("agent:Prompt ID")}><Text code>{record.promptId}</Text></Descriptions.Item>}
      {record.toolUseId && <Descriptions.Item label={i18next.t("agent:Tool use ID")}><Text code>{record.toolUseId}</Text></Descriptions.Item>}
      {record.toolName && <Descriptions.Item label={i18next.t("agent:Tool")}><Text code>{record.toolName}</Text></Descriptions.Item>}
      {mcpTarget && <Descriptions.Item label={i18next.t("agent:MCP target")}><Text code>{mcpTarget}</Text></Descriptions.Item>}
      {record.model && <Descriptions.Item label={i18next.t("agent:Model")}><Text code>{record.model}</Text></Descriptions.Item>}
      {record.durationMs !== undefined && <Descriptions.Item label={i18next.t("agent:Duration")}>{record.durationMs.toLocaleString()} ms</Descriptions.Item>}
      {record.clientIp && <Descriptions.Item label={i18next.t("agent:Reported from")}><Text code>{record.clientIp}</Text></Descriptions.Item>}
      {record.detail && <Descriptions.Item label={i18next.t("agent:Detail")}>{record.detail}</Descriptions.Item>}
      {record.object && (
        <Descriptions.Item label={i18next.t("agent:Payload")}>
          <pre style={payloadStyle}>{formatPayload(record.object)}</pre>
        </Descriptions.Item>
      )}
    </Descriptions>
  );
}

export default function AgentRecordsPage({account}) {
  const history = useHistory();
  const location = useLocation();
  const search = new URLSearchParams(location.search);
  const agent = search.get("agent") || "";
  const eventType = search.get("eventType") || "";
  const outcome = search.get("outcome") || "";
  const session = search.get("session") || "";
  const isAdmin = Setting.isAdminUser(account);
  const [agents, setAgents] = useState([]);
  const [records, setRecords] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [sessionDraft, setSessionDraft] = useState(session);

  const load = useCallback(() => {
    if (!isAdmin) {
      return;
    }

    setLoading(true);
    AgentBackend.getAgentRecords(agent, eventType, outcome, session).then(res => {
      if (res.status === "ok") {
        setRecords(res.data || []);
        setError("");
      } else {
        setError(res.msg || i18next.t("agent:Failed to get agent records"));
      }
    }).catch(err => {
      setError(err.message || String(err));
    }).then(() => {
      setLoading(false);
    });
  }, [agent, eventType, isAdmin, outcome, session]);

  useEffect(() => {
    if (!isAdmin) {
      return;
    }

    AgentBackend.getAgents().then(res => {
      if (res.status === "ok") {
        setAgents(res.data || []);
      }
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

  useEffect(() => {
    setSessionDraft(session);
  }, [session]);

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

  const setFilter = (key, value) => {
    const params = new URLSearchParams(location.search);
    if (value) {
      params.set(key, value);
    } else {
      params.delete(key);
    }
    const query = params.toString();
    history.push(query ? `/agent-records?${query}` : "/agent-records");
  };

  const agentOptions = [
    {label: i18next.t("agent:All agents"), value: ""},
    ...Array.from(new Set(agents.map(item => monitorAgentId(item.agentId)))).filter(Boolean).map(value => ({label: value, value: value})),
  ];

  const columns = [
    {
      title: i18next.t("agent:Time"),
      dataIndex: "createdTime",
      key: "createdTime",
      width: 180,
      render: value => new Date(value).toLocaleString(),
    },
    {
      title: i18next.t("agent:Agent"),
      dataIndex: "agent",
      key: "agent",
      width: 150,
      render: value => (
        <Tag color="blue" style={{alignItems: "center", display: "inline-flex", gap: 6}}>
          <AgentIcon agent={value} fallback={<RobotOutlined />} size={16} />
          {value}
        </Tag>
      ),
    },
    {
      title: i18next.t("agent:Event"),
      key: "event",
      width: 180,
      render: (_, record) => (
        <Space size={4} wrap>
          <Tag>{record.eventType}</Tag>
          {record.action && <Text code>{record.action}</Text>}
          {record.outcome && <Tag color={getOutcomeColor(record.outcome)}>{record.outcome}</Tag>}
        </Space>
      ),
    },
    {
      title: i18next.t("agent:Target / Model"),
      key: "target",
      width: 190,
      render: (_, record) => {
        const target = record.mcpServer
          ? `${record.mcpServer}${record.mcpTool ? ` / ${record.mcpTool}` : ""}`
          : record.toolName;
        return (
          <Space direction="vertical" size={0}>
            {target && <Text code>{target}</Text>}
            {record.model && <Text type="secondary">{record.model}</Text>}
          </Space>
        );
      },
    },
    {
      title: i18next.t("agent:Session"),
      dataIndex: "sessionKey",
      key: "sessionKey",
      width: 220,
      render: (value, record) => value && (
        <Space direction="vertical" size={0} style={{display: "flex", minWidth: 0}}>
          {record.title && <Text strong ellipsis>{record.title}</Text>}
          <Link to={`/agent-records?agent=${encodeURIComponent(record.agent)}&session=${encodeURIComponent(value)}`} style={{overflow: "hidden", textOverflow: "ellipsis"}}>{value}</Link>
        </Space>
      ),
    },
    {
      title: i18next.t("agent:Duration"),
      dataIndex: "durationMs",
      key: "durationMs",
      width: 110,
      render: value => value === undefined ? null : `${value.toLocaleString()} ms`,
    },
  ];

  return (
    <div style={{padding: "24px"}}>
      <Space style={{display: "flex", justifyContent: "space-between", marginBottom: 16}}>
        <Title level={3} style={{margin: 0}}>{i18next.t("agent:Agent Records")}</Title>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={load}>{i18next.t("general:Refresh")}</Button>
      </Space>
      <Space wrap style={{marginBottom: 16}}>
        <Select
          value={agent}
          options={agentOptions}
          onChange={value => setFilter("agent", value)}
          style={{width: 190}}
        />
        <Select
          value={eventType}
          options={[
            {label: i18next.t("agent:All event types"), value: ""},
            ...["session", "prompt", "llm", "tool", "mcp", "permission", "subagent", "compact"].map(value => ({label: value, value: value})),
          ]}
          onChange={value => setFilter("eventType", value)}
          style={{width: 170}}
        />
        <Select
          value={outcome}
          options={[
            {label: i18next.t("agent:All outcomes"), value: ""},
            ...["attempted", "success", "failure", "denied"].map(value => ({label: value, value: value})),
          ]}
          onChange={value => setFilter("outcome", value)}
          style={{width: 160}}
        />
        <Input.Search
          allowClear
          enterButton={i18next.t("agent:Filter")}
          onChange={event => setSessionDraft(event.target.value)}
          onSearch={value => setFilter("session", value)}
          placeholder={i18next.t("agent:Session")}
          style={{width: 290}}
          value={sessionDraft}
        />
      </Space>
      {error && <Alert type="error" message={error} style={{marginBottom: 16}} />}
      <Table
        rowKey="id"
        dataSource={records}
        columns={columns}
        loading={loading}
        pagination={{pageSize: 20}}
        size="small"
        scroll={{x: 1030}}
        locale={{emptyText: i18next.t("agent:No agent records yet - patch an agent to start collecting them")}}
        expandable={{expandedRowRender: record => <RecordDetail record={record} />}}
      />
    </div>
  );
}
