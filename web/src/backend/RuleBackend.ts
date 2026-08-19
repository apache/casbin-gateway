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

import {itemId, query, request} from "@/backend/request";
import type {Rule} from "@/types";

export function getRules(
  owner: string,
  page: string | number = "",
  pageSize: string | number = "",
  sortField = "",
  sortOrder = "",
) {
  return request<Rule[], number>(
    `/api/get-rules${query({
      owner: owner,
      p: page,
      pageSize: pageSize,
      sortField: sortField,
      sortOrder: sortOrder,
    })}`,
  );
}

export function getRule(owner: string, name: string) {
  return request<Rule>(`/api/get-rule${query({id: itemId(owner, name)})}`);
}

export function addRule(rule: Rule) {
  return request("/api/add-rule", "POST", rule);
}

export function updateRule(owner: string, name: string, rule: Rule) {
  return request(`/api/update-rule${query({id: itemId(owner, name)})}`, "POST", rule);
}

export function deleteRule(rule: Rule) {
  return request("/api/delete-rule", "POST", rule);
}
