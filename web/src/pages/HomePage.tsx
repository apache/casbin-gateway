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
import {Bot, Container, ListChecks, MoreHorizontal, Plus, RefreshCw, Table2} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {AgentCatalog} from "@/components/AgentCatalog";
import {CcSwitchBanner} from "@/components/CcSwitchBanner";
import {AgentGridCard} from "@/components/AgentGridCard";
import {HomeSummary} from "@/components/HomeSummary";
import {OnboardingChecklist, useOnboarding} from "@/components/OnboardingChecklist";
import {EmptyState, ErrorState} from "@/components/shared/empty-state";
import {Fold} from "@/components/shared/fold";
import {Loading} from "@/components/shared/loading";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  agentKey,
  agentSpend,
  runtimeOf,
  useAgentInstall,
  useAgentInstances,
  useAgents,
  useAgentUpdates,
  updateOf,
  type AgentSpend,
} from "@/lib/agents";
import type {Agent, Account, AgentUsage, LlmAgentStat, Provider, ProviderHealth} from "@/types";

/** How often the numbers on the cards are read again. */
const refreshMs = 10000;

/**
 * How often the transcripts are added up again. Reading them walks every
 * session file on the machine, so it is asked for far less often than the
 * counters the proxy keeps in memory.
 */
const usageRefreshMs = 60000;

/** Whether this installation is worth a card before anyone asks for one. */
function inUse(agent: Agent, spend: AgentSpend, running: boolean) {
  return running || agent.provider !== "" || spend.requests > 0;
}

/**
 * The home screen. It answers two questions and leaves the rest to the pages
 * below it: what did this machine spend, and what is each agent on. Everything
 * an agent carries beyond that - its path, its account, its extra copies, its
 * plan balance - is a click away on its own page, because eighteen cards saying
 * all of it at once is a page that gets scrolled past rather than read.
 */
