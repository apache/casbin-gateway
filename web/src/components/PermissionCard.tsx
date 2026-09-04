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
import {
  ChevronDown,
  ChevronRight,
  Copy,
  FileCode2,
  FolderOpen,
  Globe,
  ListChecks,
  Pencil,
  Plug,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import copy from "copy-to-clipboard";
import i18next from "i18next";

import * as PermissionBackend from "@/backend/PermissionBackend";
import * as Setting from "@/Setting";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {Switch} from "@/components/ui/switch";
import {TagsInput} from "@/components/ui/tags-input";
import {Textarea} from "@/components/ui/textarea";
import {
  agentDecidesHere,
  agentRoutedHere,
  agentToolsEnforced,
  directMode,
  gatewayMode,
  routedAgentId,
} from "@/lib/agents";
import {providerIdOf} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {
  Agent,
  AgentPermission,
  AgentPermissionInfo,
  Provider,
  ToolGroup,
  ToolItem,
} from "@/types";

/** The name and icon of each group, by the name the server gives it. */
const groupLabels: {[group: string]: {label: string; icon: React.ElementType}} = {
  shell: {label: "agent:Terminal", icon: Terminal},
  read: {label: "agent:Reading the project", icon: FolderOpen},
  write: {label: "agent:Changing the project", icon: Pencil},
  network: {label: "agent:The internet", icon: Globe},
  agentic: {label: "agent:Planning and delegation", icon: ListChecks},
  mcp: {label: "agent:MCP servers", icon: Plug},
};

/** What each switch is called. An item with no wording here - one of the agent's
 *  own MCP servers - is named by the label the server sent. */
const itemLabels: {[item: string]: string} = {
  "shell/run": "agent:Run a command",
  "shell/output": "agent:Read a running command's output",
  "shell/kill": "agent:Stop a running command",
  "shell/other": "agent:Any other command tool",
  "read/file": "agent:Read a file",
  "read/many": "agent:Read many files at once",
  "read/image": "agent:Look at an image",
  "read/list": "agent:List a directory",
  "read/find": "agent:Find files by name",
  "read/grep": "agent:Search inside files",
  "read/semantic": "agent:Search the codebase by meaning",
  "read/notebook": "agent:Read a notebook",
  "read/other": "agent:Any other tool that reads",
  "write/create": "agent:Create or overwrite a file",
  "write/edit": "agent:Edit a file",
  "write/multi": "agent:Edit several places at once",
  "write/patch": "agent:Apply a patch",
  "write/notebook": "agent:Edit a notebook",
  "write/delete": "agent:Delete a file",
  "write/move": "agent:Move or rename a file",
  "write/mkdir": "agent:Create a directory",
  "write/other": "agent:Any other tool that writes",
  "network/fetch": "agent:Fetch a URL",
  "network/search": "agent:Search the web",
  "network/browser": "agent:Drive a browser",
  "network/other": "agent:Any other tool that goes online",
  "agentic/subagent": "agent:Start a sub-agent",
  "agentic/todo": "agent:Keep a task list",
  "agentic/plan": "agent:Leave plan mode",
  "agentic/ask": "agent:Ask the user a question",
  "agentic/command": "agent:Run a slash command",
  "agentic/skill": "agent:Run a skill",
  "agentic/memory": "agent:Remember something",
  "mcp/other": "agent:Any other MCP server",
};

function itemLabel(item: ToolItem) {
  const known = itemLabels[item.name];
  return known ? i18next.t(known) : item.label || item.name.split("/")[1];
}

/** The three ways a list of models or providers is read. */
const listModes = ["all", "allow", "deny"];

function modeOptions(anyLabel: string) {
  return [
    {label: i18next.t(anyLabel), value: "all"},
    {label: i18next.t("agent:Only the ones I pick"), value: "allow"},
    {label: i18next.t("agent:All but the ones I pick"), value: "deny"},
  ];
}

/** An item is allowed until somebody turns it off, so a switch this agent was
 *  configured before it existed never takes anything away. */
function isAllowed(tools: AgentPermission["tools"], item: string) {
  return (tools ?? {})[item] !== false;
}

/** One group: its own switch sets every item in it at once, and the items
 *  underneath are what a finer answer is given with. */
function GroupBlock({
  group,
  tools,
  busy,
  onItems,
}: {
  group: ToolGroup;
  tools: AgentPermission["tools"];
  busy: boolean;
  onItems: (changed: {[item: string]: boolean}) => void;
}) {
  const [open, setOpen] = React.useState(false);
  const known = groupLabels[group.name];
  const Icon = known ? known.icon : ShieldCheck;
  const items = group.items ?? [];
  const blocked = items.filter(item => !isAllowed(tools, item.name));
  const Chevron = open ? ChevronDown : ChevronRight;

  const setAll = (allowed: boolean) => {
    const changed: {[item: string]: boolean} = {};
    items.forEach(item => {
      changed[item.name] = allowed;
    });
    onItems(changed);
  };

  return (
    <div className="rounded-md border">
      <div className="flex items-center gap-2 p-2.5">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className="flex min-w-0 flex-1 items-center gap-2 text-left text-sm"
        >
          <Chevron className="h-4 w-4 shrink-0 text-muted-foreground" />
          <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
          <span className="truncate font-medium">
            {known ? i18next.t(known.label) : group.name}
          </span>
          {blocked.length === 0 ? (
            <Badge variant="muted" className="shrink-0 font-normal">
              {`${items.length} ${i18next.t("agent:allowed")}`}
            </Badge>
          ) : (
            <Badge variant="warning" className="shrink-0 font-normal">
              {`${blocked.length} ${i18next.t("agent:blocked")}`}
            </Badge>
          )}
        </button>
        <Switch
          checked={blocked.length === 0}
          disabled={busy}
          aria-label={known ? i18next.t(known.label) : group.name}
          onCheckedChange={setAll}
        />
      </div>

      {open ? (
        <div className="grid grid-cols-1 gap-x-4 border-t p-2.5 sm:grid-cols-2">
          {items.map(item => (
            <label
              key={item.name}
              className="flex items-start justify-between gap-3 border-b py-2 text-sm last:border-b-0 sm:[&:nth-last-child(2)]:border-b-0"
            >
              <span className="min-w-0">
                {itemLabel(item)}
                {(item.tools ?? []).length > 0 ? (
                  <code className="block truncate text-xs text-muted-foreground">
                    {(item.tools ?? []).join(", ")}
                  </code>
                ) : null}
              </span>
              <Switch
                className="mt-0.5 shrink-0"
                checked={isAllowed(tools, item.name)}
                disabled={busy}
                onCheckedChange={allowed => onItems({[item.name]: allowed})}
              />
            </label>
          ))}
        </div>
      ) : null}
    </div>
  );
}

/** The casbin model and policy the switches above compile to, which is what is
 *  actually enforced, plus the rules that are only written here. */
function AdvancedView({
  info,
  busy,
  onRules,
}: {
  info: AgentPermissionInfo;
  busy: boolean;
  onRules: (rules: string[]) => void;
}) {
  const policy = info.policy ?? [];
  const stored = (info.permission.rules ?? []).join("\n");
  const [draft, setDraft] = React.useState(stored);

  // The textarea is only a draft until it loses focus, so a half-typed rule is
  // never saved; a rule saved elsewhere replaces it.
  React.useEffect(() => setDraft(stored), [stored]);

  return (
    <div className="space-y-3 rounded-md border p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-sm font-medium">
          <FileCode2 className="h-4 w-4 text-muted-foreground" />
          {i18next.t("agent:Casbin configuration")}
        </span>
        <Button
          variant="outline"
          size="xs"
          onClick={() => {
            copy(`${info.model}\n${policy.join("\n")}\n`);
            Setting.showMessage("success", i18next.t("agent:Casbin configuration copied"));
          }}
        >
          <Copy />
          {i18next.t("general:Copy")}
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">{i18next.t("agent:Casbin hint")}</p>

      <div className="space-y-1">
        <div className="text-xs text-muted-foreground">model.conf</div>
        <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">{info.model}</pre>
      </div>

      <div className="space-y-1">
        <div className="text-xs text-muted-foreground">policy.csv</div>
        <pre className="overflow-x-auto rounded-md bg-muted p-2 text-xs">
          {policy.length === 0 ? "# " + i18next.t("agent:No rule yet") : policy.join("\n")}
        </pre>
      </div>

      <div className="space-y-1">
        <div className="text-xs text-muted-foreground">{i18next.t("agent:Extra rules")}</div>
        <Textarea
          className="font-mono text-xs"
          rows={3}
          spellCheck={false}
          value={draft}
          disabled={busy}
          placeholder="claude-code, tool:mcp/*, use, deny"
          onChange={event => setDraft(event.target.value)}
          onBlur={() => {
            const rules = draft.split("\n").map(line => line.trim()).filter(line => line !== "");
            if (rules.join("\n") !== stored) {
              onRules(rules);
            }
          }}
        />
        <p className="text-xs text-muted-foreground">{i18next.t("agent:Extra rules hint")}</p>
      </div>
    </div>
  );
}

/** Where the rules below are actually applied. An agent that relays through
 *  Gateway is held in the request; one that does not is held by the hook
 *  Gateway installed in it, which only decides tool calls; an agent with
 *  neither is held nowhere, and says so. */
function RoutingWarning({agent}: {agent: Agent}) {
  if (agentRoutedHere(agent)) {
    return null;
  }

  if (agentDecidesHere(agent)) {
    return (
      <p className="text-sm text-muted-foreground">{i18next.t("agent:Hook enforced hint")}</p>
    );
  }

  const routed = routedAgentId(agent);
  return (
    <p className="text-sm text-warning">
      {routed !== "" ? (
        <>
          {i18next.t("agent:Shared config permission hint")} <code>{routed}</code>
        </>
      ) : agent.providerConfig?.supported === false ? (
        i18next.t("agent:Unknown routing permission hint")
      ) : (agent.mode || gatewayMode) === directMode ? (
        i18next.t("agent:Direct mode permission hint")
      ) : (
        i18next.t("agent:Unrouted permission hint")
      )}
    </p>
  );
}

/**
 * What one agent is allowed to ask Gateway for: the tools it may be offered, the
 * models it may name and the providers its requests may reach. The switches are
 * what is set; casbin is what decides, and the advanced view shows the policy
 * they compile to.
 */
export function PermissionCard({
  agent,
  providers,
  className,
  onSaved,
}: {
  agent: Agent;
  providers: Provider[];
  className?: string;
  /** Told after a save, for a page that lists what every agent is held to. */
  onSaved?: () => void;
}) {
  const [info, setInfo] = React.useState<AgentPermissionInfo | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [advanced, setAdvanced] = React.useState(false);

  const agentId = agent.agentId;
  const owner = agent.owner;

  React.useEffect(() => {
    if (agentId === "") {
      return;
    }
    setInfo(null);
    PermissionBackend.getAgentPermission(agentId, owner)
      .then(res => setInfo(res.status === "ok" ? (res.data ?? null) : null))
      .catch(() => setInfo(null));
  }, [agentId, owner]);

  if (info === null) {
    return null;
  }

  const permission = info.permission;
  const groups = info.groups ?? [];
  const items = groups.flatMap(group => group.items ?? []);
  const blocked = items.filter(item => !isAllowed(permission.tools, item.name));

  const save = (changed: Partial<AgentPermission>) => {
    const next = {...permission, ...changed};
    setBusy(true);
    PermissionBackend.updateAgentPermission(agentId, next, owner)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setInfo(res.data);
          onSaved?.();
          Setting.showMessage("success", i18next.t("agent:Permissions saved"));
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agent:Failed to save the permissions"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusy(false));
  };

  // Every switch the page drew is sent back, so an item nobody touched is
  // stored as allowed rather than as unknown: that is what lets a group be
  // closed with one rule instead of one per item.
  const saveItems = (changed: {[item: string]: boolean}) => {
    const tools: {[item: string]: boolean} = {};
    items.forEach(item => {
      tools[item.name] = isAllowed(permission.tools, item.name);
    });
    save({tools: {...tools, ...changed}});
  };

  // The models of every provider, so the field offers what this machine can
  // actually reach rather than an empty box.
  const modelSuggestions = Array.from(
    new Set(providers.flatMap(provider => provider.models ?? [])),
  ).sort();

  const listMode = (mode: string) => (listModes.includes(mode) ? mode : "all");

  return (
    <Card className={className}>
      <CardHeader className="p-4 pb-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-base">
            {i18next.t("agent:Permissions")}
            {!permission.enabled ? (
              <Badge variant="muted">{i18next.t("agent:Unrestricted")}</Badge>
            ) : !agentToolsEnforced(agent) ? (
              <Badge variant="warning">{i18next.t("agent:Not enforced")}</Badge>
            ) : blocked.length === 0 ? (
              <Badge variant="muted">{i18next.t("agent:Enforced")}</Badge>
            ) : (
              <Badge variant="success">
                {`${blocked.length} ${i18next.t("agent:blocked")}`}
              </Badge>
            )}
          </CardTitle>
          <Button variant="ghost" size="xs" onClick={() => setAdvanced(!advanced)}>
            {i18next.t("general:Advanced")}
          </Button>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 p-4 pt-0">
        <label className="flex items-start gap-2 text-sm">
          <Switch
            className="mt-0.5"
            checked={permission.enabled}
            disabled={busy}
            onCheckedChange={checked => save({enabled: checked})}
          />
          <span>
            {i18next.t("agent:Enforce permissions")}
            {/* The claim that every relayed request is held to the rules is only
                made where it is true; where it is not, the warning below says
                why instead. */}
            {!permission.enabled || agentRoutedHere(agent) ? (
              <span className="block text-muted-foreground">
                {i18next.t(permission.enabled ? "agent:Enforced hint" : "agent:Unrestricted hint")}
              </span>
            ) : null}
          </span>
        </label>

        {permission.enabled ? <RoutingWarning agent={agent} /> : null}

        <div className={cn("space-y-3", !permission.enabled && "pointer-events-none opacity-50")}>
          <div className="space-y-2">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="text-sm font-medium">{i18next.t("agent:What it may do")}</div>
              <div className="text-xs text-muted-foreground">
                {`${items.length} ${i18next.t("agent:switches")}`}
              </div>
            </div>
            <div className="space-y-2">
              {groups.map(group => (
                <GroupBlock
                  key={group.name}
                  group={group}
                  tools={permission.tools}
                  busy={busy || !permission.enabled}
                  onItems={saveItems}
                />
              ))}
            </div>
            <p className="text-xs text-muted-foreground">{i18next.t("agent:Tool hint")}</p>
          </div>

          <div className="space-y-2">
            <div className="text-sm font-medium">{i18next.t("provider:Models")}</div>
            <SimpleSelect
              value={listMode(permission.modelMode)}
              options={modeOptions("agent:Any model")}
              disabled={busy || !permission.enabled}
              onChange={value => save({modelMode: value})}
            />
            {listMode(permission.modelMode) === "all" ? null : (
              <>
                <TagsInput
                  value={permission.models}
                  suggestions={modelSuggestions}
                  disabled={busy || !permission.enabled}
                  placeholder={i18next.t("agent:Model placeholder")}
                  onChange={models => save({models: models})}
                />
                <p className="text-xs text-muted-foreground">{i18next.t("agent:Model list hint")}</p>
              </>
            )}
          </div>

          <div className="space-y-2">
            <div className="text-sm font-medium">{i18next.t("provider:Providers")}</div>
            <SimpleSelect
              value={listMode(permission.providerMode)}
              options={modeOptions("agent:Any provider")}
              disabled={busy || !permission.enabled}
              onChange={value => save({providerMode: value})}
            />
            {listMode(permission.providerMode) === "all" ? null : providers.length === 0 ? (
              <p className="text-sm text-muted-foreground">{i18next.t("agent:No provider yet")}</p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {providers.map(provider => {
                  const id = providerIdOf(provider);
                  const picked = (permission.providers ?? []).includes(id);
                  return (
                    <button
                      key={id}
                      type="button"
                      disabled={busy || !permission.enabled}
                      onClick={() =>
                        save({
                          providers: picked
                            ? (permission.providers ?? []).filter(item => item !== id)
                            : [...(permission.providers ?? []), id],
                        })
                      }
                      className={cn(
                        "rounded-md border px-3 py-1.5 text-sm transition-colors",
                        picked ? "border-primary bg-primary/10" : "hover:bg-accent",
                        (busy || !permission.enabled) && "cursor-not-allowed opacity-50",
                      )}
                    >
                      {provider.displayName || provider.name}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        {advanced ? (
          <AdvancedView info={info} busy={busy} onRules={rules => save({rules: rules})} />
        ) : null}
      </CardContent>
    </Card>
  );
}
