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
import {Bot, CircleX, RefreshCw, Zap} from "lucide-react";
import i18next from "i18next";

import * as ChannelBackend from "@/backend/ChannelBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {DataTable, type Column} from "@/components/DataTable";
import {PageHeader} from "@/components/FormRow";
import {UnauthorizedResult} from "@/components/Result";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Spinner} from "@/components/ui/spinner";
import {Tooltip} from "@/components/ui/tooltip";
import {agentDetailPath, agentKey, monitorAgentId, useAgents} from "@/lib/agents";
import type {Account, Agent, Channel} from "@/types";

// ProviderSwitcher lets an admin pick a channel and write it into a switchable
// agent's own config. It holds the pending selection locally so each row's
// dropdown is independent.
function ProviderSwitcher({
  agent,
  channels,
  busy,
  onSwitch,
}: {
  agent: Agent;
  channels: Channel[];
  busy: boolean;
  onSwitch: (agent: Agent, channelId: string) => void;
}) {
  const [channelId, setChannelId] = React.useState("");

  if (channels.length === 0) {
    return <span className="text-xs text-muted-foreground">{i18next.t("agent:No providers")}</span>;
  }

  const selected = channels.find(channel => `${channel.owner}/${channel.name}` === channelId);
  return (
    <div className="flex items-center gap-2">
      <Select value={channelId} onValueChange={setChannelId}>
        <SelectTrigger className="h-8 w-44">
          <SelectValue placeholder={i18next.t("agent:Select a provider")} />
        </SelectTrigger>
        <SelectContent>
          {channels.map(channel => {
            const id = `${channel.owner}/${channel.name}`;
            return (
              <SelectItem key={id} value={id}>
                {channel.displayName || channel.name}
              </SelectItem>
            );
          })}
        </SelectContent>
      </Select>
      <ConfirmButton
        title={`${i18next.t("agent:Switch Provider")}: ${agent.name}?`}
        description={selected ? `${i18next.t("agent:Provider")}: ${selected.displayName || selected.name}` : undefined}
        okText={i18next.t("agent:Switch Provider")}
        onConfirm={() => onSwitch(agent, channelId)}
      >
        <Button size="sm" variant="outline" disabled={!channelId || busy}>
          {busy ? <Spinner /> : <Zap />}
          {i18next.t("agent:Switch Provider")}
        </Button>
      </ConfirmButton>
    </div>
  );
}

export default function AgentsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {agents, loading, error, busyKey, scan, togglePatch, switchProvider} = useAgents(isAdmin);
  const [channels, setChannels] = React.useState<Channel[]>([]);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    ChannelBackend.getChannels("admin").then(res => {
      if (res.status === "ok") {
        setChannels(res.data ?? []);
      }
    });
  }, [isAdmin]);

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
      title: i18next.t("agent:Provider"),
      key: "provider",
      render: (_value, record) =>
        record.switchable ? (
          <ProviderSwitcher
            agent={record}
            channels={channels}
            busy={busyKey === agentKey(record)}
            onSwitch={switchProvider}
          />
        ) : null,
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
