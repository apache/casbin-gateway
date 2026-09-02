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
import {FileArchive, FolderOpen, Github, Plus, Trash2, Upload, X} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {formatModified} from "@/lib/agent-configs";
import {cn} from "@/lib/utils";
import type {SkillSource, SkillSourceKind} from "@/types";

const kindIcons: Record<SkillSourceKind, React.ComponentType<{className?: string}>> = {
  github: Github,
  archive: FileArchive,
  upload: Upload,
  local: FolderOpen,
};

/** What one source is, in the reader's words rather than in the API's. */
export function sourceKindLabel(kind: SkillSourceKind) {
  const labels: Record<SkillSourceKind, string> = {
    github: "agentConfig:GitHub repository",
    archive: "agentConfig:Archive URL",
    upload: "agentConfig:Uploaded archive",
    local: "agentConfig:Local folder",
  };
  return i18next.t(labels[kind] ?? "agentConfig:GitHub repository");
}

/** The one line under a source's name: where it is, and what came of it. */
function sourceSubtitle(source: SkillSource) {
  const where = source.kind === "upload" ? sourceKindLabel(source.kind) : source.url || "";
  const ref = source.ref ? `#${source.ref}` : "";
  const inside = source.subdir ? `/${source.subdir}` : "";
  return `${where}${ref}${inside}`;
}

/**
 * The list of places skills are installed from, and the form that adds one.
 * Anything with a SKILL.md in it can be a source: a GitHub repository, an
 * archive at a URL or on this machine, or a folder that is already here.
 */
