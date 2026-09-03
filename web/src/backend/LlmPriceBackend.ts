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
import type {
  LlmPriceEntry,
  LlmPriceView,
  ModelsDevModel,
  ModelsDevSync,
  ModelsDevSyncMode,
  ModelsDevSyncState,
} from "@/types";

/** Every price in effect, built-in ones included, each with the layer it came from. */
export function getLlmPrices() {
  return request<LlmPriceView[]>("/api/get-llm-prices");
}

/** Stores one price, which from then on overrides the built-in table for that model. */
export function updateLlmPrice(entry: Partial<LlmPriceEntry>) {
  return request<LlmPriceEntry>("/api/update-llm-price", "POST", entry);
}

/** Drops a stored price, putting the built-in one back in effect where there is one. */
export function deleteLlmPrice(model: string) {
  return request<boolean>(`/api/delete-llm-price${query({model: model})}`, "POST");
}

/**
 * One page of the models.dev catalogue. The whole thing is a few megabytes, so
 * the search happens on the server and the browser only ever sees the matches.
 */
export function searchModelsDevModels(q: string, refresh = false) {
  return request<ModelsDevModel[]>(
    `/api/search-models-dev-models${query({q: q, refresh: refresh ? "true" : ""})}`,
  );
}

/** Prices every model this machine has been seen running, from models.dev. */
export function syncModelsDevPrices(refresh = true) {
  return request<ModelsDevSync>(
    `/api/sync-models-dev-prices${query({refresh: refresh ? "true" : ""})}`,
    "POST",
  );
}

/** The schedule an automatic sync is on, and what the last run did. */
export function getModelsDevSync() {
  return request<ModelsDevSyncState>("/api/get-models-dev-sync");
}

/** Turns the automatic sync on or off and sets how often it runs. */
export function updateModelsDevSync(mode: ModelsDevSyncMode, intervalHours: number) {
  return request<ModelsDevSyncState>("/api/update-models-dev-sync", "POST", {
    mode: mode,
    intervalHours: intervalHours,
  });
}
