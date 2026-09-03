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
import type {ProbeCase} from "@/types";

/** The whole suite a probe runs, disabled cases included. */
export function getProbeCases() {
  return request<ProbeCase[]>("/api/get-probe-cases");
}

export function addProbeCase(probeCase: ProbeCase) {
  return request<ProbeCase>("/api/add-probe-case", "POST", probeCase);
}

/** Writes an edited case back. The name identifies the row and is not renamed. */
export function updateProbeCase(name: string, probeCase: ProbeCase) {
  return request<ProbeCase>(`/api/update-probe-case${query({name: name})}`, "POST", probeCase);
}

export function deleteProbeCase(name: string) {
  return request<boolean>(`/api/delete-probe-case${query({name: name})}`, "POST");
}

/** Puts the shipped suite back, leaving cases someone wrote where they are. */
export function resetProbeCases() {
  return request<ProbeCase[]>("/api/reset-probe-cases", "POST");
}