export function SkillSourcePanel({
  owner,
  sources,
  selectedId,
  onSelect,
  onChanged,
  disabled = false,
}: {
  owner: string;
  sources: SkillSource[];
  selectedId: string;
  onSelect: (id: string) => void;
  onChanged: (selectId?: string) => void;
  disabled?: boolean;
}) {
  const [adding, setAdding] = React.useState(false);
  const [kind, setKind] = React.useState<SkillSourceKind>("github");
  const [url, setUrl] = React.useState("");
  const [ref, setRef] = React.useState("");
  const [subdir, setSubdir] = React.useState("");
  const [name, setName] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const file = React.useRef<HTMLInputElement>(null);

  const reset = () => {
    setUrl("");
    setRef("");
    setSubdir("");
    setName("");
  };

  const added = (source: SkillSource | undefined, message: string) => {
    Setting.showMessage("success", message);
    setAdding(false);
    reset();
    onChanged(source?.id);
  };

  const add = () => {
    setBusy(true);
    AgentConfigBackend.addSkillSource({
      owner: owner,
      kind: kind,
      url: url.trim(),
      ref: ref.trim(),
      subdir: subdir.trim(),
      name: name.trim(),
    })
      .then(res => {
        if (res.status === "ok") {
          added(res.data, i18next.t("agentConfig:Source added"));
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to add this source"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusy(false));
  };

  const uploadArchive = (picked: File) => {
    setBusy(true);
    AgentConfigBackend.uploadSkillSource(owner, picked)
      .then(res => {
        if (res.status === "ok") {
          added(res.data, i18next.t("agentConfig:Source added"));
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to add this source"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => {
        setBusy(false);
        if (file.current) {
          file.current.value = "";
        }
      });
  };

  const remove = (source: SkillSource) =>
    AgentConfigBackend.deleteSkillSource(owner, source.id)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", `${i18next.t("agentConfig:Source removed")}: ${source.name}`);
          onChanged();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to remove this source"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)));

  const ready = kind === "upload" ? false : url.trim() !== "";

  return (
    <div className="flex min-h-0 w-64 shrink-0 flex-col gap-2 border-r pr-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">{i18next.t("agentConfig:Sources")}</span>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={i18next.t("agentConfig:Add source")}
          disabled={disabled}
          onClick={() => setAdding(previous => !previous)}
        >
          {adding ? <X className="size-4" /> : <Plus className="size-4" />}
        </Button>
      </div>

      <div className="scrollbar-thin flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto">
        {sources.map(source => {
          const Icon = kindIcons[source.kind] ?? Github;
          const active = source.id === selectedId;
          return (
            <div
              key={source.id}
              className={cn(
                "group flex items-start gap-2 rounded-md border px-2 py-1.5 transition-colors",
                active ? "border-primary bg-primary/10" : "hover:bg-accent",
              )}
            >
              <button
                type="button"
                disabled={disabled}
                onClick={() => onSelect(source.id)}
                className="flex min-w-0 flex-1 items-start gap-2 text-left"
              >
                <Icon className="text-muted-foreground mt-0.5 size-4 shrink-0" />
                <span className="flex min-w-0 flex-col">
                  <span className="flex items-center gap-1.5">
                    <span className="truncate text-sm leading-tight font-medium">{source.name}</span>
                    {source.builtin ? (
                      <Badge variant="muted">{i18next.t("agentConfig:Built in")}</Badge>
                    ) : null}
                  </span>
                  <span className="text-muted-foreground truncate text-xs" title={sourceSubtitle(source)}>
                    {sourceSubtitle(source)}
                  </span>
                  {source.fetchedAt ? (
                    <span className="text-muted-foreground text-xs">
                      {i18next
                        .t("agentConfig:{count} skills, fetched {when}")
                        .replace("{count}", String(source.skills ?? 0))
                        .replace("{when}", formatModified(source.fetchedAt))}
                    </span>
                  ) : null}
                </span>
              </button>
              <ConfirmDialog
                title={i18next.t("agentConfig:Remove this source?")}
                description={i18next.t("agentConfig:Remove source description")}
                onConfirm={() => remove(source)}
                disabled={disabled}
              >
                <SimpleTooltip title={i18next.t("agentConfig:Remove source")}>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    className="text-destructive opacity-0 group-hover:opacity-100"
                    aria-label={i18next.t("agentConfig:Remove source")}
                    disabled={disabled}
                  >
                    <Trash2 className="size-3.5" />
                  </Button>
                </SimpleTooltip>
              </ConfirmDialog>
            </div>
          );
        })}
        {sources.length === 0 && !adding ? (
          <p className="text-muted-foreground px-1 py-4 text-xs">{i18next.t("agentConfig:No sources yet")}</p>
        ) : null}
      </div>

      {adding ? (
        <div className="flex flex-col gap-2 rounded-md border p-2">
          <div className="grid grid-cols-2 gap-1">
            {(["github", "archive", "local", "upload"] as SkillSourceKind[]).map(option => {
              const Icon = kindIcons[option];
              return (
                <button
                  key={option}
                  type="button"
                  onClick={() => setKind(option)}
                  className={cn(
                    "flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-xs transition-colors",
                    kind === option ? "border-primary bg-primary/10" : "hover:bg-accent",
                  )}
                >
                  <Icon className="size-3.5 shrink-0" />
                  <span className="truncate">{sourceKindLabel(option)}</span>
                </button>
              );
            })}
          </div>

          {kind === "upload" ? (
            <>
              <input
                ref={file}
                type="file"
                accept=".zip,.tgz,.gz,.tar.gz"
                className="hidden"
                onChange={event => {
                  const picked = event.target.files?.[0];
                  if (picked) {
                    uploadArchive(picked);
                  }
                }}
              />
              <Button variant="outline" size="sm" loading={busy} onClick={() => file.current?.click()}>
                <Upload className="size-4" />
                {i18next.t("agentConfig:Choose a .zip")}
              </Button>
            </>
          ) : (
            <>
              <Input
                value={url}
                className="h-8"
                placeholder={
                  kind === "github"
                    ? "anthropics/skills"
                    : kind === "archive"
                      ? "https://example.com/skills.zip"
                      : "/home/me/my-skills"
                }
                onChange={event => setUrl(event.target.value)}
              />
              {kind === "github" ? (
                <div className="flex gap-2">
                  <Input
                    value={ref}
                    className="h-8"
                    placeholder={i18next.t("agentConfig:Branch")}
                    onChange={event => setRef(event.target.value)}
                  />
                  <Input
                    value={subdir}
                    className="h-8"
                    placeholder={i18next.t("agentConfig:Subfolder")}
                    onChange={event => setSubdir(event.target.value)}
                  />
                </div>
              ) : null}
              <Input
                value={name}
                className="h-8"
                placeholder={i18next.t("agentConfig:Display name, optional")}
                onChange={event => setName(event.target.value)}
              />
              <Button size="sm" loading={busy} disabled={!ready} onClick={add}>
                {i18next.t("agentConfig:Add source")}
              </Button>
            </>
          )}

          <p className="text-muted-foreground text-xs">
            {kind === "github"
              ? i18next.t("agentConfig:GitHub source hint")
              : kind === "local"
                ? i18next.t("agentConfig:Local source hint")
                : kind === "upload"
                  ? i18next.t("agentConfig:Upload source hint")
                  : i18next.t("agentConfig:Archive source hint")}
          </p>
        </div>
      ) : null}
    </div>
  );
}
