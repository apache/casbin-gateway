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
import {Bot, ChevronRight, UserRound} from "lucide-react";
import i18next from "i18next";

import {AgentIcon} from "@/components/AgentIcon";
import {AgentCardInstances} from "@/components/AgentInstances";
import {AgentUsageTrend, hasUsageTrend} from "@/components/AgentUsageTrend";
import {AgentUpdateBadge} from "@/components/AgentVersionDialog";
import {RunButton, RunDot} from "@/components/AgentRunControl";
import {ProviderIcon} from "@/components/ProviderIcon";
import {QuotaBadge} from "@/components/ProviderQuota";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Card} from "@/components/ui/card";
import {Progress} from "@/components/ui/progress";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {Switch} from "@/components/ui/switch";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {
  agentBuiltin,
  agentCanUse,
  agentDetailPath,
  builtinProvider,
  directMode,
  type AgentInstanceControls,
} from "@/lib/agents";
import {providerIdOf, providerProtocol} from "@/lib/providers";
import {formatCost, formatTokens} from "@/lib/usage";
import type {
  Agent,
  AgentAccount,
  AgentRuntime,
  AgentUpdate,
  AgentUsageStat,
  LlmAgentStat,
  Provider,
  ProviderHealth,
  ProviderQuota,
} from "@/types";

/** A hairline across the whole card, drawn out of a section that is padded. */
function Divider() {
  return <div className="border-border -mx-4 border-t" />;
}

/** One labelled line of the card, with the value pinned right. */
function Row({label, children}: {label: React.ReactNode; children: React.ReactNode}) {
  return (
    <div className="flex min-w-0 items-center justify-between gap-2">
      <span className="text-muted-foreground shrink-0 text-[11px]">{label}</span>
      {children}
    </div>
  );
}

/** One number of the card, and what it is. */
function Metric({
  label,
  value,
  hint,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  hint?: React.ReactNode;
}) {
  const shown = <span className="block truncate text-[13px] font-medium tabular-nums">{value}</span>;

  return (
    <div className="min-w-0">
      <span className="text-muted-foreground block truncate text-[11px]">{label}</span>
      {hint ? (
        <SimpleTooltip title={hint}>
          <span className="block min-w-0">{shown}</span>
        </SimpleTooltip>
      ) : (
        shown
      )}
    </div>
  );
}

/** The label for a signed-in account: its name, or its email when unnamed. */
export function accountLabel(account: AgentAccount) {
  return account.name || account.email || "";
}

/** The signed-in account of an agent, shown under its path. */
function AccountLine({account}: {account?: AgentAccount}) {
  const label = account ? accountLabel(account) : "";
  if (!label) {
    return null;
  }
  const title = account?.email && account.email !== label ? `${label} · ${account.email}` : label;

  return (
    <SimpleTooltip title={title}>
      <p className="text-muted-foreground flex items-center gap-1 truncate text-[11px]">
        <UserRound className="size-3 shrink-0" />
        <span className="truncate">{label}</span>
      </p>
    </SimpleTooltip>
  );
}

/** What the vendor of the bound provider says is left on the plan. */
function PlanUsage({quota}: {quota?: ProviderQuota}) {
  // A provider nobody can read a balance from has nothing to show here.
  if (!quota || (!quota.supported && quota.error === "")) {
    return null;
  }

  const percent =
    quota.total !== null && quota.total > 0 && quota.used !== null
      ? Math.min(100, (quota.used / quota.total) * 100)
      : null;

  return (
    <div className="space-y-1.5">
      <Row label={i18next.t("provider:Plan usage")}>
        <QuotaBadge quota={quota} />
      </Row>
      {percent === null ? null : (
        <Progress
          value={percent}
          tone={percent >= 90 ? "danger" : percent >= 75 ? "warning" : "default"}
          className="h-1"
        />
      )}
    </div>
  );
}

/**
 * One agent installed on this machine, as the card the home page is built out
 * of: what it is on now, what it has spent there, and the controls that change
 * both. Picking another provider is the one click the page exists for.
 */
