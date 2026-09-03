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
import {Copy, FileCode2, FolderOpen, Globe, Pencil, Plug, ShieldCheck, Terminal} from "lucide-react";
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
import {directMode} from "@/lib/agents";
import {providerIdOf} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {Agent, AgentPermission, AgentPermissionInfo, Provider, ToolGroup} from "@/types";

/** The label and the icon of each tool group, by the name the server gives it.
 *  A group added to a later server than this page is drawn by its own name. */
const groupLabels: {[group: string]: {label: string; icon: React.ElementType}} = {
  shell: {label: "agent:Run commands", icon: Terminal},
  fileRead: {label: "agent:Read files", icon: FolderOpen},
  fileWrite: {label: "agent:Change files", icon: Pencil},
  network: {label: "agent:Reach the internet", icon: Globe},
  mcp: {label: "agent:Use MCP servers", icon: Plug},
};

/** The three ways a list of models or providers is read. */
const listModes = ["all", "allow", "deny"];

function modeOptions(anyLabel: string) {
  return [
    {label: i18next.t(anyLabel), value: "all"},
    {label: i18next.t("agent:Only the ones I pick"), value: "allow"},
    {label: i18next.t("agent:All but the ones I pick"), value: "deny"},
  ];
}

/** One tool group, as the switch that turns it off. */
function ToolSwitch({
  group,
  allowed,
  busy,
  onChange,
}: {
  group: ToolGroup;
  allowed: boolean;
  busy: boolean;
  onChange: (allowed: boolean) => void;
}) {
  const known = groupLabels[group.name];
  const Icon = known ? known.icon : ShieldCheck;

  return (
    <label className="flex items-start gap-2.5 rounded-md border p-2.5 text-sm">
      <Switch className="mt-0.5" checked={allowed} disabled={busy} onCheckedChange={onChange} />
      <span className="min-w-0">
        <span className="flex items-center gap-1.5">
          <Icon className="h-3.5 w-3.5 text-muted-foreground" />
          {known ? i18next.t(known.label) : group.name}
        </span>
        <code className="block truncate text-xs text-muted-foreground">
          {(group.examples ?? []).join(", ")}
        </code>
      </span>
    </label>
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
          placeholder="claude-code, tool:mcp, use, deny"
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

/**
 * What one agent is allowed to ask Gateway for: the tools it may be offered,
 * the models it may name and the providers its requests may reach. The switches
 * are what is set; casbin is what decides, and the advanced view shows the
 * policy they compile to.
 */
export function PermissionCard({agent, providers}: {agent: Agent; providers: Provider[]}) {
  const [info, setInfo] = React.useState<AgentPermissionInfo | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [advanced, setAdvanced] = React.useState(false);

  const agentId = agent.agentId;

  React.useEffect(() => {
    if (agentId === "") {
      return;
    }
    PermissionBackend.getAgentPermission(agentId)
      .then(res => setInfo(res.status === "ok" ? (res.data ?? null) : null))
      .catch(() => setInfo(null));
  }, [agentId]);

  if (info === null) {
    return null;
  }

  const permission = info.permission;
  const groups = info.groups ?? [];
  const blocked = groups.filter(group => permission.tools?.[group.name] === false);

  const save = (changed: Partial<AgentPermission>) => {
    const next = {...permission, ...changed};
    setBusy(true);
    PermissionBackend.updateAgentPermission(agentId, next)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setInfo(res.data);
          Setting.showMessage("success", i18next.t("agent:Permissions saved"));
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agent:Failed to save the permissions"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusy(false));
  };

  const setTool = (group: string, allowed: boolean) =>
    save({tools: {...(permission.tools ?? {}), [group]: allowed}});

  // The models of every provider, so the field offers what this machine can
  // actually reach rather than an empty box.
  const modelSuggestions = Array.from(
    new Set(providers.flatMap(provider => provider.models ?? [])),
  ).sort();

  const listMode = (mode: string) => (listModes.includes(mode) ? mode : "all");

  return (
    <Card>
      <CardHeader className="p-4 pb-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-base">
            {i18next.t("agent:Permissions")}
            {permission.enabled ? (
              <Badge variant={blocked.length > 0 ? "success" : "muted"}>
                {i18next.t("agent:Enforced")}
              </Badge>
            ) : (
              <Badge variant="muted">{i18next.t("agent:Unrestricted")}</Badge>
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
            <span className="block text-muted-foreground">
              {i18next.t(permission.enabled ? "agent:Enforced hint" : "agent:Unrestricted hint")}
            </span>
          </span>
        </label>

        {/* An agent that was written the provider's own URL never calls the
            proxy, so there is nothing here to hold it to. */}
        {permission.enabled && agent.mode === directMode ? (
          <p className="text-sm text-warning">{i18next.t("agent:Direct mode permission hint")}</p>
        ) : null}

        <div className={cn("space-y-3", !permission.enabled && "pointer-events-none opacity-50")}>
          <div className="space-y-2">
            <div className="text-sm font-medium">{i18next.t("agent:What it may do")}</div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {groups.map(group => (
                <ToolSwitch
                  key={group.name}
                  group={group}
                  allowed={permission.tools?.[group.name] !== false}
                  busy={busy || !permission.enabled}
                  onChange={allowed => setTool(group.name, allowed)}
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
