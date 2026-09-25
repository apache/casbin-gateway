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
import {ChevronDown, ChevronRight, Globe, KeyRound, Rocket, ShieldCheck, Trash2} from "lucide-react";
import i18next from "i18next";

import * as PermissionBackend from "@/backend/PermissionBackend";
import * as Setting from "@/Setting";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Switch} from "@/components/ui/switch";
import {agentDecidesHere} from "@/lib/agents";
import type {Agent, AgentPermission, PermissionPack} from "@/types";

const packTexts: {[pack: string]: {title: string; hint: string; icon: React.ElementType}} = {
  "no-destructive-commands": {
    title: "agent:No destructive commands",
    hint: "agent:No destructive commands hint",
    icon: Trash2,
  },
  "protect-secrets": {title: "agent:Protect secrets", hint: "agent:Protect secrets hint", icon: KeyRound},
  "egress-allowlist": {title: "agent:Only known hosts", hint: "agent:Only known hosts hint", icon: Globe},
  "no-publishing": {title: "agent:No publishing", hint: "agent:No publishing hint", icon: Rocket},
};

export function packTitle(pack: string) {
  const known = packTexts[pack];
  return known ? i18next.t(known.title) : pack;
}

function packHint(pack: string) {
  const known = packTexts[pack];
  return known ? i18next.t(known.hint) : "";
}

function packIcon(pack: string) {
  return packTexts[pack]?.icon ?? ShieldCheck;
}

function PackGuards({pack}: {pack: PermissionPack}) {
  const [open, setOpen] = React.useState(false);
  const Chevron = open ? ChevronDown : ChevronRight;

  return (
    <div>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
      >
        <Chevron className="h-3.5 w-3.5" />
        {i18next.t("agent:{count} guards").split("{count}").join(String(pack.guards.length))}
      </button>
      {open ? (
        <pre className="mt-1.5 max-h-56 overflow-auto rounded-md bg-muted p-2 text-xs">
          {pack.guards.join("\n")}
        </pre>
      ) : null}
    </div>
  );
}

/** The pack cards on the Permissions page, each with an apply-to-all button. */
export function PermissionPackBoard({
  agents,
  permissions,
  onChanged,
}: {
  agents: Agent[];
  permissions: AgentPermission[];
  onChanged: () => void;
}) {
  const [packs, setPacks] = React.useState<PermissionPack[]>([]);
  const [busy, setBusy] = React.useState("");

  React.useEffect(() => {
    PermissionBackend.getPermissionPacks()
      .then(res => setPacks(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setPacks([]));
  }, []);

  if (packs.length === 0 || agents.length === 0) {
    return null;
  }

  const agentIds = agents.map(agent => agent.agentId);
  const unheld = agents.filter(agent => !agentDecidesHere(agent));
  const hasPack = (agentId: string, pack: string) =>
    permissions.some(permission => permission.name === agentId && permission.enabled && (permission.packs ?? []).includes(pack));

  const setPack = (pack: string, on: boolean) => {
    setBusy(pack);
    PermissionBackend.setPermissionPack(pack, on, agentIds)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t(on ? "agent:Pack applied to all" : "agent:Pack removed from all"));
          onChanged();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agent:Failed to save the permissions"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusy(""));
  };

  return (
    <div className="space-y-2">
      <div>
        <div className="flex items-center gap-1.5 text-sm font-medium">
          <ShieldCheck className="h-4 w-4 text-muted-foreground" />
          {i18next.t("agent:Policy packs")}
        </div>
        <p className="text-sm text-muted-foreground">{i18next.t("agent:Policy packs hint")}</p>
      </div>

      <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
        {packs.map(pack => {
          const Icon = packIcon(pack.name);
          const on = agentIds.filter(agentId => hasPack(agentId, pack.name)).length;
          const all = on === agentIds.length;
          return (
            <div key={pack.name} className="flex flex-col gap-2 rounded-md border p-3">
              <div className="flex items-start gap-2">
                <Icon className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
                    {packTitle(pack.name)}
                    <Badge variant={on === 0 ? "muted" : "success"} className="font-normal">
                      {`${on}/${agentIds.length}`}
                    </Badge>
                  </div>
                  <p className="text-sm text-muted-foreground">{packHint(pack.name)}</p>
                </div>
              </div>
              <div className="mt-auto flex items-center justify-between gap-2">
                <PackGuards pack={pack} />
                <Button
                  variant={all ? "outline" : "default"}
                  size="xs"
                  disabled={busy !== ""}
                  onClick={() => setPack(pack.name, !all)}
                >
                  {i18next.t(all ? "agent:Remove from all agents" : "agent:Apply to all agents")}
                </Button>
              </div>
            </div>
          );
        })}
      </div>

      {unheld.length > 0 ? (
        <p className="text-xs text-muted-foreground">
          {i18next.t("agent:Packs not held hint")} {unheld.map(agent => agent.name).join(", ")}
        </p>
      ) : null}
    </div>
  );
}

export function PackSwitches({
  agent,
  packs,
  permission,
  busy,
  onPacks,
}: {
  agent: Agent;
  packs: PermissionPack[];
  permission: AgentPermission;
  busy: boolean;
  onPacks: (packs: string[]) => void;
}) {
  const current = permission.packs ?? [];

  return (
    <div className="space-y-2">
      <div className="text-sm font-medium">{i18next.t("agent:Policy packs")}</div>
      {agentDecidesHere(agent) ? null : (
        <p className="text-sm text-warning">{i18next.t("agent:Packs not held on this agent")}</p>
      )}
      <div className="rounded-md border">
        {packs.map(pack => {
          const Icon = packIcon(pack.name);
          return (
            <label key={pack.name} className="flex items-start gap-3 border-b p-2.5 text-sm last:border-b-0">
              <Icon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">
                {packTitle(pack.name)}
                <span className="block text-xs text-muted-foreground">{packHint(pack.name)}</span>
              </span>
              <Switch
                className="mt-0.5 shrink-0"
                checked={current.includes(pack.name)}
                disabled={busy}
                onCheckedChange={checked =>
                  onPacks(checked ? [...current, pack.name] : current.filter(name => name !== pack.name))}
              />
            </label>
          );
        })}
      </div>
      <p className="text-xs text-muted-foreground">{i18next.t("agent:Packs card hint")}</p>
    </div>
  );
}
