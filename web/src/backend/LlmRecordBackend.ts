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
import {ServerUrl} from "@/Setting";
import type {LlmAgentStat, LlmPrice, LlmRecord, LlmRecordStats, LlmRecordStatus, LlmTrendPoint} from "@/types";

export interface LlmRecordFilter {
  model?: string;
  provider?: string;
  agent?: string;
  clientIp?: string;
  outcome?: string;
  /** How far back the list and the totals reach. 0 is everything. */
  windowHours?: number;
  /** How many rows the per-model and per-provider breakdowns may return. */
  top?: number;
}

export function getLlmRecords(page = 1, pageSize = 25, filter: LlmRecordFilter = {}) {
  return request<LlmRecord[], number>(
    `/api/get-llm-records${query({p: page, pageSize: pageSize, ...filter})}`,
  );
}

/** The second payload is the rates the record was costed at, when it has any. */
export function getLlmRecord(id: number) {
  return request<LlmRecord, LlmPrice>(`/api/get-llm-record${query({id: id})}`);
}

export function getLlmRecordStats(filter: LlmRecordFilter = {}) {
  return request<LlmRecordStats>(`/api/get-llm-record-stats${query({...filter})}`);
}

/**
 * The same window cut into time buckets, with the empty ones filled in by the
 * server. `bucket` is "hour" or "day".
 */
export function getLlmUsageTrend(filter: LlmRecordFilter = {}, bucket: "hour" | "day" = "day") {
  return request<LlmTrendPoint[]>(`/api/get-llm-usage-trend${query({...filter, bucket: bucket})}`);
}

/** The same totals per agent, in one request rather than one request each. */
export function getLlmAgentStats(filter: LlmRecordFilter = {}) {
  return request<LlmAgentStat[]>(`/api/get-llm-agent-stats${query({...filter})}`);
}

/**
 * Opens the live feed of records; the browser reconnects on its own. Returns
 * the function that closes it.
 */
export function streamLlmRecords(handlers: {
  onRecord: (record: LlmRecord) => void;
  onOpen?: () => void;
  onError?: () => void;
}) {
  const source = new EventSource(`${ServerUrl}/api/stream-llm-records`, {withCredentials: true});

  source.addEventListener("record", event => {
    try {
      handlers.onRecord(JSON.parse((event as MessageEvent).data) as LlmRecord);
    } catch {
      // A malformed event is not worth tearing the feed down for.
    }
  });
  source.onopen = () => handlers.onOpen?.();
  source.onerror = () => handlers.onError?.();

  return () => source.close();
}

export function getLlmRecordStatus() {
  return request<LlmRecordStatus>("/api/get-llm-record-status");
}

export function deleteLlmRecord(id: number) {
  return request<boolean>(`/api/delete-llm-record${query({id: id})}`, "POST");
}

export function clearLlmRecords() {
  return request<boolean>("/api/clear-llm-records", "POST");
}
