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
import {Bot, Container, Plus, RefreshCw, Table2} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {AgentCatalog} from "@/components/AgentCatalog";
import {AuthenticityOverview} from "@/components/AuthenticityOverview";
import {AgentGridCard} from "@/components/AgentGridCard";
import {OnboardingButton, OnboardingChecklist, useOnboarding} from "@/components/OnboardingChecklist";
import {EmptyState, ErrorState} from "@/components/shared/empty-state";
import {Loading} from "@/components/shared/loading";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {
  agentKey,
  runtimeOf,
  useAgentInstall,
  useAgentInstances,
  useAgents,
  useAgentUpdates,
  updateOf,
  usageOf,
} from "@/lib/agents";
import type {
  Account,
  AgentUsage,
  LlmAgentStat,
  Provider,
  ProviderHealth,
  ProviderQuota,
} from "@/types";

/** How often the numbers on the cards are read again. */
const refreshMs = 10000;

/**
 * How often the transcripts are added up again. Reading them walks every
 * session file on the machine, so it is asked for far less often than the
 * counters the proxy keeps in memory.
 */
const usageRefreshMs = 60000;

/**
 * The home screen: one card per agent installed on this machine. Each card
 * carries what that agent is on, what it has spent there and whether it is
 * running, and the provider box switches it over in one click — the routing is
 * stored and, where Gateway writes the agent's configuration file, that file is
 * rewritten with it.
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
  const [quotas, setQuotas] = React.useState<ProviderQuota[]>([]);
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

    // What is already known paints at once; the refresh then asks only the
    // vendors whose answer has gone stale, so a page view costs at most one
    // request per provider every ten minutes.
    ProviderBackend.getProviderQuotas()
      .then(res => setQuotas(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined)
      .then(() => ProviderBackend.refreshProviderQuotas("", false))
      .then(res => setQuotas(res?.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined);
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

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agent:Agents on this machine")}
        description={account.hostname}
        actions={
          <>
            {loaded && !onboarding.open ? <OnboardingButton onboarding={onboarding} /> : null}
            <Button asChild variant="ghost">
              <Link to="/agents">
                <Table2 />
                {i18next.t("agent:Advanced view")}
              </Link>
            </Button>
            {rescan}
          </>
        }
      />

      {loaded && onboarding.open ? <OnboardingChecklist onboarding={onboarding} /> : null}

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

      {/* Whether the keys behind these agents reach what they are sold as. It
          is measured without being asked for, so it belongs above the fold
          rather than behind a button on another page. */}
      {providers.length > 0 ? <AuthenticityOverview providers={providers} /> : null}

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
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {agents.map(agent => (
            <AgentGridCard
              key={agentKey(agent)}
              agent={agent}
              agents={agents}
              providers={providers}
              health={health}
              quota={quotas.find(item => item.provider === agent.provider)}
              stats={stats.find(item => item.agent === agent.agentId)}
              usage={usageOf(usage, agent)}
              status={runtimeOf(runtime, agent)}
              update={updateOf(updates.updates, agent)}
              instances={instances}
              recording={recording}
              busy={busyKey === agentKey(agent)}
              runBusy={runBusyKey === agentKey(agent)}
              onEnable={providerId => activateProvider(agent, providerId)}
              onLocated={() => scan(true)}
              onToggleRunning={toggleRunning}
              onTogglePatch={() => togglePatch(agent)}
            />
          ))}
        </div>
      )}

      {/* What is not here yet, so an empty machine has somewhere to go. */}
      <AgentCatalog agents={agents} enabled={scanned} installer={installer} onLocated={() => scan(true)} />
    </PageContainer>
  );
}
