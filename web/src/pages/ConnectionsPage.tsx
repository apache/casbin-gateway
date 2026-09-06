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
import {Check, ExternalLink, Plug, RefreshCw, Search, TriangleAlert} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import * as ConnectorBackend from "@/backend/ConnectorBackend";
import {AgentIcon} from "@/components/AgentIcon";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {CopyButton, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {connectorText as text} from "@/lib/connectors";
import {cn} from "@/lib/utils";
import type {
  Account,
  AgentConfigPlanItem,
  ConnectorCatalog,
  ConnectorEntry,
  ConnectorTarget,
  ConnectorTool,
  OrphanConnection,
} from "@/types";

const CREDENTIAL_MASK = "***";

/** The catalog sections, keyed by the id the backend lists them under. */
const categoryLabels: Record<string, string> = {
  office: "connector:Office",
  docs: "connector:Documents",
  dev: "connector:Development",
  pro: "connector:Professional",
  mail: "connector:Mail",
  storage: "connector:Storage",
  life: "connector:Life",
};

function iconUrl(icon: string) {
  return icon === "" ? "" : `https://www.google.com/s2/favicons?domain=${icon}&sz=64`;
}

/** What one planned write ended up doing, as the result list reads it out. */
function planLabel(item: AgentConfigPlanItem) {
  switch (item.action) {
  case "create":
  case "overwrite":
    return "connector:Written";
  case "remove":
    return "connector:Taken out";
  default:
    return "connector:Failed";
  }
}

/**
 * The Connections page: every application an agent on this machine can be
 * connected to. A connection is stored here once, credentials included, and
 * written into whichever agents are ticked, which is what makes connecting an
 * application one action rather than one per agent.
 */
export default function ConnectionsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [catalog, setCatalog] = React.useState<ConnectorCatalog | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [category, setCategory] = React.useState("");
  const [query, setQuery] = React.useState("");
  const [editing, setEditing] = React.useState<ConnectorEntry | null>(null);
  // An "add this to Gateway" link lands here naming one application, and what
  // it asks for is this dialog rather than anything written on its behalf.
  const [params, setParams] = useSearchParams();
  const asked = params.get("connect") ?? "";
  const [retesting, setRetesting] = React.useState(false);

  const reload = React.useCallback(() => {
    if (!isAdmin) {
      setLoading(false);
      return;
    }
    setLoading(true);
    ConnectorBackend.getConnectors(account.name)
      .then(response => {
        if (response.status === "ok") {
          setCatalog(response.data);
        } else {
          Setting.showMessage("error", response.msg ?? "");
        }
      })
      .catch(error => Setting.showMessage("error", error.message))
      .finally(() => setLoading(false));
  }, [account.name, isAdmin]);

  React.useEffect(() => reload(), [reload]);

  React.useEffect(() => {
    if (asked === "" || catalog === null) {
      return;
    }
    const found = catalog.connectors.find(entry => entry.id === asked);
    if (found) {
      setEditing(found);
    } else {
      Setting.showMessage("error", i18next.t("connector:No such connector").replace("{name}", asked));
    }
    // Taken out of the address once it has been acted on, so a refresh does not
    // reopen a dialog the operator already closed.
    params.delete("connect");
    setParams(params, {replace: true});
  }, [asked, catalog, params, setParams]);

  // A server that has been updated since it was connected may offer different
  // tools, and the per-tool switches are built from the list the last test
  // found. This is how that list is brought up to date.
  const retestAll = () => {
    setRetesting(true);
    ConnectorBackend.retestConnections(account.name)
      .then(response => {
        if (response.status !== "ok") {
          Setting.showMessage("error", response.msg ?? "");
          return;
        }
        Setting.showMessage("success", i18next.t("connector:Retesting").replace("{count}", `${response.data ?? 0}`));
        // Each test starts a server and waits for it, so the answers arrive
        // over the next while rather than at once.
        window.setTimeout(reload, 20000);
        window.setTimeout(reload, 60000);
      })
      .catch(error => Setting.showMessage("error", error.message))
      .finally(() => setRetesting(false));
  };

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const connectors = catalog?.connectors ?? [];
  const categories = (catalog?.categories ?? []).filter(name => connectors.some(entry => entry.category === name));
  // The search reads the wording actually on the card, so what somebody types
  // after seeing it finds it again whichever language the page is in.
  const needle = query.trim().toLowerCase();
  const shown = connectors.filter(entry => {
    if (category !== "" && entry.category !== category) {
      return false;
    }
    if (needle === "") {
      return true;
    }
    return [entry.id, text(entry.displayName), text(entry.description)]
      .join(" ")
      .toLowerCase()
      .includes(needle);
  });

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("connector:Connections")}
        description={i18next.t("connector:Connections detail")}
        actions={
          connectors.some(entry => entry.connected) ? (
            <Button variant="outline" onClick={retestAll} disabled={retesting}>
              <RefreshCw className={cn("size-4", retesting && "animate-spin")} />
              {i18next.t("connector:Test them all")}
            </Button>
          ) : null
        }
      />

      <div className="flex flex-wrap items-center gap-2">
        <FilterChip active={category === ""} onClick={() => setCategory("")}>
          {i18next.t("connector:All")}
          <span className="text-muted-foreground ml-1.5 text-xs">{connectors.length}</span>
        </FilterChip>
        {categories.map(name => (
          <FilterChip key={name} active={category === name} onClick={() => setCategory(name)}>
            {i18next.t(categoryLabels[name] ?? name)}
            <span className="text-muted-foreground ml-1.5 text-xs">
              {connectors.filter(entry => entry.category === name).length}
            </span>
          </FilterChip>
        ))}

        <div className="relative ml-auto w-full sm:w-56">
          <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
          <Input
            value={query}
            onChange={event => setQuery(event.target.value)}
            placeholder={i18next.t("connector:Search applications")}
            className="h-9 pl-8"
          />
        </div>
      </div>

      {(catalog?.orphans ?? []).map(orphan => (
        <OrphanRow key={orphan.name} account={account} orphan={orphan} onDone={reload} />
      ))}

      {loading && connectors.length === 0 ? (
        <p className="text-muted-foreground text-sm">{i18next.t("general:Loading")}</p>
      ) : shown.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          {i18next.t("connector:Nothing matches").replace("{query}", query.trim())}
        </p>
      ) : (
        <div className="grid gap-4 lg:grid-cols-2">
          {shown.map(entry => (
            <ConnectorCard
              key={entry.id}
              entry={entry}
              targets={catalog?.targets ?? []}
              onConnect={() => setEditing(entry)}
            />
          ))}
        </div>
      )}

      {editing ? (
        <ConnectDialog
          account={account}
          entry={editing}
          targets={catalog?.targets ?? []}
          onClose={() => setEditing(null)}
          onRefresh={reload}
          onDone={() => {
            setEditing(null);
            reload();
          }}
        />
      ) : null}
    </PageContainer>
  );
}

