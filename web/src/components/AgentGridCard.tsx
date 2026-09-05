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
import {Bot, ChevronRight} from "lucide-react";
import i18next from "i18next";

import {AgentIcon} from "@/components/AgentIcon";
import {AgentCardInstances} from "@/components/AgentInstances";
import {RunButton, RunDot} from "@/components/AgentRunControl";
import {ProviderIcon} from "@/components/ProviderIcon";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Card} from "@/components/ui/card";
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
  type AgentInstanceControls,
  type AgentSpend,
} from "@/lib/agents";
import {providerIdOf, providerProtocol} from "@/lib/providers";
import {formatCost} from "@/lib/usage";
import type {Agent, AgentAccount, AgentRuntime, AgentUpdate, Provider, ProviderHealth} from "@/types";

/** The label for a signed-in account: its name, or its email when unnamed. */
export function accountLabel(account: AgentAccount) {
  return account.name || account.email || "";
}

/**
 * One agent installed on this machine, as the card the home page is built out
 * of. It carries three things and no more: which agent it is, what it has spent,
 * and the provider box that switches it over - the one click the page exists
 * for. Its account, its instances, its plan balance and its configuration are a
 * click deeper, on the page named at the bottom right, because eighteen cards
 * carrying all of that is a page nobody reads.
 */
export function AgentGridCard({
  agent,
  agents,
  providers,
  health,
  spend,
  status,
  update,
  instances,
  recording,
  busy,
  runBusy,
  onEnable,
  onLocated,
  onToggleRunning,
  onTogglePatch,
}: {
  agent: Agent;
  /** Every installation, which is what decides whether the link needs a path. */
  agents: Agent[];
  providers: Provider[];
  health: ProviderHealth[];
  /** What this agent spent, from whichever of the two accounts has it. */
  spend: AgentSpend;
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
  /** Called once a program is picked for an agent nothing here can start. */
  onLocated?: () => void;
  onToggleRunning: (agent: Agent, running: boolean) => void;
  onTogglePatch: () => void;
}) {
  const boundHealth = health.find(item => item.provider === agent.provider);
  // An agent bound directly reads the provider's own answers, so only the ones
  // it can speak to are offered; through the gateway they all are. Whatever it
  // is on stays listed either way, so the box shows where it points today.
  const options = providers.filter(
    provider => agentCanUse(agent, provider) || providerIdOf(provider) === agent.provider,
  );
  const patchAction = i18next.t(`agent:${agent.patched ? "Unpatch" : "Patch"}`);
  const detail = agentDetailPath(agent, agents);

  // Off is only the truth about the relayed totals; a transcript is written
  // whether or not Gateway is recording anything.
  const counted = spend.own !== undefined || recording;
  const sourceHint = spend.own
    ? i18next.t("agent:Read from the agent's own logs")
    : recording
      ? undefined
      : i18next.t("llm:Recording is off");
  const costHint =
    spend.own && spend.own.unpriced > 0
      ? i18next
        .t("llm:{count} of these requests have no list price")
        .replace("{count}", spend.own.unpriced.toLocaleString())
      : sourceHint;
  const subtitle = [agent.version || i18next.t("agent:Unknown"), agent.account ? accountLabel(agent.account) : ""]
    .filter(Boolean)
    .join(" · ");

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
            <div className="flex min-w-0 items-center gap-1.5">
              <Link to={detail} className="truncate text-sm font-medium hover:underline">
                {agent.name}
              </Link>
              {/* A waiting release is a dot rather than a line of its own: it is
                  worth noticing and not worth a sentence on every card. */}
              {update?.available ? (
                <SimpleTooltip title={`${i18next.t("agent:New version")} ${update.latest}`}>
                  <span className="bg-warning size-1.5 shrink-0 rounded-full" />
                </SimpleTooltip>
              ) : null}
            </div>
            <SimpleTooltip title={agent.path}>
              <p className="text-muted-foreground/80 truncate text-[11px]">{subtitle}</p>
            </SimpleTooltip>
          </div>
          <RunDot status={status} />
        </div>

        {/* The one number. An agent nobody has run says so in words: a zero
            there reads as free rather than as unused. */}
        <SimpleTooltip title={costHint}>
          <div className="flex min-w-0 flex-col gap-0.5">
            {counted && spend.requests > 0 ? (
              <>
                <span className="text-lg leading-none font-semibold tabular-nums">
                  {formatCost(spend.cost)}
                </span>
                <span className="text-muted-foreground min-w-0 truncate text-[11px] tabular-nums">
                  {i18next.t("llm:{count} requests").replace("{count}", spend.requests.toLocaleString())}
                  {spend.lastModel ? <span className="font-mono"> · {spend.lastModel}</span> : null}
                </span>
              </>
            ) : (
              <>
                <span className="text-muted-foreground text-lg leading-none font-semibold">-</span>
                <span className="text-muted-foreground truncate text-[11px]">
                  {i18next.t(counted ? "agent:Never used" : "llm:Recording is off")}
                </span>
              </>
            )}
          </div>
        </SimpleTooltip>

        {boundHealth && !boundHealth.healthy ? (
          <SimpleTooltip title={boundHealth.lastError}>
            <span className="text-warning -mb-1 truncate text-[11px]">
              {i18next.t("agent:Cooling down")}
            </span>
          </SimpleTooltip>
        ) : null}

        {/* The bottom block of the card: where it points, and the copies that
            point there. Held together so the boxes line up across a row of
            cards whose spend lines are different heights. */}
        <div className="mt-auto flex flex-col gap-2">
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
                <AgentIcon agent={agent.agentId || agent.name} size={16} fallback={<Bot className="size-4" />} />
                {agentBuiltin(agent)}
                <span className="text-muted-foreground ml-2 text-xs">{i18next.t("agent:Built-in")}</span>
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

          {instances ? <AgentCardInstances agent={agent} controls={instances} /> : null}
        </div>
      </div>

      <div className="bg-muted/40 mt-auto flex items-center gap-2 border-t px-3 py-1.5">
        <RunButton
          agent={agent}
          status={status}
          busy={runBusy}
          className="h-7 px-2 text-xs"
          onLocated={onLocated}
          onToggle={onToggleRunning}
        />

        <label className="text-muted-foreground flex min-w-0 items-center gap-1.5 text-[11px]">
          <span className="truncate">{i18next.t("agent:Monitored")}</span>
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

        <SimpleTooltip title={i18next.t("agent:Details")}>
          <Link
            to={detail}
            aria-label={i18next.t("agent:Details")}
            className="text-muted-foreground hover:text-foreground ml-auto inline-flex shrink-0 items-center"
          >
            <ChevronRight className="size-4" />
          </Link>
        </SimpleTooltip>
      </div>
    </Card>
  );
}
