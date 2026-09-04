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

import {Link} from "react-router-dom";
import {ArrowLeft, Bot, Container, Eye, EyeOff, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {AgentCatalog} from "@/components/AgentCatalog";
import {AgentIcon} from "@/components/AgentIcon";
import {RunBadge, RunButton} from "@/components/AgentRunControl";
import {ToolUpgradeConfirmDialog} from "@/components/ToolUpgradeConfirmDialog";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {CodeText, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {
  agentBuiltin,
  agentDetailPath,
  agentKey,
  directMode,
  monitorAgentId,
  runtimeOf,
  useAgentInstall,
  useAgents,
} from "@/lib/agents";
import type {Account, Agent} from "@/types";

export default function AgentsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {
    agents,
    loading,
    error,
    busyKey,
    inContainer,
    scanned,
    runtime,
    runBusyKey,
    scan,
    loadRuntime,
    toggleRunning,
    togglePatch,
  } = useAgents(isAdmin);
  // One installer for the page: it installs what is missing below the table and
  // upgrades what is in it.
  const installer = useAgentInstall(isAdmin, () => scan(true));

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const columns: Column<Agent>[] = [
    {
      title: i18next.t("agent:Agent"),
      key: "name",
      dataIndex: "name",
      render: (value: string, record) => (
        <Link to={agentDetailPath(record, agents)} className="hover:text-primary flex items-center gap-2">
          <AgentIcon agent={record.agentId || value} fallback={<Bot className="text-muted-foreground size-4" />} />
          <span className="font-medium hover:underline">{value}</span>
        </Link>
      ),
    },
    {
      title: i18next.t("agent:Run Status"),
      key: "running",
      render: (_value, record) => <RunBadge status={runtimeOf(runtime, record)} />,
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
      render: (value: string) => <Badge variant="muted">{value || "-"}</Badge>,
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
      ellipsis: true,
      render: (value: string) => <CodeText copyable>{value}</CodeText>,
    },
    {
      title: i18next.t("agent:Provider"),
      key: "provider",
      dataIndex: "provider",
      render: (value: string, record) =>
        value ? (
          <div className="flex flex-col items-start gap-1">
            <Link to={`/providers/${value}`} className="text-primary hover:underline">
              {value}
            </Link>
            <span className="flex flex-wrap items-center gap-1">
              {record.fallbacks?.length ? (
                <SimpleTooltip title={record.fallbacks.join(", ")}>
                  <Badge variant="muted">{`+${record.fallbacks.length}`}</Badge>
                </SimpleTooltip>
              ) : null}
              {record.providerConfig?.applied ? (
                <SimpleTooltip title={record.providerConfig.detail}>
                  <Badge variant="success">
                    {i18next.t(record.mode === directMode ? "agent:Direct" : "agent:Gateway")}
                  </Badge>
                </SimpleTooltip>
              ) : null}
              {record.providerConfig?.envConflicts?.length ? (
                <SimpleTooltip
                  title={`${i18next.t("agent:Environment overrides this configuration")}: ${record.providerConfig.envConflicts
                    .map(conflict => conflict.key)
                    .join(", ")}`}
                >
                  <Badge variant="warning">{i18next.t("agent:Env override")}</Badge>
                </SimpleTooltip>
              ) : null}
            </span>
          </div>
        ) : (
          <SimpleTooltip title={i18next.t("agent:Built-in hint")}>
            <span className="text-muted-foreground">{agentBuiltin(record)}</span>
          </SimpleTooltip>
        ),
    },
    {
      title: i18next.t("agent:Patch Status"),
      key: "patched",
      render: (_value, record) => {
        const badge = !record.supported ? (
          <Badge variant="muted">{i18next.t("agent:Not supported")}</Badge>
        ) : record.patched ? (
          <Badge variant="success">{i18next.t("agent:Patched")}</Badge>
        ) : (
          <Badge variant="muted">{i18next.t("agent:Not patched")}</Badge>
        );
        return (
          <SimpleTooltip title={record.detail}>
            <span>{badge}</span>
          </SimpleTooltip>
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
      title: i18next.t("general:Action"),
      key: "action",
      render: (_value, record) => (
        <div className="flex items-center gap-2">
          <RunButton
            agent={record}
            status={runtimeOf(runtime, record)}
            busy={runBusyKey === agentKey(record)}
            onToggle={toggleRunning}
          />
          <ToolUpgradeConfirmDialog
            agent={record}
            job={installer.jobs[record.agentId]}
            busy={installer.busyId === record.agentId}
            onConfirm={() => installer.upgrade(record)}
          />
          {patchButton(record)}
        </div>
      ),
    },
  ];

  function patchButton(record: Agent) {
    if (!record.supported) {
      return (
        <Button size="sm" variant="outline" disabled>
          <Eye />
          {i18next.t("agent:Patch")}
        </Button>
      );
    }

    const action = i18next.t(`agent:${record.patched ? "Unpatch" : "Patch"}`);
    const Icon = record.patched ? EyeOff : Eye;
    const note = [record.notice, record.followup].filter(Boolean).join(" ");
    return (
      <ConfirmDialog
        title={`${action} ${record.name}?`}
        description={note || undefined}
        confirmText={action}
        variant={record.patched ? "destructive" : "default"}
        onConfirm={() => togglePatch(record)}
      >
        <Button
          size="sm"
          variant={record.patched ? "outline" : "default"}
          loading={busyKey === agentKey(record)}
        >
          <Icon />
          {action}
        </Button>
      </ConfirmDialog>
    );
  }

  return (
    <PageContainer>
      <Link
        to="/"
        className="text-muted-foreground hover:text-foreground inline-flex items-center gap-1 text-sm"
      >
        <ArrowLeft className="size-4" />
        {i18next.t("agent:Agents")}
      </Link>

      <PageHeader
        title={i18next.t("agent:Advanced view")}
        description={`${account.hostname} · ${i18next.t("agent:Advanced view hint")}`}
      />

      {error ? <MessageAlert title={error} /> : null}

      <DataTable
        title={i18next.t("agent:Agents")}
        description={`${agents.length} ${i18next.t("agent:Agents")}`}
        columns={columns}
        dataSource={agents}
        rowKey={agentKey}
        loading={loading}
        pageSize={0}
        searchable
        emptyIcon={inContainer ? Container : Bot}
        emptyText={i18next.t(
          inContainer ? "agent:Running in a container detail" : "agent:No supported agents found",
        )}
        toolbar={
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              scan(true);
              loadRuntime();
            }}
            loading={loading}
          >
            <RefreshCw />
            {i18next.t("agent:Scan")}
          </Button>
        }
      />

      <AgentCatalog agents={agents} enabled={scanned} installer={installer} />
    </PageContainer>
  );
}