/**
 * A connection whose connector went away with a release. It is not a card: there
 * is nothing to fill in and nothing to test, only an entry still sitting in some
 * agents and a credential still stored here, both of which disconnecting undoes.
 */
function OrphanRow({
  account,
  orphan,
  onDone,
}: {
  account: Account;
  orphan: OrphanConnection;
  onDone: () => void;
}) {
  const [busy, setBusy] = React.useState(false);

  const remove = () => {
    setBusy(true);
    ConnectorBackend.disconnect(account.name, orphan.name)
      .then(response => {
        if (response.status === "ok") {
          Setting.showMessage("success", i18next.t("connector:Disconnected"));
          onDone();
        } else {
          Setting.showMessage("error", response.msg ?? "");
        }
      })
      .catch(error => Setting.showMessage("error", error.message))
      .finally(() => setBusy(false));
  };

  return (
    <div className="border-destructive/40 bg-destructive/5 flex items-center gap-3 rounded-xl border p-4">
      <TriangleAlert className="text-destructive size-5 shrink-0" />
      <div className="min-w-0 flex-1 text-sm">
        <div className="font-medium">
          {i18next.t("connector:Orphan connection").replace("{name}", orphan.name)}
        </div>
        <p className="text-muted-foreground">
          {i18next.t("connector:Orphan hint").replace("{count}", `${orphan.agents.length}`)}
        </p>
      </div>
      <Button variant="outline" onClick={remove} disabled={busy}>
        {i18next.t("connector:Disconnect")}
      </Button>
    </div>
  );
}

