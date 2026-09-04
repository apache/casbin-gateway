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
import {Archive, Download, FolderSymlink, History, Trash2} from "lucide-react";
import i18next from "i18next";

import * as BackupBackend from "@/backend/BackupBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {SimpleSelect} from "@/components/shared/simple-select";
import {CountsBadges, downloadSnapshot, ImportReportView} from "@/components/settings/snapshot";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import type {Backup, BackupState, ImportReport} from "@/types";

/** How often a backup is taken. The configuration only changes when somebody
 *  changes it, so even the shortest of these is often. */
const INTERVALS = [6, 24, 168];
const RETENTIONS = [5, 10, 20, 50];

function intervalLabel(hours: number) {
  if (hours === 24) {
    return i18next.t("setting:Once a day");
  }
  if (hours === 168) {
    return i18next.t("setting:Once a week");
  }
  return i18next.t("setting:Every {count} hours").replace("{count}", String(hours));
}

/** The presets plus whatever is stored, so a value set in the configuration
 *  file rather than here still shows in the dropdown instead of leaving it blank. */
function withCurrent(presets: number[], current: number, label: (value: number) => string) {
  const values = presets.includes(current) ? presets : [...presets, current].sort((a, b) => a - b);
  return values.map(value => ({label: label(value), value: String(value)}));
}

function reasonLabel(reason: string) {
  switch (reason) {
  case "schedule":
    return i18next.t("setting:On a schedule");
  case "before-import":
    return i18next.t("setting:Before an import");
  default:
    return i18next.t("setting:By hand");
  }
}

