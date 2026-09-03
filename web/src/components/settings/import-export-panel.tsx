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
import {Download, Upload, X} from "lucide-react";
import i18next from "i18next";

import * as BackupBackend from "@/backend/BackupBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {SimpleSelect} from "@/components/shared/simple-select";
import {
  CountsBadges,
  countsOf,
  downloadSnapshot,
  FULL_SCOPE,
  ImportReportView,
  isScopeEmpty,
  ScopePicker,
} from "@/components/settings/snapshot";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import type {ImportMode, ImportReport, Snapshot, SnapshotScope} from "@/types";

/** The file name an export downloads under: the host it came from and the day. */
function exportFileName(snapshot: Snapshot) {
  const day = (snapshot.createdTime ?? "").slice(0, 10).replace(/-/g, "") || "export";
  const host = (snapshot.host ?? "gateway").replace(/[^A-Za-z0-9_-]/g, "-");
  return `casbin-gateway-${host}-${day}.json`;
}

/** A file is only a snapshot if it says which version and which sections it is. */
function readSnapshot(text: string) {
  const parsed = JSON.parse(text) as Snapshot;
  if (typeof parsed?.version !== "number" || parsed?.scope === undefined) {
    throw new Error(i18next.t("setting:This file is not a Gateway snapshot"));
  }
  return parsed;
}

/** The sections a file actually carries, so nothing else can be asked for. */
function scopeOfFile(snapshot: Snapshot): SnapshotScope {
  return {...snapshot.scope, secrets: snapshot.scope.secrets};
}

