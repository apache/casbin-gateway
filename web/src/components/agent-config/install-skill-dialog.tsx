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
import {Download, Link2, RefreshCw, Search} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {SkillSourcePanel} from "@/components/agent-config/skill-source-panel";
import {TargetPicker} from "@/components/agent-config/target-picker";
import {Field} from "@/components/shared/form-dialog";
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
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {counted, folderOf, formatBytes, sharedName} from "@/lib/agent-configs";
import {cn} from "@/lib/utils";
import type {
  AgentConfigInventory,
  AgentConfigPlanItem,
  SkillCatalog,
  SkillInstallMode,
  SkillSource,
} from "@/types";

/**
 * Installing skills from outside this machine: pick a source, pick what it
 * holds, pick the agents to put it in. A copy belongs to the agent afterwards;
 * a link keeps every agent on Gateway's copy of the source, so fetching the
 * source again moves all of them at once.
 */
export function InstallSkillDialog({
  open,
  onOpenChange,
  inventories,
  source,
  onDone,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  inventories: AgentConfigInventory[];
  source: AgentConfigInventory;
  onDone: () => void;
}) {
  const [sources, setSources] = React.useState<SkillSource[]>([]);
  const [sourceId, setSourceId] = React.useState("");
  const [catalog, setCatalog] = React.useState<SkillCatalog | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [refreshing, setRefreshing] = React.useState(false);
  const [search, setSearch] = React.useState("");
  const [selected, setSelected] = React.useState<string[]>([]);
  const [targets, setTargets] = React.useState<string[]>([]);
  const [mode, setMode] = React.useState<SkillInstallMode>("copy");
  const [overwrite, setOverwrite] = React.useState(false);
  const [result, setResult] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  // A skill is written into one account's home directory, so the agents on
  // offer are the ones reading that same home.
  const candidates = inventories.filter(
    inventory => inventory.home === source.home && inventory.skillsSupported,
  );

  const loadSources = React.useCallback(
    (selectId?: string) => {
      AgentConfigBackend.getSkillSources(source.owner)
        .then(res => {
          if (res.status !== "ok") {
            setError(res.msg || i18next.t("agentConfig:Failed to read the skill sources"));
            return;
          }
          const list = res.data ?? [];
          setSources(list);
          setSourceId(previous => {
            const wanted = selectId ?? previous;
            return list.some(item => item.id === wanted) ? wanted : (list[0]?.id ?? "");
          });
        })
        .catch(err => setError(err.message || String(err)));
    },
    [source.owner],
  );

  React.useEffect(() => {
    if (open) {
      setSearch("");
      setSelected([]);
      setTargets(source.skillsSupported ? [source.agentId] : []);
      setMode("copy");
      setOverwrite(false);
      setResult(null);
      setError("");
      loadSources();
    }
  }, [open, source.agentId, source.skillsSupported, loadSources]);

  // Reading a catalog downloads the source the first time, so it is asked for
  // only when a source is actually being looked at.
  const readCatalog = React.useCallback(
    (refresh: boolean) => {
      if (sourceId === "") {
        setCatalog(null);
        return;
      }

      if (refresh) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      setError("");
      AgentConfigBackend.getSkillCatalog(source.owner, sourceId, refresh)
        .then(res => {
          if (res.status === "ok") {
            setCatalog(res.data ?? null);
            // Reading the catalog is what fills the store, so the source list
            // only learns its count and its fetch time from this.
            loadSources(sourceId);
          } else {
            setCatalog(null);
            setError(res.msg || i18next.t("agentConfig:Failed to read this source"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => {
          setLoading(false);
          setRefreshing(false);
        });
    },
    [source.owner, sourceId, loadSources],
  );

  React.useEffect(() => {
    if (open) {
      setSelected([]);
      setResult(null);
      readCatalog(false);
    }
    // readCatalog is left out on purpose: it changes identity whenever the
    // source list is reloaded, and re-reading the catalog on that would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, sourceId]);

  // What the target agents already hold, so a skill that is already there says
  // so before it is picked rather than after the install reports a skip.
  const heldBy = new Map<string, string[]>();
  candidates.forEach(inventory => {
    inventory.skills.forEach(item => {
      const holders = heldBy.get(sharedName(item)) ?? [];
      holders.push(inventory.name);
      heldBy.set(sharedName(item), holders);
    });
  });

  const skills = (catalog?.skills ?? []).filter(skill => {
    const text = search.trim().toLowerCase();
    return (
      text === "" ||
      skill.name.toLowerCase().includes(text) ||
      (skill.description ?? "").toLowerCase().includes(text)
    );
  });
  const allSelected = skills.length > 0 && selected.length === skills.length;

  const toggleOne = (name: string) =>
    setSelected(previous =>
      previous.includes(name) ? previous.filter(item => item !== name) : [...previous, name],
    );

  const toggleTarget = (agentId: string) =>
    setTargets(previous =>
      previous.includes(agentId) ? previous.filter(item => item !== agentId) : [...previous, agentId],
    );

  const install = () => {
    setBusy(true);
    setError("");
    AgentConfigBackend.installSkills({
      owner: source.owner,
      sourceId: sourceId,
      to: targets,
      names: selected,
      mode: mode,
      overwrite: overwrite,
    })
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to install"));
          return;
        }

        const written = res.data ?? [];
        const done = written.filter(item => item.action === "create" || item.action === "overwrite").length;
        const failed = written.filter(item => item.action === "failed").length;
        setResult(written);
        Setting.showMessage(
          failed > 0 || done === 0 ? "error" : "success",
          failed > 0
            ? i18next
              .t("agentConfig:Installed {done}, {failed} failed")
              .replace("{done}", String(done))
              .replace("{failed}", String(failed))
            : counted(done, "agentConfig:Installed 1 skill", "agentConfig:Installed {done} skills", "{done}"),
        );
        onDone();
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  const nameOf = (agentId: string) =>
    candidates.find(inventory => inventory.agentId === agentId)?.name ?? agentId;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[88vh] flex-col gap-4 sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agentConfig:Install skills")}</DialogTitle>
          <DialogDescription>{i18next.t("agentConfig:Install skills hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 gap-3">
          <SkillSourcePanel
            owner={source.owner}
            sources={sources}
            selectedId={sourceId}
            onSelect={setSourceId}
            onChanged={loadSources}
            disabled={busy}
          />

          <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
            <div className="flex items-center gap-2">
              <div className="relative flex-1">
                <Search className="text-muted-foreground absolute top-1/2 left-2 size-4 -translate-y-1/2" />
                <Input
                  value={search}
                  className="h-8 pl-8"
                  placeholder={i18next.t("agentConfig:Search by name")}
                  onChange={event => setSearch(event.target.value)}
                />
              </div>
              <Button
                variant="outline"
                size="sm"
                disabled={sourceId === "" || loading || refreshing}
                onClick={() => readCatalog(true)}
              >
                <RefreshCw className={cn("size-4", refreshing && "animate-spin")} />
                {i18next.t("agentConfig:Fetch again")}
              </Button>
            </div>

            {loading ? (
              <Loading />
            ) : skills.length === 0 ? (
              <p className="text-muted-foreground flex-1 py-8 text-center text-sm">
                {sourceId === ""
                  ? i18next.t("agentConfig:No sources yet")
                  : i18next.t("agentConfig:This source holds no skills")}
              </p>
            ) : (
              <>
                <label className="text-muted-foreground flex items-center gap-2 text-xs">
                  <input
                    type="checkbox"
                    className="accent-primary size-4"
                    checked={allSelected}
                    onChange={() => setSelected(allSelected ? [] : skills.map(skill => skill.name))}
                  />
                  {i18next.t("agentConfig:Select all")}
                  <span className="ml-auto">
                    {counted(
                      skills.length,
                      "agentConfig:1 skill in this source",
                      "agentConfig:{count} skills in this source",
                    )}
                  </span>
                </label>

                <div className="scrollbar-thin min-h-0 flex-1 divide-y overflow-y-auto rounded-md border">
                  {skills.map(skill => {
                    const holders = heldBy.get(folderOf(skill.name)) ?? [];
                    return (
                      <label
                        key={skill.name}
                        className="hover:bg-accent flex cursor-pointer items-start gap-2.5 px-3 py-2"
                      >
                        <input
                          type="checkbox"
                          className="accent-primary mt-0.5 size-4 shrink-0"
                          checked={selected.includes(skill.name)}
                          onChange={() => toggleOne(skill.name)}
                        />
                        <span className="flex min-w-0 flex-col">
                          <span className="flex flex-wrap items-center gap-1.5">
                            <span className="truncate text-sm font-medium">{folderOf(skill.name)}</span>
                            {skill.name.includes("/") ? (
                              <Badge variant="muted">{skill.name.slice(0, skill.name.lastIndexOf("/"))}</Badge>
                            ) : null}
                            {holders.length > 0 ? (
                              <SimpleTooltip
                                title={i18next
                                  .t("agentConfig:Already installed in")
                                  .replace("{agents}", holders.join(", "))}
                              >
                                <Badge variant="info">{i18next.t("agentConfig:Already installed")}</Badge>
                              </SimpleTooltip>
                            ) : null}
                          </span>
                          {skill.description ? (
                            <span className="text-muted-foreground line-clamp-2 text-xs">
                              {skill.description}
                            </span>
                          ) : null}
                          <span className="text-muted-foreground text-xs">
                            {counted(skill.files ?? 0, "agentConfig:1 file", "agentConfig:{files} files", "{files}")}
                            {skill.bytes ? ` · ${formatBytes(skill.bytes)}` : ""}
                          </span>
                        </span>
                      </label>
                    );
                  })}
                </div>
              </>
            )}
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={i18next.t("agentConfig:Install into")} hint={i18next.t("agentConfig:Install into hint")}>
            <TargetPicker
              candidates={candidates}
              kind="skill"
              selected={targets}
              onToggle={toggleTarget}
              disabled={busy}
            />
          </Field>

          <Field
            label={i18next.t("agentConfig:How to install")}
            hint={
              mode === "copy"
                ? i18next.t("agentConfig:Copy mode hint")
                : i18next.t("agentConfig:Link mode hint")
            }
          >
            <div className="flex flex-wrap gap-2">
              {(["copy", "link"] as SkillInstallMode[]).map(option => (
                <button
                  key={option}
                  type="button"
                  disabled={busy}
                  onClick={() => setMode(option)}
                  className={cn(
                    "flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors",
                    mode === option ? "border-primary bg-primary/10" : "hover:bg-accent",
                    busy && "cursor-not-allowed opacity-50",
                  )}
                >
                  {option === "copy" ? <Download className="size-4" /> : <Link2 className="size-4" />}
                  {option === "copy"
                    ? i18next.t("agentConfig:Copy into the agent")
                    : i18next.t("agentConfig:Link to Gateway's copy")}
                </button>
              ))}
            </div>
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={overwrite} onCheckedChange={setOverwrite} disabled={busy} />
              <span>{i18next.t("agentConfig:Replace items that already exist")}</span>
            </label>
          </Field>
        </div>

        {error ? <MessageAlert description={error} /> : null}

        {result === null ? null : (
          <ul className="scrollbar-thin max-h-32 divide-y overflow-y-auto rounded-md border">
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
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {i18next.t("general:Close")}
          </Button>
          <Button
            onClick={install}
            loading={busy}
            disabled={busy || selected.length === 0 || targets.length === 0}
          >
            {mode === "link" ? <Link2 className="size-4" /> : <Download className="size-4" />}
            {selected.length === 0
              ? i18next.t("agentConfig:Install skills")
              : counted(selected.length, "agentConfig:Install 1 skill", "agentConfig:Install {count} skills")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
