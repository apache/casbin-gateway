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
import {Play, Plus, RefreshCw, Square, Trash2, UserPlus, Users} from "lucide-react";
import i18next from "i18next";

import {accountLabel} from "@/components/AgentGridCard";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {CodeText} from "@/components/shared/misc";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {instancesOf, useAgentInstances, type AgentInstanceControls} from "@/lib/agents";
import {cn} from "@/lib/utils";
import type {Agent, AgentInstance} from "@/types";

/** The label of one instance: whoever is signed in to it, or what it was named. */
function instanceLabel(instance: AgentInstance) {
  const signedIn = instance.account ? accountLabel(instance.account) : "";
  return signedIn || instance.displayName || instance.instance;
}

/** Adds one more copy, which the server names and lays out on its own. */
function AddInstanceButton({
  agent,
  controls,
  compact = false,
}: {
  agent: Agent;
  controls: AgentInstanceControls;
  compact?: boolean;
}) {
  const label = i18next.t("agent:Add instance");
  const busy = controls.busyName === agent.agentId;

  if (!compact) {
    return (
      <Button size="sm" loading={busy} onClick={() => controls.add(agent)}>
        <Plus />
        {label}
      </Button>
    );
  }
  return (
    <SimpleTooltip title={label}>
      <Button
        size="icon"
        variant="ghost"
        className="size-6"
        loading={busy}
        aria-label={label}
        onClick={() => controls.add(agent)}
      >
        <Plus />
      </Button>
    </SimpleTooltip>
  );
}

/**
 * The name of one instance, editable where it is listed. A copy is added in one
 * click and numbered by the server, so this is where it is given a name worth
 * reading - once there is an account in it to name it after.
 */
function InstanceName({
  instance,
  onRename,
}: {
  instance: AgentInstance;
  onRename: (instance: AgentInstance, displayName: string) => void;
}) {
  const [value, setValue] = React.useState(instance.displayName);

  // A reload brings the stored name back, which is what a rejected rename has
  // to fall back to.
  React.useEffect(() => setValue(instance.displayName), [instance.displayName]);

  return (
    <Input
      value={value}
      placeholder={instance.instance}
      aria-label={i18next.t("general:Name")}
      className="hover:border-input h-7 border-transparent px-1 font-medium shadow-none"
      onChange={event => setValue(event.target.value)}
      onBlur={() => {
        const named = value.trim();
        if (named !== instance.displayName) {
          onRename(instance, named);
        }
      }}
      onKeyDown={event => {
        if (event.key === "Enter") {
          event.currentTarget.blur();
        } else if (event.key === "Escape") {
          setValue(instance.displayName);
        }
      }}
    />
  );
}

/** Starts or stops one copy, as the icon that says which it would do. */
function InstanceRunButton({
  instance,
  busy,
  onToggle,
}: {
  instance: AgentInstance;
  busy: boolean;
  onToggle: (instance: AgentInstance) => void;
}) {
  const label = i18next.t(instance.running ? "agent:Stop" : "agent:Start");
  return (
    <SimpleTooltip title={instance.running || instance.canStart ? label : instance.detail}>
      <span>
        <Button
          size="icon"
          variant="ghost"
          className="size-6"
          disabled={!instance.running && !instance.canStart}
          loading={busy}
          aria-label={label}
          onClick={() => onToggle(instance)}
        >
          {instance.running ? <Square /> : <Play />}
        </Button>
      </span>
    </SimpleTooltip>
  );
}

/**
 * The extra copies of one agent. Each is started against a state directory of
 * its own, so each signs in separately and the two run side by side without
 * seeing each other's sessions.
 */
