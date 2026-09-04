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
import type {AutostartState, UpdateStatus, VersionInfo} from "@/types";

/** The token an agent has to send to the relay, and whether it is needed here. */
export function getRelayToken() {
  return request<{relayToken: string; localOnly: boolean}>("/api/get-relay-token");
}

/** Which build this Gateway is, and which one is published. */
export function getVersion(refresh = false) {
  return request<VersionInfo>(`/api/get-version${query({refresh: refresh ? 1 : undefined})}`);
}

/** Starts the download; the work carries on after this answers. */
export function updateGateway() {
  return request<UpdateStatus>("/api/update-gateway", "POST");
}

export function getUpdateStatus() {
  return request<UpdateStatus>("/api/get-update-status");
}

/** Whether this machine starts Gateway at login. */
export function getAutostart() {
  return request<AutostartState>("/api/get-autostart");
}

export function updateAutostart(enabled: boolean) {
  return request<AutostartState>("/api/update-autostart", "POST", {enabled: enabled});
}