export function AgentGridCard({
  agent,
  agents,
  providers,
  health,
  quota,
  stats,
  usage,
  status,
  update,
  instances,
  recording,
  busy,
  runBusy,
  onEnable,
  onToggleRunning,
  onTogglePatch,
}: {
  agent: Agent;
  /** Every installation, which is what decides whether the link needs a path. */
  agents: Agent[];
  providers: Provider[];
  health: ProviderHealth[];
  quota?: ProviderQuota;
  stats?: LlmAgentStat;
  /**
   * What this agent's own transcripts say it spent, which counts the requests
   * it made straight to its vendor as well as the ones Gateway relayed.
   */
  usage?: AgentUsageStat;
  status?: AgentRuntime;
  /** The release check for this installation, absent until it lands. */
  update?: AgentUpdate;
  /** The extra copies of this agent, for the agents that can run more than one. */
  instances?: AgentInstanceControls;
  /** False while llmRecordMode is off, when a zero would be a lie. */
  recording: boolean;
  busy: boolean;
  runBusy: boolean;
  onEnable: (providerId: string) => void;
  onToggleRunning: (agent: Agent, running: boolean) => void;
  onTogglePatch: () => void;
}) {
  const bound = providers.find(provider => providerIdOf(provider) === agent.provider);
  const boundHealth = health.find(item => item.provider === agent.provider);
  // An agent bound directly reads the provider's own answers, so only the ones
  // it can speak to are offered; through the gateway they all are. Whatever it
  // is on stays listed either way, so the box shows where it points today.
  const options = providers.filter(
    provider => agentCanUse(agent, provider) || providerIdOf(provider) === agent.provider,
  );
  const patchAction = i18next.t(`agent:${agent.patched ? "Unpatch" : "Patch"}`);
  const dash = <span className="text-muted-foreground">-</span>;
  const offHint = recording ? undefined : i18next.t("llm:Recording is off");
  // An agent's own transcript accounts for every request it made, the ones that
  // never came near Gateway included, so it is the fuller of the two and wins
  // wherever it has anything to say. The relayed totals answer for the agents
  // that keep no transcript, and for the failures a transcript cannot show.
  const spent = usage && usage.requests > 0 ? usage : undefined;
  const sourceHint = spent ? i18next.t("agent:Read from the agent's own logs") : offHint;
  // Off is only the truth about the relayed totals; a transcript is written
  // whether or not Gateway is recording anything.
  const counted = spent !== undefined || recording;
  const costHint =
    spent && spent.unpriced > 0
      ? i18next
        .t("llm:{count} of these requests have no list price")
        .replace("{count}", spent.unpriced.toLocaleString())
      : sourceHint;
  const lastModel = spent?.lastModel || stats?.lastModel;
  const lastTime = spent?.lastTime || stats?.lastTime;

  return (
    <Card className="hover:border-foreground/25 gap-0 overflow-hidden py-0 shadow-xs transition-colors">
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex items-start gap-2.5">
          <AgentIcon
            agent={agent.agentId || agent.name}
            size={22}
            fallback={<Bot className="text-muted-foreground size-5" />}
          />
          <div className="min-w-0 flex-1">
            <Link
              to={agentDetailPath(agent, agents)}
              className="block truncate text-sm font-medium hover:underline"
            >
              {agent.name}
            </Link>
            <SimpleTooltip title={agent.path}>
              <p className="text-muted-foreground/80 truncate font-mono text-[11px]">
                <span className="tabular-nums">{agent.version || i18next.t("agent:Unknown")}</span>
                <span className="mx-1">·</span>
                {agent.path}
              </p>
            </SimpleTooltip>
            <AgentUpdateBadge update={update} className="mt-1 text-[11px]" />
            <AccountLine account={agent.account} />
          </div>
          <RunDot status={status} />
        </div>

        <Divider />

        <div className="space-y-1.5">
          <Row label={i18next.t("agent:Provider")}>
            <span className="flex min-w-0 items-center gap-2">
              {boundHealth && !boundHealth.healthy ? (
                <SimpleTooltip title={boundHealth.lastError}>
                  <span className="text-warning truncate text-[11px]">
                    {i18next.t("agent:Cooling down")}
                  </span>
                </SimpleTooltip>
              ) : null}
              {bound === undefined ? null : (
                <span className="text-muted-foreground truncate text-[11px]">
                  {i18next.t(agent.mode === directMode ? "agent:Direct" : "agent:Gateway")}
                </span>
              )}
            </span>
          </Row>

          <Select
            value={agent.provider === "" ? builtinProvider : agent.provider}
            disabled={busy}
            onValueChange={value => onEnable(value === builtinProvider ? "" : value)}
          >
            <SelectTrigger size="sm" className="w-full text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={builtinProvider}>
                {agentBuiltin(agent)}
                <span className="text-muted-foreground ml-2 text-xs">
                  {i18next.t("agent:Built-in")}
                </span>
              </SelectItem>
              {options.map(provider => (
                <SelectItem key={providerIdOf(provider)} value={providerIdOf(provider)}>
                  <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} size={16} />
                  {provider.displayName || provider.name}
                  <span className="text-muted-foreground ml-2 text-xs">
                    {providerProtocol(provider.type)}
                  </span>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Row label={i18next.t("agent:Model")}>
            {lastModel ? (
              <SimpleTooltip
                title={`${i18next.t("agent:Last activity")}: ${new Date(lastTime ?? "").toLocaleString()}`}
              >
                <span className="min-w-0 truncate font-mono text-[11px]">{lastModel}</span>
              </SimpleTooltip>
            ) : (
              <span className="text-muted-foreground text-[11px]">-</span>
            )}
          </Row>
        </div>

        <Divider />

        <div className="space-y-3">
          {/* The shape of the spend wherever the transcripts carry a history,
              and the bare totals for the agents whose only account is what
              Gateway relayed - that source keeps no per-day breakdown. */}
          {spent && hasUsageTrend(spent) ? (
            <AgentUsageTrend
              stat={spent}
              hints={{tokens: sourceHint, requests: sourceHint, cost: costHint}}
            />
          ) : (
            <div className="grid grid-cols-3 gap-1.5">
              <Metric
                label={i18next.t("llm:Requests")}
                value={counted ? ((spent ?? stats)?.requests ?? 0).toLocaleString() : dash}
                hint={
                  sourceHint ??
                  (stats && stats.failed > 0
                    ? `${stats.failed.toLocaleString()} ${i18next.t("llm:failed")}`
                    : undefined)
                }
              />
              <Metric
                label={i18next.t("llm:Tokens")}
                value={counted ? formatTokens(spent?.totalTokens ?? stats?.tokens ?? 0) : dash}
                hint={sourceHint}
              />
              <Metric
                label={i18next.t("llm:Cost")}
                value={counted ? formatCost(spent?.cost ?? stats?.cost ?? 0) : dash}
                hint={costHint}
              />
            </div>
          )}

          <PlanUsage quota={quota} />

          {instances ? <AgentCardInstances agent={agent} controls={instances} /> : null}
        </div>
      </div>

      <div className="bg-muted/40 mt-auto flex flex-wrap items-center gap-x-2 gap-y-1 border-t px-3 py-2">
        <RunButton
          agent={agent}
          status={status}
          busy={runBusy}
          className="h-7 px-2 text-xs"
          onToggle={onToggleRunning}
        />

        <label className="text-muted-foreground flex items-center gap-1.5 text-[11px]">
          {i18next.t("agent:Monitored")}
          {agent.supported ? (
            <ConfirmDialog
              title={`${patchAction} ${agent.name}?`}
              description={[agent.notice, agent.followup].filter(Boolean).join(" ") || undefined}
              confirmText={patchAction}
              variant={agent.patched ? "destructive" : "default"}
              onConfirm={onTogglePatch}
            >
              {/* The dialog owns the click, so the switch only ever mirrors
                  what the last scan reported. */}
              <Switch
                checked={agent.patched}
                disabled={busy}
                aria-label={patchAction}
                onCheckedChange={() => undefined}
              />
            </ConfirmDialog>
          ) : (
            <SimpleTooltip title={agent.detail || i18next.t("agent:Not supported")}>
              <span>
                <Switch checked={false} disabled aria-label={i18next.t("agent:Patch")} />
              </span>
            </SimpleTooltip>
          )}
        </label>

        <Link
          to={agentDetailPath(agent, agents)}
          className="text-muted-foreground hover:text-foreground ml-auto inline-flex items-center text-xs"
        >
          {i18next.t("agent:Details")}
          <ChevronRight className="size-3.5" />
        </Link>
      </div>
    </Card>
  );
}
