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

import {request} from "@/backend/request";
import type {CloudSyncDirection, CloudSyncFile, CloudSyncMode, CloudSyncState} from "@/types";

/** Where the backups are copied to, what else they could be copied to, and how
 *  the last run went. */
export function getCloudSyncState() {
  return request<CloudSyncState>("/api/get-cloud-sync-state");
}

export function updateCloudSync(mode: CloudSyncMode, kind: string, options: Record<string, string>) {
  return request<CloudSyncState>("/api/update-cloud-sync", "POST", {mode: mode, kind: kind, options: options});
}

/** Reaches the target the form is describing, without storing it. */
export function testCloudSync(kind: string, options: Record<string, string>) {
  return request<{target: string; files: CloudSyncFile[]}>("/api/test-cloud-sync", "POST", {
    kind: kind,
    options: options,
  });
}

export function runCloudSync(direction: CloudSyncDirection) {
  return request<CloudSyncState>("/api/run-cloud-sync", "POST", {direction: direction});
}