function FilterChip({active, onClick, children}: {active: boolean; onClick: () => void; children: React.ReactNode}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-full px-3 py-1.5 text-sm transition-colors",
        active ? "bg-foreground text-background font-medium" : "text-muted-foreground hover:bg-muted",
      )}
    >
      {children}
    </button>
  );
}

function ConnectorCard({
  entry,
  targets,
  onConnect,
}: {
  entry: ConnectorEntry;
  targets: ConnectorTarget[];
  onConnect: () => void;
}) {
  const image = iconUrl(entry.icon);
  const nameOf = (agentId: string) => targets.find(target => target.agentId === agentId)?.name ?? agentId;
  return (
    <Card className="bg-muted/40 border-none shadow-none">
      <CardContent className="flex items-start gap-4 p-5">
        <div className="bg-background flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-xl">
          {image === "" ? <Plug className="text-muted-foreground size-5" /> : <img src={image} alt="" className="size-6" />}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-semibold">{text(entry.displayName)}</span>
            {entry.connected && entry.agents.length === 0 ? (
              <Badge variant="destructive">{i18next.t("connector:In no agent")}</Badge>
            ) : null}
            {entry.tools && entry.tools.length > 0 ? (
              <Badge variant="outline">
                {i18next.t("connector:Tool count", {count: entry.tools.length})}
              </Badge>
            ) : null}
            {entry.probeError ? <Badge variant="destructive">{i18next.t("connector:Test failed")}</Badge> : null}
            {entry.paid ? <Badge variant="outline">{i18next.t("connector:Billed by vendor")}</Badge> : null}
            {entry.unverified ? <Badge variant="outline">{i18next.t("connector:Unverified")}</Badge> : null}
          </div>
          <p className="text-muted-foreground mt-1 line-clamp-2 text-sm">{text(entry.description)}</p>

          {entry.agents.length > 0 ? (
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              <span className="text-muted-foreground text-xs">{i18next.t("connector:In agents")}</span>
              {entry.agents.map(agentId => (
                <span
                  key={agentId}
                  className="bg-background flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium"
                >
                  <AgentIcon agent={agentId} size={14} />
                  {nameOf(agentId)}
                </span>
              ))}
            </div>
          ) : null}
        </div>

        <Button
          variant={entry.connected ? "outline" : "default"}
          className="shrink-0 rounded-full"
          onClick={onConnect}
        >
          {entry.connected ? i18next.t("connector:Manage") : i18next.t("connector:Connect")}
        </Button>
      </CardContent>
    </Card>
  );
}

/**
 * The connect dialog: the credentials this connector asks for, and the agents
 * to write it into. A stored secret comes back masked, and leaving the mask in
 * place keeps whatever is already there.
 */
