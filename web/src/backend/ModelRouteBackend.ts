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
import type {ModelRoute, RoutePreviewStep} from "@/types";

/** Every routing rule, disabled ones included, in the order they are read in. */
export function getModelRoutes() {
  return request<ModelRoute[]>("/api/get-model-routes");
}

export function addModelRoute(route: ModelRoute) {
  return request<ModelRoute>("/api/add-model-route", "POST", route);
}

/** Writes an edited rule back. The name identifies the row and is not renamed. */
export function updateModelRoute(name: string, route: ModelRoute) {
  return request<ModelRoute>(`/api/update-model-route${query({name: name})}`, "POST", route);
}

export function deleteModelRoute(name: string) {
  return request<boolean>(`/api/delete-model-route${query({name: name})}`, "POST");
}

/** What a request naming this model would actually be sent, without sending it. */
export function previewModelRoute(model: string, agent = "") {
  return request<RoutePreviewStep[]>(`/api/preview-model-route${query({model: model, agent: agent})}`);
}
