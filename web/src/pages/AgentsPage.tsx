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
import {Link} from "react-router-dom";
import {Bot, CircleX, PlugZap, RefreshCw, RotateCcw} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import * as AgentBackend from "@/backend/AgentBackend";
import * as ChannelBackend from "@/backend/ChannelBackend";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/DataTable";
import {PageHeader} from "@/components/FormRow";
import {UnauthorizedResult} from "@/components/Result";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import {Spinner} from "@/components/ui/spinner";
import {Tooltip} from "@/components/ui/tooltip";
import {agentDetailPath, agentKey, monitorAgentId, useAgents} from "@/lib/agents";
import type {Account, Agent} from "@/types";

export default function AgentsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {agents, loading, error, busyKey, scan, togglePatch} = useAgents(isAdmin);
  const [gatewayBusy, setGatewayBusy] = React.useState("");
  const [hasAnthropicChannel, setHasAnthropicChannel] = React.useState(false);

  React.useEffect(() => {
    if (!isAdmin) return;
    ChannelBackend.getChannels(account.name).then(res => {
      if (res.status === "ok") {
        setHasAnthropicChannel((res.data ?? []).some(channel => channel.type === "anthropic" && channel.status === "enabled"));
      }
    });
  }, [account.name, isAdmin]);

  const changeGateway = (agent: Agent, restore: boolean) => {
    const target = {agentId: agent.agentId, path: agent.path, owner: agent.owner};
    setGatewayBusy(agentKey(agent));
    (restore ? AgentBackend.restoreAgentGateway(target) : AgentBackend.configureAgentGateway(target))
      .then(res => {
        if (res.status === "ok") {
          const successMessage = restore
            ? i18next.t("agent:Gateway configuration restored")
            : i18next.t("agent:Gateway connected");
          Setting.showMessage("success", `${successMessage}. ${i18next.t("agent:Restart Claude Code")}`);
          scan();
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setGatewayBusy(""));
  };

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<Agent>[] = [
    {
      title: i18next.t("agent:Agent"),
      key: "name",
      dataIndex: "name",
      render: (value: string, record) => (
        <Link to={agentDetailPath(record)} className="flex items-center gap-2 hover:text-primary">
          <AgentIcon
            agent={record.agentId || value}
            fallback={<Bot className="h-[18px] w-[18px] text-muted-foreground" />}
          />
          <span className="hover:underline">{value}</span>
        </Link>
      ),
    },
    {
      title: i18next.t("agent:Version"),
      key: "version",
      dataIndex: "version",
      render: (value: string) => value || i18next.t("agent:Unknown"),
    },
    {
      title: i18next.t("agent:Install Method"),
      key: "installMethod",
      dataIndex: "installMethod",
      render: (value: string) => <Badge variant="secondary">{value || "-"}</Badge>,
    },
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
    },
    {
      title: i18next.t("general:Path"),
      key: "path",
      dataIndex: "path",
      render: (value: string) => <code className="text-xs">{value}</code>,
    },
    {
      title: i18next.t("agent:Patch Status"),
      key: "patched",
      render: (_value, record) => {
        const badge = !record.supported ? (
          <Badge variant="secondary">{i18next.t("agent:Not supported")}</Badge>
        ) : record.patched ? (
          <Badge variant="success">{i18next.t("agent:Patched")}</Badge>
        ) : (
          <Badge variant="secondary">{i18next.t("agent:Not patched")}</Badge>
        );
        return (
          <Tooltip title={record.detail}>
            <span>{badge}</span>
          </Tooltip>
        );
      },
    },
    {
      title: i18next.t("agent:Records"),
      key: "records",
      render: (_value, record) =>
        record.patched ? (
          <Link
            to={`/agent-records?agent=${encodeURIComponent(monitorAgentId(record.agentId))}`}
            className="text-primary hover:underline"
          >
            {i18next.t("agent:View Records")}
          </Link>
        ) : null,
    },
    {
      title: i18next.t("agent:Gateway Connection"),
      key: "gatewayConfig",
      render: (_value, record) => {
        if (record.agentId !== "claude-code" || !record.gatewayConfig) return null;
        const busy = gatewayBusy === agentKey(record);
        return (
          <div className="flex flex-wrap items-center gap-2">
            {record.gatewayConfig.configured ? <Badge variant="success">{i18next.t("agent:Connected")}</Badge> : null}
            <Tooltip title={!hasAnthropicChannel ? i18next.t("agent:Create an enabled Anthropic channel first") : record.gatewayConfig.detail}>
              <span>
                <Button size="sm" variant="outline" disabled={busy || !hasAnthropicChannel} onClick={() => changeGateway(record, false)}>
                  {busy ? <Spinner /> : <PlugZap />}
                  {record.gatewayConfig.configured
                    ? i18next.t("agent:Reconfigure Gateway")
                    : i18next.t("agent:Connect Gateway")}
                </Button>
              </span>
            </Tooltip>
            {!hasAnthropicChannel ? <Link className="text-sm text-primary hover:underline" to="/channels">{i18next.t("agent:Go to Channels")}</Link> : null}
            {record.gatewayConfig.restorable ? (
              <ConfirmButton title={i18next.t("agent:Restore original Claude Code configuration?")} onConfirm={() => changeGateway(record, true)}>
                <Button size="sm" variant="outline" disabled={busy}><RotateCcw />{i18next.t("agent:Restore configuration")}</Button>
              </ConfirmButton>
            ) : null}
          </div>
        );
      },
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      render: (_value, record) => {
        if (!record.supported) {
          return (
            <Button size="sm" variant="outline" disabled>
              {i18next.t("agent:Patch")}
            </Button>
          );
        }

        const action = i18next.t(`agent:${record.patched ? "Unpatch" : "Patch"}`);
        const note = [record.notice, record.followup].filter(Boolean).join(" ");
        return (
          <ConfirmButton
            title={`${action} ${record.name}?`}
            description={note || undefined}
            okText={action}
            destructive={record.patched}
            onConfirm={() => togglePatch(record)}
          >
            <Button
              size="sm"
              variant={record.patched ? "outline" : "default"}
              disabled={busyKey === agentKey(record)}
            >
              {busyKey === agentKey(record) ? <Spinner /> : null}
              {action}
            </Button>
          </ConfirmButton>
        );
      },
    },
  ];

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("agent:Agents")}>
        <Button variant="outline" onClick={() => scan(true)} disabled={loading}>
          <RefreshCw className={loading ? "animate-spin" : undefined} />
          {i18next.t("agent:Scan")}
        </Button>
      </PageHeader>

      {error && (
        <Alert variant="destructive" className="mb-4">
          <CircleX />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <DataTable
        columns={columns}
        data={agents}
        rowKey={agentKey}
        loading={loading}
        pageSize={0}
        emptyText={i18next.t("agent:No supported agents found")}
      />
    </div>
  );
}