function ConnectDialog({
  account,
  entry,
  targets,
  onClose,
  onRefresh,
  onDone,
}: {
  account: Account;
  entry: ConnectorEntry;
  targets: ConnectorTarget[];
  onClose: () => void;
  onRefresh: () => void;
  onDone: () => void;
}) {
  const fields = entry.auth.fields ?? [];
  const isOauth = entry.auth.kind === "oauth2";
  const [credentials, setCredentials] = React.useState<Record<string, string>>({});
  const [agents, setAgents] = React.useState<string[]>(entry.agents);
  // installed is the agents the server says this connection actually reached. A
  // tick is what the operator is asking for; these are what is already there.
  const [installed, setInstalled] = React.useState<string[]>(entry.agents);
  const [plan, setPlan] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [submitting, setSubmitting] = React.useState(false);
  const [authorized, setAuthorized] = React.useState(entry.authorized);
  const [awaiting, setAwaiting] = React.useState(false);
  const [redirectUri, setRedirectUri] = React.useState("");

  // syncAgents is for the first read only: this also runs while the dialog is
  // open, and resetting the ticks there would undo what is being edited.
  const load = React.useCallback(
    (syncAgents = false) =>
      ConnectorBackend.getConnection(account.name, entry.id).then(response => {
        if (response.status === "ok" && response.data) {
          setCredentials(response.data.credentials ?? {});
          setInstalled(response.data.agents ?? []);
          if (syncAgents) {
            setAgents(response.data.agents ?? []);
          }
          return (response.data.credentials ?? {})["accessToken"] !== undefined;
        }
        return false;
      }),
    [account.name, entry.id],
  );

  React.useEffect(() => {
    if (entry.connected) {
      load(true);
    }
  }, [entry.connected, load]);

  React.useEffect(() => {
    if (!isOauth) {
      return;
    }
    ConnectorBackend.getConnectorRedirectUri().then(response => {
      if (response.status === "ok") {
        setRedirectUri(response.data ?? "");
      }
    });
  }, [isOauth]);

  // Approving happens in another tab, and the vendor sends the operator back to
  // Gateway rather than to this dialog, so the only way this form learns it
  // worked is to keep asking until the grant is there.
  React.useEffect(() => {
    if (!awaiting) {
      return;
    }
    const timer = window.setInterval(() => {
      load().then(granted => {
        if (granted) {
          setAuthorized(true);
          setAwaiting(false);
          Setting.showMessage("success", i18next.t("connector:Authorized"));
        }
      });
    }, 2000);
    const giveUp = window.setTimeout(() => setAwaiting(false), 3 * 60 * 1000);
    return () => {
      window.clearInterval(timer);
      window.clearTimeout(giveUp);
    };
  }, [awaiting, load]);

  const authorize = () => {
    setSubmitting(true);
    ConnectorBackend.startConnectorAuth(account.name, entry.id, credentials)
      .then(response => {
        if (response.status === "ok" && response.data) {
          window.open(response.data, "_blank", "noopener");
          setAwaiting(true);
        } else {
          Setting.showMessage("error", response.msg ?? "");
        }
      })
      .catch(error => Setting.showMessage("error", error.message))
      .finally(() => setSubmitting(false));
  };

  const missing =
    fields.some(field => field.required && (credentials[field.key] ?? "").trim() === "") || (isOauth && !authorized);

  const [testing, setTesting] = React.useState(false);
  const [probe, setProbe] = React.useState<{tools?: ConnectorTool[]; server?: string; error?: string}>({
    tools: entry.tools,
    server: entry.serverName,
    error: entry.probeError,
  });

  const test = () => {
    setTesting(true);
    setProbe({});
    ConnectorBackend.testConnection(account.name, entry.id)
      .then(response => {
        if (response.status === "ok" && response.data) {
          setProbe({tools: response.data.tools, server: response.data.serverName});
        } else {
          setProbe({error: response.msg ?? ""});
        }
      })
      .catch(error => setProbe({error: error.message}))
      .finally(() => setTesting(false));
  };

  // wrote is what happened to an agent the operator asked for. An item about an
  // agent being let go is not one of these: its entry is meant to go.
  const wrote = (item: AgentConfigPlanItem) => item.action === "create" || item.action === "overwrite";
  const missed = (item: AgentConfigPlanItem) => agents.includes(item.agentId) && !wrote(item);

  const submit = () => {
    setSubmitting(true);
    setPlan(null);
    ConnectorBackend.connect({owner: account.name, name: entry.id, credentials: credentials, agents: agents})
      .then(response => {
        if (response.status !== "ok") {
          Setting.showMessage("error", response.msg ?? "");
          return;
        }
        // A connection that reached none of the agents used to close the dialog
        // and look exactly like one that worked, so a failure keeps it open with
        // the file each agent was written to.
        const planned = response.data ?? [];
        const failures = planned.filter(missed);
        if (failures.length > 0) {
          setPlan(planned);
          load();
          onRefresh();
          Setting.showMessage("error", i18next.t("connector:Not written", {count: failures.length}));
          return;
        }
        Setting.showMessage("success", i18next.t("connector:Testing in background"));
        onDone();
        window.setTimeout(onDone, 12000);
      })
      .catch(error => Setting.showMessage("error", error.message))
      .finally(() => setSubmitting(false));
  };

  const remove = () => {
    setSubmitting(true);
    ConnectorBackend.disconnect(account.name, entry.id)
      .then(response => {
        if (response.status === "ok") {
          Setting.showMessage("success", i18next.t("connector:Disconnected"));
          onDone();
        } else {
          Setting.showMessage("error", response.msg ?? "");
        }
      })
      .catch(error => Setting.showMessage("error", error.message))
      .finally(() => setSubmitting(false));
  };

  const nameOf = (agentId: string) => targets.find(target => target.agentId === agentId)?.name ?? agentId;
  const adding = agents.filter(agentId => !installed.includes(agentId));
  const removing = installed.filter(agentId => !agents.includes(agentId));
  // What pressing Connect would change, so the button is not the only way to
  // find out. Empty leaves the field's own hint in place.
  const pending = [
    adding.length > 0 ? i18next.t("connector:Will write").replace("{names}", adding.map(nameOf).join(", ")) : "",
    removing.length > 0 ? i18next.t("connector:Will remove").replace("{names}", removing.map(nameOf).join(", ")) : "",
  ]
    .filter(part => part !== "")
    .join(" ");

  return (
    <FormDialog
      open={true}
      onOpenChange={open => !open && onClose()}
      title={text(entry.displayName)}
      description={text(entry.description)}
      onSubmit={submit}
      submitting={submitting}
      submitDisabled={missing || agents.length === 0}
      submitText={i18next.t("connector:Connect")}
      note={
        entry.connected ? (
          <Button variant="ghost" className="text-destructive" onClick={remove} disabled={submitting}>
            {i18next.t("connector:Disconnect")}
          </Button>
        ) : null
      }
    >
      {entry.auth.registerUrl ? (
        <a
          href={entry.auth.registerUrl}
          target="_blank"
          rel="noreferrer"
          className="text-primary inline-flex items-center gap-1 text-sm hover:underline"
        >
          {i18next.t("connector:Where these come from")}
          <ExternalLink className="size-3.5" />
        </a>
      ) : null}

      {isOauth && redirectUri !== "" ? (
        <Field label={i18next.t("connector:Callback URL")} hint={i18next.t("connector:Callback URL hint")}>
          <div className="flex items-center gap-2">
            <code className="bg-muted min-w-0 flex-1 truncate rounded px-2 py-1.5 text-xs">{redirectUri}</code>
            <CopyButton value={redirectUri} />
          </div>
        </Field>
      ) : null}

      {fields.map(field => (
        <Field key={field.key} label={text(field.label)} required={field.required} hint={text(field.help)}>
          <Input
            type={field.secret ? "password" : "text"}
            value={credentials[field.key] ?? ""}
            placeholder={field.placeholder}
            onFocus={event => {
              // A masked secret stands for one already stored, not a value to
              // edit, so touching the box clears it rather than letting the
              // mask be sent back as the new credential.
              if (event.target.value === CREDENTIAL_MASK) {
                setCredentials({...credentials, [field.key]: ""});
              }
            }}
            onChange={event => setCredentials({...credentials, [field.key]: event.target.value})}
          />
        </Field>
      ))}

      {isOauth ? (
        <Field
          label={i18next.t("connector:Authorization")}
          hint={authorized ? undefined : i18next.t("connector:Authorization hint")}
        >
          <div className="flex items-center gap-3">
            <Button
              type="button"
              variant={authorized ? "outline" : "default"}
              onClick={authorize}
              disabled={submitting || (credentials["clientId"] ?? "").trim() === ""}
            >
              {authorized ? i18next.t("connector:Authorize again") : i18next.t("connector:Authorize")}
            </Button>
            <span className="text-muted-foreground text-sm">
              {awaiting
                ? i18next.t("connector:Waiting for approval")
                : authorized
                  ? i18next.t("connector:Authorized")
                  : i18next.t("connector:Not authorized")}
            </span>
          </div>
        </Field>
      ) : null}

      {entry.connected ? (
        <Field label={i18next.t("connector:Test")} hint={i18next.t("connector:Test hint")}>
          <div className="flex items-start gap-3">
            <Button type="button" variant="outline" onClick={test} disabled={testing}>
              {testing ? i18next.t("connector:Testing") : i18next.t("connector:Run test")}
            </Button>
            <div className="min-w-0 flex-1 pt-1.5 text-sm">
              {!testing && !probe.error && !probe.tools ? (
                <span className="text-muted-foreground">{i18next.t("connector:Never tested")}</span>
              ) : null}
              {probe.error ? (
                <span className="text-destructive break-words">{probe.error}</span>
              ) : probe.tools ? (
                <span className="text-muted-foreground">
                  {i18next
                    .t("connector:Test result")
                    .replace("{server}", probe.server || entry.server.name)
                    .replace("{count}", `${probe.tools.length}`)}
                </span>
              ) : null}
              {!testing && entry.probedTime && !probe.error ? (
                <span className="text-muted-foreground ml-2 text-xs">
                  {i18next.t("connector:Last tested").replace("{time}", new Date(entry.probedTime).toLocaleString())}
                </span>
              ) : null}
            </div>
          </div>
          {probe.tools && probe.tools.length > 0 ? (
            <div className="flex flex-wrap gap-1.5 pt-1">
              {probe.tools.map(tool => (
                <code key={tool.name} className="bg-muted rounded px-1.5 py-0.5 text-xs" title={tool.description}>
                  {tool.name}
                </code>
              ))}
            </div>
          ) : null}
        </Field>
      ) : null}

      <Field
        label={i18next.t("connector:Install into")}
        hint={pending === "" ? i18next.t("connector:Install into hint") : pending}
        error={agents.length === 0 ? i18next.t("connector:Pick at least one agent") : undefined}
      >
        <div className="flex flex-wrap gap-2">
          {targets.map(target => {
            const picked = agents.includes(target.agentId);
            // A tick is what will be there; the check mark is what is there now,
            // which is the difference between asking and having asked.
            const has = installed.includes(target.agentId);
            return (
              <button
                key={target.agentId}
                type="button"
                title={target.mcpFile}
                onClick={() =>
                  setAgents(picked ? agents.filter(id => id !== target.agentId) : [...agents, target.agentId])
                }
                className={cn(
                  "flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-sm transition-colors",
                  picked && has && "border-foreground bg-foreground text-background",
                  picked && !has && "border-foreground border-dashed",
                  !picked && has && "border-destructive/60 text-destructive border-dashed line-through",
                  !picked && !has && "hover:bg-muted",
                )}
              >
                <AgentIcon agent={target.agentId} size={16} />
                {target.name}
                {has ? <Check className="size-3.5" /> : null}
              </button>
            );
          })}
          {targets.length === 0 ? (
            <p className="text-muted-foreground text-sm">{i18next.t("connector:No agent found")}</p>
          ) : null}
        </div>
      </Field>

      {plan && plan.length > 0 ? (
        <Field label={i18next.t("connector:Where it went")}>
          <ul className="space-y-1.5 text-xs">
            {plan.map(item => (
              <li key={`${item.agentId}-${item.action}`} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                <span className="font-medium">{nameOf(item.agentId)}</span>
                <span className={missed(item) ? "text-destructive" : "text-muted-foreground"}>
                  {i18next.t(planLabel(item))}
                </span>
                {item.path ? (
                  <code className="bg-muted min-w-0 rounded px-1.5 py-0.5 break-all">{item.path}</code>
                ) : null}
                {item.reason ? <span className="text-destructive break-words">{item.reason}</span> : null}
              </li>
            ))}
          </ul>
        </Field>
      ) : null}
    </FormDialog>
  );
}
