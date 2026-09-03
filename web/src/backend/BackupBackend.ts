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

import {query, request} from "@/backend/request";
import type {BackupMode, BackupState, ImportMode, ImportReport, Snapshot, SnapshotScope} from "@/types";

/** The configuration as one document, which the page then saves as a file. */
export function exportSnapshot(scope: SnapshotScope) {
  return request<Snapshot>("/api/export-snapshot", "POST", scope);
}

/**
 * Writes a snapshot back. A dry run decides everything the real one would and
 * writes nothing, which is what the confirmation shows.
 */
export function importSnapshot(snapshot: Snapshot, scope: SnapshotScope, mode: ImportMode, dryRun: boolean) {
  return request<ImportReport>("/api/import-snapshot", "POST", {
    snapshot: snapshot,
    scope: scope,
    mode: mode,
    dryRun: dryRun,
  });
}

/** The schedule automatic backups are on, and the files themselves. */
export function getBackupState() {
  return request<BackupState>("/api/get-backup-state");
}

/** One stored snapshot in full, for downloading it. */
export function getBackup(name: string) {
  return request<Snapshot>(`/api/get-backup${query({name: name})}`);
}

export function createBackup() {
  return request<BackupState>("/api/create-backup", "POST");
}

export function restoreBackup(name: string, dryRun: boolean) {
  return request<ImportReport>("/api/restore-backup", "POST", {name: name, dryRun: dryRun});
}

export function deleteBackup(name: string) {
  return request<BackupState>("/api/delete-backup", "POST", {name: name});
}

export function updateBackupSchedule(mode: BackupMode, intervalHours: number, retention: number) {
  return request<BackupState>("/api/update-backup-schedule", "POST", {
    mode: mode,
    intervalHours: intervalHours,
    retention: retention,
  });
}
