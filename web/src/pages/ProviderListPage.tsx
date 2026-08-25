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
import {useNavigate} from "react-router-dom";
import {
  ChevronDown,
  ExternalLink,
  Plug,
  Plus,
  RefreshCw,
  Wallet,
} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {ProviderIconField} from "@/components/ProviderIcon";
import {ProviderGridCard} from "@/components/ProviderGridCard";
import {ProviderModelsField} from "@/components/ProviderModelsField";
import {ProviderSourcePicker, sourceTitle} from "@/components/ProviderSourcePicker";
import {ProviderTestField, useProviderTest} from "@/components/ProviderTestField";
import {type SortOrder} from "@/components/shared/data-table";
import {ErrorState} from "@/components/shared/empty-state";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {SearchSelect, SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {
  authProvider,
  authClient,
  baseUrlPlaceholder,
  baseUrlPresets,
  customSource,
  providerIdOf,
  providerSlug,
  providerSources,
  usesClientAuth,
  type ProviderSource,
} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {Account, Agent, Provider, ProviderHealth, ProviderQuota} from "@/types";

function newProvider(owner: string, label = "New Provider"): Provider {
  return {
    owner: owner,
    // The server appends a number when this is taken, so a second account with
    // the same vendor keeps a name someone can read.
    name: providerSlug(label),
    displayName: label,
    type: "openai",
    status: "enabled",
    models: [],
    priority: 0,
    baseUrl: "",
    apiKey: "",
    authMode: authProvider,
    icon: "",
    notes: "",
  };
}

/** What a picked source leaves to fill in, which for a subscription is nothing. */
function providerFromSource(owner: string, source: ProviderSource): Provider {
  return {...newProvider(owner, sourceTitle(source)), ...source.provider};
}

/**
 * The fields the source already answered. They stay reachable — a preset is a
 * starting point, not a lock — but out of the way of the one field, if any,
 * that is actually left to fill in.
 */
function Advanced({defaultOpen, children}: {defaultOpen: boolean; children: React.ReactNode}) {
  const [open, setOpen] = React.useState(defaultOpen);

  return (
    <div className="grid gap-4">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-sm"
      >
        <ChevronDown className={cn("size-3.5 transition-transform", open && "rotate-180")} />
        {i18next.t("general:Advanced")}
      </button>
      {open ? children : null}
    </div>
  );
}

export default function ProviderListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Provider[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [loaded, setLoaded] = React.useState(false);
  // Empty because there is nothing, or empty because the listing failed? The
  // two must never render the same: being told that an account which holds your
  // API keys has no providers is alarming when it is really a dropped request.
  const [error, setError] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(10);
  const [sort, setSort] = React.useState<{field: string; order: SortOrder}>({
    field: "",
    order: undefined,
  });
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Provider>(() => newProvider(account.name));
  // null while the dialog is still asking where the credentials come from.
  const [source, setSource] = React.useState<ProviderSource | null>(null);
  const [nameError, setNameError] = React.useState("");
  const [health, setHealth] = React.useState<ProviderHealth[]>([]);
  const [quotas, setQuotas] = React.useState<ProviderQuota[]>([]);
  const [refreshingQuotas, setRefreshingQuotas] = React.useState(false);
  const [agents, setAgents] = React.useState<Agent[]>([]);
  const [binding, setBinding] = React.useState("");
  const test = useProviderTest(form);

  const fetchProviders = React.useCallback(
    (nextPage = page, nextPageSize = pageSize, nextSort = sort) => {
      setLoading(true);
      ProviderBackend.getProviders(
        account.name,
        nextPage,
        nextPageSize,
        nextSort.order ? nextSort.field : "",
        nextSort.order ?? "",
      )
        .then(res => {
          if (res.status === "ok") {
            setData(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setSort(nextSort);
            setError("");
          } else {
            setError(res.msg || i18next.t("provider:Failed to get providers"));
          }
        })
        .catch(failure => setError(failure.message || String(failure)))
        .then(() => {
          setLoading(false);
          setLoaded(true);
        });
    },
    [account.name, page, pageSize, sort],
  );

  React.useEffect(() => {
    fetchProviders(1, 10, {field: "", order: undefined});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  // What the proxy has seen of each provider lives in memory and changes as
  // requests are relayed, so it is polled rather than read once.
  React.useEffect(() => {
    const load = () => {
      ProviderBackend.getProviderHealth()
        .then(res => setHealth(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setHealth([]));
    };

    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  // The balances are read from the cache first so the table fills in at once,
  // then refreshed for whichever provider has a stale answer.
  const loadQuotas = React.useCallback((force: boolean) => {
    setRefreshingQuotas(true);
    ProviderBackend.refreshProviderQuotas("", force)
      .then(res => setQuotas(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined)
      .then(() => setRefreshingQuotas(false));
  }, []);

  // Which agent a provider answers for is the thing this page is really about,
  // so the agents are loaded next to them. A non-admin cannot list them, and the
  // card simply leaves that line out.
  const loadAgents = React.useCallback(() => {
    if (!Setting.isAdminUser(account)) {
      return;
    }
    AgentBackend.getAgents()
      .then(res => setAgents(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setAgents([]));
  }, [account]);

  React.useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  React.useEffect(() => {
    ProviderBackend.getProviderQuotas()
      .then(res => setQuotas(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined)
      .then(() => loadQuotas(false));
  }, [loadQuotas]);

  const openAddDialog = (start?: ProviderSource) => {
    setForm(start ? providerFromSource(account.name, start) : newProvider(account.name));
    setSource(start ?? null);
    setNameError("");
    setAddOpen(true);
  };

  const setFormField = <K extends keyof Provider>(key: K, value: Provider[K]) => {
    setForm(prev => ({...prev, [key]: value}));
  };

  const pickSource = (picked: ProviderSource) => {
    setForm(providerFromSource(account.name, picked));
    setNameError("");
    setSource(picked);
  };

  // A vendor's link fills the same form a card would, and then stops: what it
  // carries — a base URL and often a key — is shown before anything is stored.
  const importLink = (link: string) => {
    return ProviderBackend.parseProviderLink(link)
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg || i18next.t("provider:This link cannot be read"));
          return;
        }
        const parsed = res.data;
        setForm({...newProvider(account.name, parsed.displayName), ...parsed, name: providerSlug(parsed.displayName)});
        setNameError("");
        setSource(providerSources.find(item => item.key === customSource) ?? null);
        setAddOpen(true);
      })
      .catch(error => Setting.showMessage("error", `${error}`));
  };

  // The upstream is probed before the provider is stored, so a key that was
  // pasted wrong is caught here rather than by the first agent that uses it.
  const submitProvider = () => {
    if (form.displayName.trim() === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    test.guard(addProvider);
  };

  const addProvider = () => {
    const displayName = form.displayName.trim();
    setAdding(true);
    ProviderBackend.addProvider({...form, displayName: displayName, name: providerSlug(displayName)})
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("provider:Provider added successfully"));
          setAddOpen(false);
          fetchProviders();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("provider:Failed to add")}: ${error}`);
      });
  };

  // Switching is one call: the server rewrites the configuration of an agent it
  // already switched, so nothing else has to be clicked afterwards.
  const bindAgent = (agent: Agent, provider: Provider) => {
    setBinding(agent.agentId);
    AgentBackend.updateAgentRouting(agent.agentId, {
      provider: providerIdOf(provider),
      fallbacks: [],
      mode: agent.mode || "gateway",
    })
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage(
            "success",
            `${i18next.t("provider:Switched")}: ${agent.name} → ${provider.displayName || provider.name}`,
          );
          loadAgents();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agent:Failed to update agent provider"));
          loadAgents();
        }
      })
      .catch(error => Setting.showMessage("error", `${error}`))
      .then(() => setBinding(""));
  };

  const deleteProvider = (provider: Provider) => {
    ProviderBackend.deleteProvider(provider)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to delete")}: ${res.msg}`);
          return;
        }
        Setting.showMessage("success", i18next.t("provider:Provider deleted successfully"));
        fetchProviders();
      })
      .catch(error =>
        Setting.showMessage("error", `${i18next.t("provider:Failed to delete")}: ${error}`),
      );
  };

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("provider:Providers")}
        description={i18next.t("provider:Page description")}
        actions={
          <Button onClick={() => openAddDialog()}>
            <Plus />
            {i18next.t("provider:New Provider")}
          </Button>
        }
      />

      {error !== "" && data.length === 0 ? (
        <Card>
          <ErrorState error={error} onRetry={() => fetchProviders()} />
        </Card>
      ) : loaded && error === "" && data.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Plug className="size-4" />
              {i18next.t("provider:No providers yet")}
            </CardTitle>
            <CardDescription>{i18next.t("provider:No providers yet detail")}</CardDescription>
          </CardHeader>
          <CardContent>
            <ProviderSourcePicker onPick={openAddDialog} onLink={importLink} />
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {/* A refresh that failed leaves the last good listing on screen: the
              providers below are stale, not gone. */}
          {error !== "" ? <MessageAlert title={i18next.t("provider:Failed to get providers")} description={error} /> : null}
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-muted-foreground text-sm">
              {`${total} ${i18next.t("provider:Providers")}`}
            </p>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" onClick={() => loadQuotas(true)} loading={refreshingQuotas}>
                <Wallet />
                {i18next.t("provider:Refresh balances")}
              </Button>
              <Button variant="outline" size="sm" onClick={() => fetchProviders()} loading={loading}>
                <RefreshCw />
                {i18next.t("general:Refresh")}
              </Button>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {data.map(provider => (
              <ProviderGridCard
                key={providerIdOf(provider)}
                provider={provider}
                agents={agents}
                health={health.find(item => item.provider === providerIdOf(provider))}
                quota={quotas.find(item => item.provider === providerIdOf(provider))}
                busy={binding !== ""}
                onEdit={() => navigate(`/providers/${provider.owner}/${provider.name}`)}
                onDelete={() => deleteProvider(provider)}
                onBind={agent => bindAgent(agent, provider)}
              />
            ))}
          </div>

          {total > pageSize ? (
            <div className="flex justify-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => fetchProviders(page - 1)}
              >
                {i18next.t("general:Previous")}
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={page * pageSize >= total}
                onClick={() => fetchProviders(page + 1)}
              >
                {i18next.t("general:Next")}
              </Button>
            </div>
          ) : null}
        </div>
      )}

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("provider:New Provider")}
        description={source === null ? i18next.t("provider:Source hint") : undefined}
        size={source === null ? "lg" : "default"}
        submitting={adding || test.testing}
        submitText={i18next.t("provider:Add provider")}
        onSubmit={submitProvider}
        // Nothing is filled in yet while the source is still being picked, so
        // there is nothing to submit.
        footer={
          source === null ? (
            <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
              {i18next.t("general:Cancel")}
            </Button>
          ) : undefined
        }
      >
        {source === null ? (
          <ProviderSourcePicker onPick={pickSource} onLink={importLink} />
        ) : (
          <>
            <Field label={i18next.t("provider:Source")}>
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="info">{sourceTitle(source)}</Badge>
                {usesClientAuth(form) ? (
                  <Badge variant="muted">{i18next.t("provider:Caller's own login")}</Badge>
                ) : null}
                <Button type="button" size="xs" variant="ghost" onClick={() => setSource(null)}>
                  {i18next.t("provider:Change source")}
                </Button>
              </div>
            </Field>
            <Field
              label={i18next.t("general:Name")}
              htmlFor="provider-display-name"
              required
              error={nameError}
              hint={i18next.t("provider:Name hint")}
            >
              <Input
                id="provider-display-name"
                value={form.displayName}
                onChange={event => {
                  // One field, because two names to invent is one too many. The
                  // identifier follows what was typed until it is edited by hand.
                  setForm(prev => ({
                    ...prev,
                    displayName: event.target.value,
                    name: providerSlug(event.target.value),
                  }));
                  setNameError("");
                }}
              />
            </Field>
            {usesClientAuth(form) ? null : (
              <Field
                label={
                  <span className="flex flex-wrap items-center gap-2">
                    {i18next.t("provider:API Key")}
                    {/* The form is asking for a key, so it says where to get one. */}
                    {source.website ? (
                      <a
                        href={source.website}
                        target="_blank"
                        rel="noreferrer"
                        className="text-primary inline-flex items-center gap-1 text-xs font-normal hover:underline"
                      >
                        <ExternalLink className="size-3" />
                        {i18next.t("provider:Get a key from {vendor}").replace("{vendor}", source.label)}
                      </a>
                    ) : null}
                  </span>
                }
                htmlFor="provider-api-key"
                hint={i18next.t("provider:API Key ownership hint")}
              >
                <PasswordInput
                  id="provider-api-key"
                  placeholder="sk-..."
                  value={form.apiKey}
                  onChange={event => setFormField("apiKey", event.target.value)}
                />
              </Field>
            )}
            <ProviderModelsField
              provider={form}
              hint={usesClientAuth(form) ? i18next.t("provider:Any model hint") : i18next.t("provider:Models hint")}
              onChange={value => setFormField("models", value)}
            />
            <ProviderTestField test={test} submitText={i18next.t("provider:Add provider")} />
            <Field
              label={i18next.t("provider:Notes")}
              htmlFor="provider-notes"
              hint={i18next.t("provider:Notes hint")}
            >
              <Textarea
                id="provider-notes"
                rows={2}
                value={form.notes}
                onChange={event => setFormField("notes", event.target.value)}
              />
            </Field>
            <Advanced key={source.key} defaultOpen={source.key === customSource}>
              <Field label={i18next.t("provider:Type")}>
                <SimpleSelect
                  value={form.type}
                  onChange={value => setFormField("type", value)}
                  options={[
                    {label: "OpenAI", value: "openai"},
                    {label: "Anthropic", value: "anthropic"},
                    {label: "Custom", value: "custom"},
                  ]}
                />
              </Field>
              <Field label={i18next.t("provider:Base URL")} htmlFor="provider-base-url">
                <SearchSelect
                  allowCustomValue
                  id="provider-base-url"
                  value={form.baseUrl}
                  placeholder={baseUrlPlaceholder(form.type)}
                  options={baseUrlPresets(form.type)}
                  onChange={value => setFormField("baseUrl", value)}
                />
              </Field>
              <Field
                label={i18next.t("provider:Authentication")}
                hint={usesClientAuth(form) ? i18next.t("provider:Client auth hint") : undefined}
              >
                <SimpleSelect
                  value={form.authMode}
                  // The stored key is meaningless once the caller's own is forwarded.
                  onChange={value => setForm(prev => ({...prev, authMode: value, apiKey: ""}))}
                  options={[
                    {label: i18next.t("provider:Stored API key"), value: authProvider},
                    {label: i18next.t("provider:Caller's own login"), value: authClient},
                  ]}
                />
              </Field>
              <ProviderIconField provider={form} onChange={value => setFormField("icon", value)} />
            </Advanced>
          </>
        )}
      </FormDialog>
    </PageContainer>
  );
}
