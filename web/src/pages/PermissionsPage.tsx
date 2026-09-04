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
import {useSearchParams} from "react-router-dom";
import {Bot, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as PermissionBackend from "@/backend/PermissionBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {PermissionCard} from "@/components/PermissionCard";
import {EmptyState} from "@/components/shared/empty-state";
import {AiDots} from "@/components/shared/loading";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {UnauthorizedResult} from "@/components/shared/misc";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {agentRoutedHere, useAgents} from "@/lib/agents";
import {cn} from "@/lib/utils";
import type {Account, Agent, AgentPermission, Provider} from "@/types";

/** One agent in the rail, with what it is held to right now. */
function AgentRow({
  agent,
  permission,
  active,
  onSelect,
}: {
  agent: Agent;
  permission?: AgentPermission;
  active: boolean;
  onSelect: () => void;
}) {
  const blocked = Object.values(permission?.tools ?? {}).filter(allowed => !allowed).length;
  const restricted = permission?.enabled === true;

  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "flex w-full items-center gap-2.5 rounded-md border px-2.5 py-2 text-left text-sm transition-colors",
        active ? "border-primary bg-primary/10" : "hover:bg-accent",
      )}
    >
      <AgentIcon
        agent={agent.agentId || agent.name}
        size={20}
        fallback={<Bot className="h-5 w-5 text-muted-foreground" />}
      />
      <span className="min-w-0 flex-1 truncate">{agent.name}</span>
      {!restricted ? (
        <Badge variant="muted" className="shrink-0 font-normal">
          {i18next.t("agent:Unrestricted")}
        </Badge>
      ) : !agentRoutedHere(agent) ? (
        <Badge variant="warning" className="shrink-0 font-normal">
          {i18next.t("agent:Not enforced")}
        </Badge>
      ) : blocked === 0 ? (
        <Badge variant="muted" className="shrink-0 font-normal">
          {i18next.t("agent:Enforced")}
        </Badge>
      ) : (
        <Badge variant="success" className="shrink-0 font-normal">
          {`${blocked} ${i18next.t("agent:blocked")}`}
        </Badge>
      )}
    </button>
  );
}

/**
 * One page for what every agent on this machine may do. The rail is the agents,
 * the panel is the switches of the one picked, and both read the same rules the
 * proxy holds each relayed request to.
 */
export default function PermissionsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [searchParams, setSearchParams] = useSearchParams();
  const {agents, scanned, scan} = useAgents(isAdmin);
  const [permissions, setPermissions] = React.useState<AgentPermission[]>([]);
  const [providers, setProviders] = React.useState<Provider[]>([]);

  // The rail lists one row per agent, not per installation: the rules are
  // stored per agent id, so two copies of one agent are held to the same ones.
  const unique: Agent[] = [];
  agents.forEach(agent => {
    if (!unique.some(seen => seen.agentId === agent.agentId)) {
      unique.push(agent);
    }
  });

  const selectedId = searchParams.get("agent") ?? "";
  const selected = unique.find(agent => agent.agentId === selectedId) ?? unique[0];

  const load = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    PermissionBackend.getAgentPermissions()
      .then(res => setPermissions(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setPermissions([]));
  }, [isAdmin]);

  React.useEffect(load, [load]);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    ProviderBackend.getProviders(account.name)
      .then(res => setProviders(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setProviders([]));
  }, [isAdmin, account.name]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const permissionOf = (agentId: string) =>
    permissions.find(permission => permission.name === agentId);

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Permissions")}
        description={i18next.t("agent:Permissions page hint")}
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              scan(true);
              load();
            }}
          >
            <RefreshCw />
            {i18next.t("agent:Scan")}
          </Button>
        }
      />

      {!scanned ? (
        <div className="flex justify-center p-10">
          <AiDots size="small" />
        </div>
      ) : unique.length === 0 ? (
        <EmptyState
          icon={Bot}
          title={i18next.t("agent:No agent found")}
          description={i18next.t("agent:Permissions empty hint")}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-[260px_minmax(0,1fr)]">
          <div className="flex flex-col gap-1.5">
            {unique.map(agent => (
              <AgentRow
                key={agent.agentId}
                agent={agent}
                permission={permissionOf(agent.agentId)}
                active={selected?.agentId === agent.agentId}
                onSelect={() => setSearchParams({agent: agent.agentId})}
              />
            ))}
          </div>

          {selected ? (
            <PermissionCard
              key={selected.agentId}
              agent={selected}
              providers={providers}
              className="self-start"
              onSaved={load}
            />
          ) : null}
        </div>
      )}
    </PageContainer>
  );
}