export function ImportExportPanel({onSettingChanged}: {onSettingChanged: () => void}) {
  const [exportScope, setExportScope] = React.useState<SnapshotScope>(FULL_SCOPE);
  const [exporting, setExporting] = React.useState(false);

  const [fileName, setFileName] = React.useState("");
  const [snapshot, setSnapshot] = React.useState<Snapshot | null>(null);
  const [importScope, setImportScope] = React.useState<SnapshotScope>(FULL_SCOPE);
  const [mode, setMode] = React.useState<ImportMode>("merge");
  const [preview, setPreview] = React.useState<ImportReport | null>(null);
  const [result, setResult] = React.useState<ImportReport | null>(null);
  const [error, setError] = React.useState("");
  const [importing, setImporting] = React.useState(false);
  const fileInput = React.useRef<HTMLInputElement>(null);

  const runExport = () => {
    setExporting(true);
    BackupBackend.exportSnapshot(exportScope)
      .then(res => {
        if (res.status === "ok" && res.data) {
          downloadSnapshot(res.data, exportFileName(res.data));
          Setting.showMessage("success", i18next.t("setting:The snapshot was saved"));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setExporting(false));
  };

  const clearFile = () => {
    setFileName("");
    setSnapshot(null);
    setPreview(null);
    setResult(null);
    setError("");
    if (fileInput.current !== null) {
      fileInput.current.value = "";
    }
  };

  const pickFile = (file: File | undefined) => {
    if (file === undefined) {
      return;
    }
    setResult(null);
    setError("");
    file
      .text()
      .then(text => {
        const parsed = readSnapshot(text);
        setSnapshot(parsed);
        setFileName(file.name);
        setImportScope(scopeOfFile(parsed));
        setMode("merge");
      })
      .catch(failure => {
        clearFile();
        setError(failure.message || String(failure));
      });
  };

  // The preview is the same walk the import makes, so what the page shows is
  // what will happen rather than a guess at it.
  React.useEffect(() => {
    if (snapshot === null || isScopeEmpty(importScope)) {
      setPreview(null);
      return;
    }

    let current = true;
    BackupBackend.importSnapshot(snapshot, importScope, mode, true)
      .then(res => {
        if (!current) {
          return;
        }
        if (res.status === "ok" && res.data) {
          setPreview(res.data);
          setError("");
        } else {
          setPreview(null);
          setError(res.msg);
        }
      })
      .catch(failure => {
        if (current) {
          setPreview(null);
          setError(failure.message || String(failure));
        }
      });
    return () => {
      current = false;
    };
  }, [snapshot, importScope, mode]);

  const runImport = () => {
    if (snapshot === null) {
      return Promise.resolve();
    }
    setImporting(true);
    return BackupBackend.importSnapshot(snapshot, importScope, mode, false)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setResult(res.data);
          Setting.showMessage("success", i18next.t("setting:The snapshot was imported"));
          onSettingChanged();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setImporting(false));
  };

  const counts = countsOf(snapshot);
  const deletes = mode === "replace" && (preview?.deleted ?? 0) > 0;

  return (
    <Card className="gap-4 py-5">
      <CardHeader className="px-5">
        <CardTitle className="text-[15px]">{i18next.t("setting:Import and export")}</CardTitle>
        <CardDescription>{i18next.t("setting:Import and export description")}</CardDescription>
      </CardHeader>

      <CardContent className="grid gap-6 px-5 md:grid-cols-2">
        <div className="flex flex-col gap-3">
          <div className="text-sm font-medium">{i18next.t("setting:Export")}</div>
          <ScopePicker
            scope={exportScope}
            onChange={setExportScope}
            disabled={exporting}
            secretsLabel={i18next.t("setting:Include the API keys")}
            secretsHint={i18next.t("setting:Include the API keys hint")}
          />
          {exportScope.secrets ? (
            <MessageAlert
              variant="warning"
              title={i18next.t("setting:The file carries every key")}
              description={i18next.t("setting:The file carries every key hint")}
            />
          ) : null}
          <div>
            <Button onClick={runExport} loading={exporting} disabled={isScopeEmpty(exportScope)}>
              <Download />
              {i18next.t("setting:Export to a file")}
            </Button>
          </div>
        </div>

        <div className="flex flex-col gap-3">
          <div className="text-sm font-medium">{i18next.t("setting:Import")}</div>

          <input
            ref={fileInput}
            type="file"
            accept="application/json,.json"
            className="hidden"
            onChange={event => pickFile(event.target.files?.[0])}
          />

          {snapshot === null ? (
            <>
              <p className="text-muted-foreground text-sm">{i18next.t("setting:Import hint")}</p>
              <div>
                <Button variant="outline" onClick={() => fileInput.current?.click()}>
                  <Upload />
                  {i18next.t("setting:Choose a file")}
                </Button>
              </div>
            </>
          ) : (
            <>
              <div className="flex flex-col gap-2 rounded-md border p-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate font-mono text-xs">{fileName}</div>
                    <div className="text-muted-foreground text-xs">
                      {i18next
                        .t("setting:Taken on {host} at {time}")
                        .replace("{host}", snapshot.host || "?")
                        .replace("{time}", Setting.getFormattedDate(snapshot.createdTime) ?? "?")}
                    </div>
                  </div>
                  <Button variant="ghost" size="icon-xs" onClick={clearFile} aria-label={i18next.t("general:Cancel")}>
                    <X />
                  </Button>
                </div>
                <CountsBadges counts={counts} />
                {snapshot.scope.secrets ? null : (
                  <span className="text-muted-foreground text-xs">
                    {i18next.t("setting:This file carries no keys")}
                  </span>
                )}
              </div>

              <ScopePicker
                scope={importScope}
                onChange={setImportScope}
                available={counts}
                disabled={importing}
                secretsLabel={i18next.t("setting:Include the API keys")}
                secretsHint={i18next.t("setting:Keys are taken from the file")}
              />

              <div className="grid gap-1.5">
                <span className="text-muted-foreground text-xs">{i18next.t("setting:What to do with what is already here")}</span>
                <SimpleSelect
                  size="sm"
                  value={mode}
                  disabled={importing}
                  onChange={value => setMode(value as ImportMode)}
                  options={[
                    {label: i18next.t("setting:Keep what is here"), value: "merge"},
                    {label: i18next.t("setting:Overwrite what the file names"), value: "overwrite"},
                    {label: i18next.t("setting:Replace it entirely"), value: "replace"},
                  ]}
                />
              </div>

              {error === "" ? null : <MessageAlert title={error} />}

              {result === null && preview !== null ? (
                <div className="flex flex-col gap-2">
                  <span className="text-muted-foreground text-xs">{i18next.t("setting:What this import would do")}</span>
                  <ImportReportView report={preview} />
                </div>
              ) : null}

              {result === null ? null : (
                <div className="flex flex-col gap-2">
                  <MessageAlert
                    variant="success"
                    title={i18next.t("setting:The snapshot was imported")}
                    description={
                      result.backup === ""
                        ? undefined
                        : i18next.t("setting:A backup was taken first").replace("{name}", result.backup)
                    }
                  />
                  <ImportReportView report={result} />
                </div>
              )}

              <div className="flex gap-2">
                <ConfirmDialog
                  title={i18next.t("setting:Import this snapshot?")}
                  description={
                    deletes
                      ? i18next
                        .t("setting:Replace warning")
                        .replace("{count}", String(preview?.deleted ?? 0))
                      : i18next.t("setting:Import warning")
                  }
                  confirmText={i18next.t("setting:Import")}
                  variant={deletes ? "destructive" : "default"}
                  onConfirm={runImport}
                  disabled={importing || preview === null}
                >
                  <Button loading={importing} disabled={preview === null}>
                    <Upload />
                    {i18next.t("setting:Import")}
                  </Button>
                </ConfirmDialog>
                <Button variant="outline" onClick={() => fileInput.current?.click()} disabled={importing}>
                  {i18next.t("setting:Choose another file")}
                </Button>
              </div>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
