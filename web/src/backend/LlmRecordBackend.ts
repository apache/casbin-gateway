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
import type {LlmRecord, LlmRecordStatus, LlmUsage} from "@/types";

export interface LlmRecordFilter {
  model?: string;
  channel?: string;
  agent?: string;
  clientIp?: string;
  outcome?: string;
}

export function getLlmRecords(page = 1, pageSize = 25, filter: LlmRecordFilter = {}) {
  return request<LlmRecord[], number>(
    `/api/get-llm-records${query({p: page, pageSize: pageSize, ...filter})}`,
  );
}

export function getLlmRecord(id: number) {
  return request<LlmRecord>(`/api/get-llm-record${query({id: id})}`);
}

export function getLlmRecordStatus() {
  return request<LlmRecordStatus>("/api/get-llm-record-status");
}

export function getLlmUsage(rangeType: string, count: number, granularity: string) {
  return request<LlmUsage>(
    `/api/get-llm-usage${query({rangeType: rangeType, count: count, granularity: granularity})}`,
  );
}

export function deleteLlmRecord(id: number) {
  return request<boolean>(`/api/delete-llm-record${query({id: id})}`, "POST");
}

export function clearLlmRecords() {
  return request<boolean>("/api/clear-llm-records", "POST");
}
