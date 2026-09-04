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
import {ShieldCheck} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {EmptyState} from "@/components/shared/empty-state";
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
import {counted, folderOf, formatBytes} from "@/lib/agent-configs";
import type {AgentConfigPlanItem, UnmanagedSkill} from "@/types";

/** The key one scanned skill is selected by: it names one folder on one agent. */
function keyOf(skill: UnmanagedSkill) {
  return `${skill.agentId}/${skill.name}`;
}

/**
 * The skills on this machine that Gateway did not install: written by hand,
 * installed by another tool, or left over from a Gateway whose records are
 * gone. They work as they are, which is why nothing else on the page points
 * them out - what they lack is a source, so the listing cannot say whether one
 * is the current version and Update has nothing to copy from.
 *
 * Importing one records it against the source it was recognized in, which is
 * the same record an install writes. Nothing on disk is touched.
 */
export function UnmanagedSkillsDialog({
  open,
  onOpenChange,
  agentNames,
  onDone,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  agentNames: Map<string, string>;
  onDone: () => void;
}) {
  const [skills, setSkills] = React.useState<UnmanagedSkill[]>([]);
  const [selected, setSelected] = React.useState<string[]>([]);
  const [result, setResult] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    if (!open) {
      return;
    }

    let current = true;
    setSkills([]);
    setSelected([]);
    setResult(null);
    setError("");
    setLoading(true);
    AgentConfigBackend.getUnmanagedSkills()
      .then(res => {
        if (!current) {
          return;
        }
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to scan for untracked skills"));
          return;
        }
        const found = res.data ?? [];
        setSkills(found);
        // A skill holding exactly what a source holds is the safe half of the
        // scan, so those are ticked and the rest are left to be read first.
        setSelected(found.filter(skill => skill.match?.same).map(keyOf));
      })
      .catch(err => current && setError(err.message || String(err)))
      .then(() => current && setLoading(false));

    return () => {
      current = false;
    };
  }, [open]);

  const toggle = (skill: UnmanagedSkill) => {
    const key = keyOf(skill);
    setSelected(previous =>
      previous.includes(key) ? previous.filter(item => item !== key) : [...previous, key],
    );
  };

  const nameOf = (agentId: string) => agentNames.get(agentId) ?? agentId;

  const matched = skills.filter(skill => skill.match !== undefined);
  const unmatched = skills.filter(skill => skill.match === undefined);

  const adopt = () => {
    const picked = skills.filter(skill => skill.match && selected.includes(keyOf(skill)));
    if (picked.length === 0) {
      return;
    }

    setBusy(true);
    setError("");
    AgentConfigBackend.adoptSkills(
      picked[0].owner,
      picked.map(skill => ({
        agentId: skill.agentId,
        name: skill.name,
        sourceId: skill.match?.sourceId ?? "",
        skill: skill.match?.skill ?? "",
      })),
    )
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to import these skills"));
          return;
        }

        const written = res.data ?? [];
        const done = written.filter(item => item.action === "create").length;
        setResult(written);
        Setting.showMessage(
          done === 0 ? "error" : "success",
          counted(done, "agentConfig:Imported 1 skill", "agentConfig:Imported {done} skills", "{done}"),
        );
        onDone();
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  const row = (skill: UnmanagedSkill) => {
    const match = skill.match;
    const key = keyOf(skill);
    return (
      <li key={key} className="flex items-start justify-between gap-3 px-3 py-2 text-sm">
        <label className="flex min-w-0 items-start gap-2">
          <input
            type="checkbox"
            className="accent-primary mt-1 size-4 shrink-0"
            checked={selected.includes(key)}
            disabled={match === undefined || result !== null}
            onChange={() => toggle(skill)}
          />
          <span className="min-w-0">
            <span className="flex flex-wrap items-center gap-2">
              <span className="truncate font-medium">{folderOf(skill.name)}</span>
              <Badge variant="muted">{nameOf(skill.agentId)}</Badge>
              {skill.bytes ? (
                <span className="text-muted-foreground text-xs">{formatBytes(skill.bytes)}</span>
              ) : null}
            </span>
            <span className="text-muted-foreground block text-xs">
              {match === undefined
                ? i18next.t("agentConfig:No source holds this skill")
                : `${match.sourceName} · ${match.skill}`}
            </span>
          </span>
        </label>

        {match === undefined ? null : (
          <Badge variant="muted" className="shrink-0">
            {match.same
              ? i18next.t("agentConfig:Same content")
              : i18next.t("agentConfig:Another version")}
          </Badge>
        )}
      </li>
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] flex-col overflow-hidden sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agentConfig:Untracked skills")}</DialogTitle>
          <DialogDescription>{i18next.t("agentConfig:Untracked skills hint")}</DialogDescription>
        </DialogHeader>

        <div className="scrollbar-thin -mx-1 flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-1 py-0.5">
          {error ? <MessageAlert description={error} /> : null}

          {loading ? <Loading /> : null}

          {!loading && skills.length === 0 && error === "" ? (
            <EmptyState
              icon={ShieldCheck}
              title={i18next.t("agentConfig:Every skill is accounted for")}
              description={i18next.t("agentConfig:Every skill is accounted for hint")}
            />
          ) : null}

          {result !== null ? (
            <ul className="divide-y rounded-md border">
              {result.map(item => (
                <li
                  key={`${item.agentId}/${item.name}`}
                  className="flex items-center justify-between gap-3 px-3 py-1.5 text-sm"
                >
                  <span className="truncate">
                    {folderOf(item.name)}
                    <span className="text-muted-foreground"> → {nameOf(item.agentId)}</span>
                  </span>
                  <span className="flex shrink-0 items-center gap-2">
                    {item.reason ? <span className="text-muted-foreground text-xs">{item.reason}</span> : null}
                    <ActionBadge action={item.action} />
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <>
              {matched.length === 0 ? null : (
                <div className="grid gap-2">
                  <span className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
                    {i18next.t("agentConfig:Recognized in a source")}
                  </span>
                  <ul className="divide-y rounded-md border">{matched.map(row)}</ul>
                </div>
              )}

              {unmatched.length === 0 ? null : (
                <div className="grid gap-2">
                  <span className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
                    {i18next.t("agentConfig:Not in any source")}
                  </span>
                  <ul className="divide-y rounded-md border">{unmatched.map(row)}</ul>
                  <p className="text-muted-foreground text-xs">
                    {i18next.t("agentConfig:Not in any source hint")}
                  </p>
                </div>
              )}
            </>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {result === null ? i18next.t("general:Cancel") : i18next.t("general:Close")}
          </Button>
          {result === null && matched.length > 0 ? (
            <Button onClick={adopt} loading={busy} disabled={busy || selected.length === 0}>
              {selected.length === 0
                ? i18next.t("agentConfig:Import into Gateway")
                : counted(
                  selected.length,
                  "agentConfig:Import 1 skill",
                  "agentConfig:Import {done} skills",
                  "{done}",
                )}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
