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
import {Bot, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {
  AgentUpdateBadge,
  AgentVersionDialog,
  ToolUninstallConfirmDialog,
} from "@/components/AgentVersionDialog";
import {InstallJobProgress} from "@/components/AgentInstallJob";
import {AgentInstallButton, ToolUpgradeConfirmDialog} from "@/components/ToolUpgradeConfirmDialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {
  agentDetailPath,
  agentKey,
  useAgentCatalog,
  useAgentInstall,
  useAgents,
  useAgentUpdates,
} from "@/lib/agents";
import type {Account, Agent, AgentCatalogEntry, AgentUpdate} from "@/types";

/**
 * One line of the table. An installed agent brings the installation it was
 * found as; one this machine does not have brings its catalogue entry instead,
 * and the two are never both set.
 */
interface VersionRow {
  key: string;
  agentId: string;
  name: string;
  installed?: Agent;
  missing?: AgentCatalogEntry;
  update?: AgentUpdate;
}

/**
 * The version page: every agent Gateway knows, the release each one is on, the
 * release its package manager publishes, and the four things that can be done
 * about the difference - install, upgrade, go back to an older version, or
 * remove it. The agents page is about what an agent is bound to; this one is
 * about which build of it is on disk.
 */
export default function AgentVersionsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {agents, loading, error, scanned, scan} = useAgents(isAdmin);
  const missing = useAgentCatalog(agents, isAdmin && scanned);
  // Every agent, installed or not, so a row can name the release an install
  // would land on as well as the one an upgrade would.
  const updates = useAgentUpdates(isAdmin, "all");
  const installer = useAgentInstall(isAdmin, () => {
    scan(true);
    updates.reload(true);
  });

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const rows: VersionRow[] = [
    ...agents.map(agent => ({
      key: agentKey(agent),
      agentId: agent.agentId,
      name: agent.name,
      installed: agent,
      update: updates.updates[agentKey(agent)],
    })),
    ...missing.map(entry => ({
      key: `catalog:${entry.agentId}`,
      agentId: entry.agentId,
      name: entry.name,
      missing: entry,
      update: updates.missing[entry.agentId],
    })),
  ];

  const columns: Column<VersionRow>[] = [
    {
      title: i18next.t("agent:Agent"),
      key: "name",
      dataIndex: "name",
      width: "220px",
      sorter: (left, right) => left.name.localeCompare(right.name),
      render: (_value, row) => (
        <span className="flex items-center gap-2 whitespace-nowrap">
          <AgentIcon agent={row.agentId || row.name} fallback={<Bot className="text-muted-foreground size-4" />} />
          {row.installed ? (
            <Link to={agentDetailPath(row.installed, agents)} className="font-medium hover:underline">
              {row.name}
            </Link>
          ) : (
            <span className="text-muted-foreground font-medium">{row.name}</span>
          )}
        </span>
      ),
    },
    {
      title: i18next.t("agent:Installed version"),
      key: "version",
      width: "130px",
      render: (_value, row) =>
        row.installed ? (
          <span className="tabular-nums">{row.installed.version || i18next.t("agent:Unknown")}</span>
        ) : (
          <span className="text-muted-foreground">-</span>
        ),
    },
    {
      title: i18next.t("agent:Latest version"),
      key: "latest",
      width: "130px",
      render: (_value, row) =>
        row.update?.latest ? (
          <span className="tabular-nums">{row.update.latest}</span>
        ) : (
          <SimpleTooltip title={row.update?.detail}>
            <span className="text-muted-foreground">{updates.checking ? "..." : "-"}</span>
          </SimpleTooltip>
        ),
    },
    {
      title: i18next.t("agent:Status"),
      key: "status",
      width: "160px",
      render: (_value, row) => <StatusBadge row={row} />,
    },
    {
      title: i18next.t("agent:Install Method"),
      key: "installMethod",
      width: "120px",
      render: (_value, row) => (
        <Badge variant="muted">
          {row.installed ? row.installed.installMethod || "-" : row.missing?.install.manager || "-"}
        </Badge>
      ),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      render: (_value, row) => {
        const job = installer.jobs[row.agentId];
        // While one runs, the row shows what it is doing instead of the
        // buttons that would start a second.
        if (job?.running) {
          return <InstallJobProgress job={job} className="w-56" />;
        }
        return (
          <div className="flex items-center gap-2 whitespace-nowrap">
            {row.installed ? (
              <>
                <ToolUpgradeConfirmDialog
                  agent={row.installed}
                  job={job}
                  busy={installer.busyId === row.agentId}
                  onConfirm={() => installer.upgrade(row.installed as Agent)}
                />
                <ToolUninstallConfirmDialog
                  agent={row.installed}
                  job={job}
                  busy={installer.busyId === row.agentId}
                  onConfirm={() => installer.uninstall(row.installed as Agent)}
                />
              </>
            ) : (
              <AgentInstallButton
                name={row.name}
                plan={row.missing?.install}
                installUrl={row.missing?.installUrl}
                job={job}
                busy={installer.busyId === row.agentId}
                onInstall={() => installer.install(row.agentId)}
              />
            )}
            <AgentVersionDialog
              agentId={row.agentId}
              name={row.name}
              installMethod={row.installed?.installMethod ?? ""}
              installedVersion={row.installed?.version ?? ""}
              update={row.update}
              busy={installer.busyId === row.agentId}
              fallbackDetail={row.installed?.upgrade?.detail ?? row.missing?.install.detail}
              onSelect={version =>
                row.installed
                  ? installer.setVersion(row.installed, version)
                  : installer.install(row.agentId, version)
              }
            />
          </div>
        );
      },
    },
  ];

  const outdated = updates.outdated;

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agent versions")}
        description={`${account.hostname} · ${i18next.t("agent:Version page hint")}`}
      />

      {error ? <MessageAlert title={error} /> : null}

      <div className="grid gap-3 sm:grid-cols-3">
        <SummaryTile label={i18next.t("agent:Installed")} value={agents.length} />
        <SummaryTile
          label={i18next.t("agent:Updates available")}
          value={outdated}
          variant={outdated > 0 ? "warning" : undefined}
        />
        <SummaryTile label={i18next.t("agent:Not installed")} value={missing.length} />
      </div>

      <DataTable
        title={i18next.t("agent:Agents")}
        description={`${rows.length} ${i18next.t("agent:Agents")}`}
        columns={columns}
        dataSource={rows}
        rowKey="key"
        loading={loading}
        pageSize={0}
        searchable
        emptyIcon={Bot}
        emptyText={i18next.t("agent:No supported agents found")}
        toolbar={
          <Button
            variant="outline"
            size="sm"
            loading={loading || updates.checking}
            onClick={() => {
              scan(true);
              updates.reload(true);
            }}
          >
            <RefreshCw />
            {i18next.t("agent:Check for updates")}
          </Button>
        }
        expandable={{
          rowExpandable: row => installer.jobs[row.agentId] !== undefined,
          expandedRowRender: row => <InstallJobProgress job={installer.jobs[row.agentId]} />,
        }}
      />
    </PageContainer>
  );
}

function SummaryTile({
  label,
  value,
  variant,
}: {
  label: string;
  value: number;
  variant?: "warning";
}) {
  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-muted-foreground text-xs">{label}</p>
        <p className={`text-2xl font-semibold tabular-nums ${variant === "warning" ? "text-warning" : ""}`}>
          {value}
        </p>
      </CardContent>
    </Card>
  );
}

/** Where one row stands: on the current release, behind it, or not there. */
function StatusBadge({row}: {row: VersionRow}) {
  if (!row.installed) {
    return <Badge variant="muted">{i18next.t("agent:Not installed")}</Badge>;
  }
  if (row.update?.available) {
    return <AgentUpdateBadge update={row.update} />;
  }
  // Neither version could be read, so nothing is known about the difference.
  if (!row.installed.version || !row.update?.latest) {
    return (
      <SimpleTooltip title={row.update?.detail}>
        <Badge variant="muted">{i18next.t("agent:Unknown")}</Badge>
      </SimpleTooltip>
    );
  }
  return <Badge variant="success">{i18next.t("agent:Up to date")}</Badge>;
}