export function AgentInstances({agent, enabled = true}: {agent: Agent; enabled?: boolean}) {
  const controls = useAgentInstances(agent.agentId, enabled && agent.supportsInstances === true);

  if (!agent.supportsInstances) {
    return null;
  }

  const columns: Column<AgentInstance>[] = [
    {
      title: i18next.t("general:Name"),
      key: "instance",
      dataIndex: "instance",
      render: (_value, record) => <InstanceName instance={record} onRename={controls.rename} />,
    },
    {
      title: i18next.t("agent:Account"),
      key: "account",
      render: (_value, record) =>
        record.account && accountLabel(record.account) ? (
          <div className="flex flex-col">
            <span>{accountLabel(record.account)}</span>
            {record.account.email && record.account.email !== accountLabel(record.account) ? (
              <code className="text-muted-foreground text-xs">{record.account.email}</code>
            ) : null}
          </div>
        ) : (
          <SimpleTooltip title={i18next.t("agent:Not signed in hint")}>
            <span className="text-muted-foreground inline-flex items-center gap-1">
              <UserPlus className="size-3.5" />
              {i18next.t("agent:Not signed in")}
            </span>
          </SimpleTooltip>
        ),
    },
    {
      title: i18next.t("agent:Run Status"),
      key: "running",
      render: (_value, record) =>
        record.running ? (
          <SimpleTooltip title={`${i18next.t("agent:Processes")}: ${record.pids.join(", ")}`}>
            <span>
              <Badge variant="success">{i18next.t("agent:Running")}</Badge>
            </span>
          </SimpleTooltip>
        ) : (
          <SimpleTooltip title={record.detail}>
            <span>
              <Badge variant="muted">{i18next.t("agent:Stopped")}</Badge>
            </span>
          </SimpleTooltip>
        ),
    },
    {
      title: i18next.t("agent:State directory"),
      key: "dataDir",
      dataIndex: "dataDir",
      ellipsis: true,
      render: (value: string) => <CodeText copyable>{value}</CodeText>,
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      render: (_value, record) => (
        <div className="flex items-center gap-2">
          <InstanceRunButton
            instance={record}
            busy={controls.busyName === record.name}
            onToggle={controls.toggleRunning}
          />
          <ConfirmDialog
            title={`${i18next.t("agent:Remove instance")}: ${instanceLabel(record)}?`}
            description={i18next.t("agent:Remove instance hint")}
            confirmText={i18next.t("agent:Remove instance")}
            variant="destructive"
            onConfirm={() => controls.remove(record)}
          >
            <Button size="icon" variant="ghost" className="size-6">
              <Trash2 />
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  return (
    <DataTable
      title={i18next.t("agent:Instances")}
      description={i18next.t("agent:Instances hint")}
      columns={columns}
      dataSource={controls.instances}
      rowKey={instance => instance.name}
      loading={controls.loading}
      pageSize={0}
      emptyText={i18next.t("agent:No instances yet")}
      toolbar={
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => controls.reload(true)}
            loading={controls.loading}
          >
            <RefreshCw />
            {i18next.t("agent:Scan")}
          </Button>
          <AddInstanceButton agent={agent} controls={controls} />
        </div>
      }
    />
  );
}

/**
 * The instances of one agent as a grid card shows them: an icon for the section,
 * one line per copy with the account it is signed in to, and the control that
 * starts or stops it. The card has no room for words about any of it, so the
 * names live in the tooltips and the renaming on the detail page.
 */
export function AgentCardInstances({
  agent,
  controls,
}: {
  agent: Agent;
  controls: AgentInstanceControls;
}) {
  if (!agent.supportsInstances) {
    return null;
  }

  const instances = instancesOf(controls.instances, agent);
  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-2">
        <SimpleTooltip title={i18next.t("agent:Instances")}>
          <span className="text-muted-foreground flex items-center gap-1 text-xs">
            <Users className="size-3.5" />
            {instances.length > 0 ? instances.length : null}
          </span>
        </SimpleTooltip>
        <AddInstanceButton agent={agent} controls={controls} compact />
      </div>

      {instances.map(instance => (
        <div key={instance.name} className="flex items-center gap-2">
          <span
            className={cn(
              "size-1.5 shrink-0 rounded-full",
              instance.running ? "bg-success" : "bg-muted-foreground/40",
            )}
          />
          <SimpleTooltip title={instance.account ? instance.account.email : instance.dataDir}>
            <span className="min-w-0 flex-1 truncate text-xs">{instanceLabel(instance)}</span>
          </SimpleTooltip>
          <InstanceRunButton
            instance={instance}
            busy={controls.busyName === instance.name}
            onToggle={controls.toggleRunning}
          />
        </div>
      ))}
    </div>
  );
}