function formatSize(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** One file, with everything that decides whether it is the one to restore. */
function BackupRow({
  backup,
  busy,
  onDownload,
  onRestore,
  onDelete,
  preview,
}: {
  backup: Backup;
  busy: boolean;
  onDownload: (backup: Backup) => void;
  onRestore: (backup: Backup) => void | Promise<unknown>;
  onDelete: (backup: Backup) => void | Promise<unknown>;
  preview: (backup: Backup) => void;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3 border-b px-3 py-2.5 last:border-b-0">
      <div className="flex min-w-0 flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-xs">{backup.name}</span>
          <Badge variant="muted" className="font-normal">
            {reasonLabel(backup.reason)}
          </Badge>
          {backup.secrets ? null : (
            <Badge variant="warning" className="font-normal">
              {i18next.t("setting:No keys")}
            </Badge>
          )}
        </div>
        <div className="text-muted-foreground text-xs">
          {Setting.getFormattedDate(backup.createdTime)} · {formatSize(backup.size)}
          {backup.gateway === "" ? "" : ` · ${backup.gateway}`}
        </div>
        <CountsBadges counts={backup.counts} />
      </div>

      <div className="flex shrink-0 items-center gap-1.5">
        <Button variant="outline" size="xs" disabled={busy} onClick={() => onDownload(backup)}>
          <Download />
          {i18next.t("setting:Download")}
        </Button>
        <ConfirmDialog
          title={i18next.t("setting:Restore this backup?")}
          description={i18next.t("setting:Restore warning")}
          confirmText={i18next.t("setting:Restore")}
          onConfirm={() => onRestore(backup)}
          onOpenChange={open => {
            if (open) {
              preview(backup);
            }
          }}
          disabled={busy}
        >
          <Button variant="outline" size="xs" disabled={busy}>
            <History />
            {i18next.t("setting:Restore")}
          </Button>
        </ConfirmDialog>
        <ConfirmDialog
          title={i18next.t("setting:Delete this backup?")}
          confirmText={i18next.t("general:Delete")}
          onConfirm={() => onDelete(backup)}
          disabled={busy}
        >
          <Button variant="ghost" size="icon-xs" disabled={busy} aria-label={i18next.t("general:Delete")}>
            <Trash2 />
          </Button>
        </ConfirmDialog>
      </div>
    </div>
  );
}

/**
 * The snapshots of the configuration Gateway keeps on this machine: whether it
 * takes them on its own, how many it keeps, and the files themselves. A backup
 * is also taken in front of every import, so the restore below is what undoes
 * one that turned out to be the wrong file.
 */
export function BackupPanel({onSettingChanged, reloadToken = 0}: {onSettingChanged: () => void; reloadToken?: number}) {
  const [state, setState] = React.useState<BackupState | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [report, setReport] = React.useState<ImportReport | null>(null);
  // Null while the field shows what is stored, a string once it is being
  // edited: the directory only moves when the button beside it is pressed.
  const [dir, setDir] = React.useState<string | null>(null);

  const load = React.useCallback(() => {
    BackupBackend.getBackupState()
      .then(res => {
        if (res.status === "ok") {
          setState(res.data);
        }
      })
      .catch(() => undefined);
  }, []);

  // A sync that pulled backups from the cloud target changes this list, and the
  // panel that did it says so by bumping the token.
  React.useEffect(load, [load, reloadToken]);

  if (state === null) {
    return null;
  }

  const auto = state.mode === "auto";

  // Every one of these answers with the state, so the list and the schedule are
  // never a reload behind what was just done.
  const apply = (promise: Promise<{status: string; msg: string; data?: BackupState}>) => {
    setBusy(true);
    return promise
      .then(res => {
        if (res.status === "ok" && res.data) {
          setState(res.data);
          // The schedule is stored on the same row the form above edits, so the
          // form is told rather than left holding what it read on load.
          onSettingChanged();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setBusy(false));
  };

  const schedule = (mode: BackupState["mode"], intervalHours: number, retention: number, directory?: string) =>
    apply(BackupBackend.updateBackupSchedule(mode, intervalHours, retention, directory ?? state.dir)).then(() =>
      setDir(null),
    );

  const backUpNow = () => {
    setReport(null);
    return apply(BackupBackend.createBackup());
  };

  const download = (backup: Backup) => {
    BackupBackend.getBackup(backup.name)
      .then(res => {
        if (res.status === "ok" && res.data) {
          downloadSnapshot(res.data, backup.name);
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)));
  };

  // Asked as the confirmation opens, so what the dialog is agreeing to is on
  // the page behind it rather than described in the abstract.
  const previewRestore = (backup: Backup) => {
    setReport(null);
    BackupBackend.restoreBackup(backup.name, true)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setReport(res.data);
        }
      })
      .catch(() => undefined);
  };

  const restore = (backup: Backup) => {
    setBusy(true);
    return BackupBackend.restoreBackup(backup.name, false)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setReport(res.data);
          Setting.showMessage("success", i18next.t("setting:The backup was restored"));
          onSettingChanged();
          load();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setBusy(false));
  };

  const remove = (backup: Backup) => apply(BackupBackend.deleteBackup(backup.name));

  const fact = (label: string, value: React.ReactNode) => (
    <div className="flex flex-col gap-0.5">
      <span className="text-muted-foreground text-xs">{i18next.t(label)}</span>
      <span className="text-sm">{value}</span>
    </div>
  );

  return (
    <Card className="gap-4 py-5">
      <CardHeader className="px-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle className="text-[15px]">{i18next.t("setting:Backups")}</CardTitle>
            <CardDescription>{i18next.t("setting:Backups description")}</CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {i18next.t(auto ? "setting:Backups are on" : "setting:Backups are off")}
            </span>
            <Switch
              checked={auto}
              disabled={busy}
              aria-label={i18next.t("setting:Backups")}
              onCheckedChange={checked =>
                schedule(checked ? "auto" : "off", state.intervalHours, state.retention)
              }
            />
          </div>
        </div>
      </CardHeader>

      <CardContent className="flex flex-col gap-4 px-5">
        <div className="grid gap-3 sm:grid-cols-4">
          {fact(
            "setting:Last backup",
            state.takenTime === "" ? i18next.t("setting:Never") : Setting.getFormattedDate(state.takenTime),
          )}
          {fact(
            "setting:Next backup",
            auto
              ? state.nextTime === ""
                ? i18next.t("setting:Due now")
                : Setting.getFormattedDate(state.nextTime)
              : i18next.t("setting:Nothing is scheduled"),
          )}
          <div className="flex flex-col gap-1">
            <span className="text-muted-foreground text-xs">{i18next.t("setting:How often")}</span>
            <SimpleSelect
              size="sm"
              disabled={!auto || busy}
              value={String(state.intervalHours)}
              onChange={value => schedule(state.mode, Number(value), state.retention)}
              options={withCurrent(INTERVALS, state.intervalHours, intervalLabel)}
            />
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-muted-foreground text-xs">{i18next.t("setting:How many to keep")}</span>
            <SimpleSelect
              size="sm"
              disabled={busy}
              value={String(state.retention)}
              onChange={value => schedule(state.mode, state.intervalHours, Number(value))}
              options={withCurrent(RETENTIONS, state.retention, String)}
            />
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <span className="text-muted-foreground text-xs">{i18next.t("setting:Where backups live")}</span>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              className="h-8 max-w-md"
              value={dir ?? state.dir}
              disabled={busy}
              aria-label={i18next.t("setting:Where backups live")}
              onChange={event => setDir(event.target.value)}
            />
            <Button
              variant="outline"
              size="xs"
              disabled={busy || dir === null || dir === state.dir}
              onClick={() => schedule(state.mode, state.intervalHours, state.retention, dir ?? state.dir)}
            >
              {i18next.t("general:Save")}
            </Button>
            {(state.folders ?? []).map(folder => (
              <Button key={folder.path} variant="ghost" size="xs" disabled={busy} onClick={() => setDir(folder.suggested)}>
                <FolderSymlink />
                {folder.name}
              </Button>
            ))}
          </div>
          <span className="text-muted-foreground text-xs">{i18next.t("setting:Where backups live hint")}</span>
        </div>

        {state.error === "" ? null : (
          <MessageAlert title={i18next.t("setting:The last backup failed")} description={state.error} />
        )}

        {report === null ? null : (
          <div className="flex flex-col gap-2">
            <span className="text-muted-foreground text-xs">
              {i18next.t(report.dryRun ? "setting:What this restore would do" : "setting:What this restore did")}
            </span>
            <ImportReportView report={report} />
          </div>
        )}

        <div className="rounded-md border">
          {state.backups.length === 0 ? (
            <div className="text-muted-foreground px-3 py-6 text-center text-sm">
              {i18next.t("setting:No backups yet")}
            </div>
          ) : (
            state.backups.map(backup => (
              <BackupRow
                key={backup.name}
                backup={backup}
                busy={busy}
                onDownload={download}
                onRestore={restore}
                onDelete={remove}
                preview={previewRestore}
              />
            ))
          )}
        </div>

        <div>
          <Button variant="outline" onClick={backUpNow} loading={busy}>
            <Archive />
            {i18next.t("setting:Back up now")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
