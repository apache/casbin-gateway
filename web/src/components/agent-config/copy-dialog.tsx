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
import {ArrowRight} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {TargetPicker} from "@/components/agent-config/target-picker";
import {Loading} from "@/components/shared/loading";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Switch} from "@/components/ui/switch";
import {counted, inventoryKey} from "@/lib/agent-configs";
import type {AgentConfigInventory, AgentConfigKind, AgentConfigPlanItem} from "@/types";

/**
 * The migration step: pick the agents to copy into, see what would change at
 * each of them, then apply. The preview is what makes an overwrite a decision
 * rather than a surprise, so it is loaded before the button does anything.
 */
export function CopyDialog({
  open,
  onOpenChange,
  kind,
  source,
  inventories,
  names,
  onDone,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  kind: AgentConfigKind;
  source: AgentConfigInventory;
  inventories: AgentConfigInventory[];
  names: string[];
  onDone: () => void;
}) {
  const [selected, setSelected] = React.useState<string[]>([]);
  const [overwrite, setOverwrite] = React.useState(false);
  const [plan, setPlan] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [planning, setPlanning] = React.useState(false);
  const [applied, setApplied] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  // Copying moves files inside one account's home directory, so an agent
  // configured under another home is not a target this page can offer.
  const candidates = inventories.filter(
    inventory => inventory.home === source.home && inventory.agentId !== source.agentId,
  );

  React.useEffect(() => {
    if (open) {
      setSelected([]);
      setOverwrite(false);
      setPlan(null);
      setApplied(false);
      setError("");
    }
  }, [open, kind, source.agentId]);

  React.useEffect(() => {
    if (!open || applied || selected.length === 0) {
      setPlan(null);
      return;
    }

    let current = true;
    setPlanning(true);
    setError("");
    AgentConfigBackend.planAgentConfigCopy({
      owner: source.owner,
      from: source.agentId,
      to: selected,
      kind: kind,
      names: names,
      overwrite: overwrite,
    })
      .then(res => {
        if (!current) {
          return;
        }
        if (res.status === "ok") {
          setPlan(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("agentConfig:Failed to plan this copy"));
        }
      })
      .catch(err => current && setError(err.message || String(err)))
      .then(() => current && setPlanning(false));

    return () => {
      current = false;
    };
  }, [open, applied, selected, overwrite, kind, names, source.owner, source.agentId]);

  const toggleTarget = (agentId: string) => {
    setSelected(previous =>
      previous.includes(agentId) ? previous.filter(item => item !== agentId) : [...previous, agentId],
    );
  };

  const changes = (plan ?? []).filter(item => item.action === "create" || item.action === "overwrite");

  const apply = () => {
    setBusy(true);
    setError("");
    AgentConfigBackend.copyAgentConfig({
      owner: source.owner,
      from: source.agentId,
      to: selected,
      kind: kind,
      names: names,
      overwrite: overwrite,
    })
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to copy"));
          return;
        }

        const result = res.data ?? [];
        const written = result.filter(item => item.action === "create" || item.action === "overwrite").length;
        const failed = result.filter(item => item.action === "failed").length;
        setPlan(result);
        setApplied(true);
        Setting.showMessage(
          failed > 0 ? "error" : "success",
          failed > 0
            ? i18next
              .t("agentConfig:Copied {done}, {failed} failed")
              .replace("{done}", String(written))
              .replace("{failed}", String(failed))
            : counted(written, "agentConfig:Copied 1 item", "agentConfig:Copied {done} items", "{done}"),
        );
        onDone();
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  const byTarget = candidates
    .filter(inventory => selected.includes(inventory.agentId))
    .map(inventory => ({
      inventory: inventory,
      items: (plan ?? []).filter(item => item.agentId === inventory.agentId),
    }));

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] gap-4 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>
            {kind === "skill"
              ? counted(names.length, "agentConfig:Copy 1 skill", "agentConfig:Copy {count} skills")
              : kind === "prompt"
                ? i18next.t("agentConfig:Copy these instructions")
                : counted(names.length, "agentConfig:Copy 1 MCP server", "agentConfig:Copy {count} MCP servers")}
          </DialogTitle>
          <DialogDescription className="flex flex-wrap items-center gap-1.5">
            {i18next.t("agentConfig:From")}
            <Badge variant="muted">{source.name}</Badge>
            <ArrowRight className="size-3.5" />
            {i18next.t("agentConfig:Pick the agents to copy into")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 overflow-y-auto">
          {candidates.length === 0 ? (
            <MessageAlert
              variant="info"
              description={i18next.t("agentConfig:No other agent on this machine can receive these")}
            />
          ) : null}

          <TargetPicker
            candidates={candidates}
            kind={kind}
            selected={selected}
            onToggle={toggleTarget}
            disabled={applied}
          />

          {!applied && candidates.length > 0 ? (
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={overwrite} onCheckedChange={setOverwrite} />
              <span>{i18next.t("agentConfig:Replace items that already exist")}</span>
            </label>
          ) : null}

          {error ? <MessageAlert description={error} /> : null}
          {planning ? <Loading /> : null}

          {byTarget.map(({inventory, items}) => (
            <div key={inventoryKey(inventory)} className="rounded-md border">
              <div className="bg-muted/50 flex items-center justify-between gap-2 border-b px-3 py-2 text-sm font-medium">
                <span>{inventory.name}</span>
                <span className="text-muted-foreground text-xs font-normal">
                  {items.filter(item => item.action !== "skip" && item.action !== "failed").length}
                  {" / "}
                  {items.length}
                </span>
              </div>
              <ul className="divide-y">
                {items.map(item => (
                  <li key={item.name} className="flex items-center justify-between gap-3 px-3 py-1.5 text-sm">
                    <span className="truncate font-mono text-xs">{item.name}</span>
                    <span className="flex shrink-0 items-center gap-2">
                      {item.reason ? (
                        <span className="text-muted-foreground text-xs">{item.reason}</span>
                      ) : null}
                      <ActionBadge action={item.action} />
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {applied ? i18next.t("general:Close") : i18next.t("general:Cancel")}
          </Button>
          {applied ? null : (
            <Button onClick={apply} disabled={busy || planning || changes.length === 0}>
              {counted(changes.length, "agentConfig:Copy 1 item", "agentConfig:Copy {count} items")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