export default function HomePage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {
    agents,
    loading,
    error,
    busyKey,
    scanned,
    inContainer,
    runtime,
    runBusyKey,
    scan,
    loadRuntime,
    toggleRunning,
    togglePatch,
    activateProvider,
  } = useAgents(isAdmin);
  // Which cards have a newer release waiting, looked up in the registries.
  const updates = useAgentUpdates(isAdmin);
  // Installs the agents this machine is missing, listed under the cards.
  const installer = useAgentInstall(isAdmin, () => {
    scan(true);
    updates.reload(true);
  });
  // Every agent's extra copies in one listing, so a page of cards costs one
  // request rather than one per card.
  const instances = useAgentInstances("", isAdmin);
  const [providers, setProviders] = React.useState<Provider[]>([]);
  const [health, setHealth] = React.useState<ProviderHealth[]>([]);
  const [stats, setStats] = React.useState<LlmAgentStat[]>([]);
  const [usage, setUsage] = React.useState<AgentUsage>();
  // Nothing is recorded until it is asked for, and until it is a zero on a card
  // would read as "this agent relayed nothing" rather than "nobody counted".
  const [recording, setRecording] = React.useState(true);
  // False until the first listing lands, so "no providers" never flashes.
  const [loaded, setLoaded] = React.useState(false);
  // A listing that failed must not read as "you have no providers": that says
  // the accounts holding your API keys are gone when the request simply
  // dropped.
  const [providerError, setProviderError] = React.useState("");
  const loadProviders = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    ProviderBackend.getProviders(account.name)
      .then(res => {
        if (res.status === "ok") {
          setProviders(res.data ?? []);
          setProviderError("");
        } else {
          setProviderError(res.msg || i18next.t("provider:Failed to get providers"));
        }
      })
      .catch(failure => setProviderError(failure.message || String(failure)))
      .then(() => setLoaded(true));
  }, [isAdmin, account.name]);

  React.useEffect(() => {
    loadProviders();
  }, [loadProviders]);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    LlmRecordBackend.getLlmRecordStatus()
      .then(res => setRecording(res.status !== "ok" || (res.data?.mode ?? "off") !== "off"))
      .catch(() => undefined);
  }, [isAdmin]);

  // What the proxy has seen changes as requests are relayed, so the health and
  // the per-agent totals are polled rather than read once.
  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }

    const load = () => {
      ProviderBackend.getProviderHealth()
        .then(res => setHealth(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setHealth([]));
      LlmRecordBackend.getLlmAgentStats()
        .then(res => setStats(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setStats([]));
    };

    load();
    const interval = setInterval(load, refreshMs);
    return () => clearInterval(interval);
  }, [isAdmin]);

  // What the agents wrote down themselves, which is the only account of the
  // requests that never went through Gateway - and the only one there is at all
  // before a provider is bound.
  React.useEffect(() => {
    if (!isAdmin) {
      return undefined;
    }

    const load = () => {
      AgentBackend.getAgentUsage()
        .then(res => setUsage(res.status === "ok" ? (res.data ?? undefined) : undefined))
        .catch(() => undefined);
    };

    load();
    const interval = setInterval(load, usageRefreshMs);
    return () => clearInterval(interval);
  }, [isAdmin]);

  // Held here rather than inside the card, so the header can offer the guide
  // back to whoever closed it.
  const onboarding = useOnboarding({providers: providers, agents: agents, stats: stats});

  // One reading of each agent, shared by the cards, the strip above them and the
  // split below: the totals cannot disagree with the cards they add up.
  const rows = agents
    .map(agent => {
      const status = runtimeOf(runtime, agent);
      const spend = agentSpend(agent, usage, stats);
      return {agent: agent, status: status, spend: spend, using: inUse(agent, spend, status?.running === true)};
    })
    // Busiest first, and whatever is running before whatever is not: a card is
    // worth its place by what it is doing, not by where it was installed.
    .sort((left, right) => {
      const run = Number(right.status?.running ?? false) - Number(left.status?.running ?? false);
      return run !== 0 ? run : right.spend.cost - left.spend.cost || right.spend.requests - left.spend.requests;
    });
  const using = rows.filter(row => row.using);
  const idle = rows.filter(row => !row.using);
  const running = rows.filter(row => row.status?.running).length;
  const total = rows.reduce(
    (sum, row) => ({requests: sum.requests + row.spend.requests, cost: sum.cost + row.spend.cost}),
    {requests: 0, cost: 0},
  );

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const refresh = () => {
    scan(true);
    loadRuntime(true);
    loadProviders();
    instances.reload(true);
  };

  const rescan = (
    <Button variant="outline" onClick={refresh} loading={loading}>
      <RefreshCw />
      {i18next.t("agent:Scan")}
    </Button>
  );

  const cards = (rendered: typeof rows) => (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {rendered.map(row => (
        <AgentGridCard
          key={agentKey(row.agent)}
          agent={row.agent}
          agents={agents}
          providers={providers}
          health={health}
          spend={row.spend}
          status={row.status}
          update={updateOf(updates.updates, row.agent)}
          instances={instances}
          recording={recording}
          busy={busyKey === agentKey(row.agent)}
          runBusy={runBusyKey === agentKey(row.agent)}
          onEnable={providerId => activateProvider(row.agent, providerId)}
          onLocated={() => scan(true)}
          onToggleRunning={toggleRunning}
          onTogglePatch={() => togglePatch(row.agent)}
        />
      ))}
    </div>
  );

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agents on this machine")}
        description={account.hostname}
        actions={
          <>
            {/* Two controls, not four: the rescan is the one anybody presses,
                and the rest are places rather than actions. */}
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon" aria-label={i18next.t("general:More")}>
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem asChild>
                  <Link to="/agents">
                    <Table2 />
                    {i18next.t("agent:Advanced view")}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuItem onSelect={onboarding.reopen}>
                  <ListChecks />
                  {i18next.t("agent:Getting started")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            {rescan}
          </>
        }
      />

      {loaded && onboarding.open ? <OnboardingChecklist onboarding={onboarding} /> : null}

      {/* Somebody arriving from CC Switch has all of this set up already, and
          the import page is the first thing they want rather than a form. */}
      <CcSwitchBanner />

      {/* What the machine adds up to, including whether the keys behind it are
          what they were sold as. Every figure here is otherwise only reachable
          by reading every card below. */}
      {scanned && agents.length > 0 ? (
        <HomeSummary
          total={total}
          agents={agents.length}
          running={running}
          providers={providers}
          loaded={usage !== undefined}
        />
      ) : null}

      {/* With no agents to show, the failure is the whole page and is rendered
          below instead; here it sits above the cards that are still on screen. */}
      {error !== "" && agents.length > 0 ? <MessageAlert title={error} /> : null}

      {providerError !== "" ? (
        <MessageAlert
          title={i18next.t("provider:Failed to get providers")}
          description={providerError}
          action={
            <Button variant="outline" size="sm" onClick={loadProviders}>
              <RefreshCw />
              {i18next.t("general:Retry")}
            </Button>
          }
        />
      ) : loaded && providers.length === 0 ? (
        <MessageAlert
          variant="info"
          title={i18next.t("provider:No providers yet")}
          description={i18next.t("provider:No providers yet detail")}
          action={
            <Button asChild size="sm">
              <Link to="/providers">
                <Plus />
                {i18next.t("provider:New Provider")}
              </Link>
            </Button>
          }
        />
      ) : null}

      {!scanned ? (
        <Loading tip={i18next.t("agent:Scan")} />
      ) : error !== "" && agents.length === 0 ? (
        // A scan that failed is not a machine without agents, and telling
        // someone to install one they already have is the wrong instruction.
        <Card className="py-0">
          <ErrorState title={i18next.t("agent:Failed to scan agents")} error={error} onRetry={refresh} />
        </Card>
      ) : agents.length === 0 ? (
        <Card className="py-0">
          <EmptyState
            icon={inContainer ? Container : Bot}
            title={i18next.t(inContainer ? "agent:Running in a container" : "agent:No supported agents found")}
            description={i18next.t(
              inContainer
                ? "agent:Running in a container detail"
                : "agent:Install an AI agent on this machine, then scan again",
            )}
            action={rescan}
          />
        </Card>
      ) : (
        <>
          {/* Everything bound, running or spending. On a machine where nothing
              is any of those the split has nothing to hide, so it is skipped. */}
          {cards(using.length > 0 ? using : rows)}

          {using.length > 0 && idle.length > 0 ? (
            <Fold
              title={i18next
                .t("agent:{count} agents not in use")
                .replace("{count}", String(idle.length))}
            >
              {cards(idle)}
            </Fold>
          ) : null}
        </>
      )}

      {/* What is not here yet, so an empty machine has somewhere to go. */}
      <AgentCatalog
        agents={agents}
        enabled={scanned}
        defaultOpen={agents.length === 0}
        installer={installer}
        onLocated={() => scan(true)}
      />
    </PageContainer>
  );
}
