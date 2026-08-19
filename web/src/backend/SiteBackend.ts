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
import type {Site} from "@/types";

export function getGlobalSites() {
  return request<Site[]>("/api/get-global-sites");
}

export function getSites(
  owner: string,
  page = "",
  pageSize = "",
  field = "",
  value = "",
  sortField = "",
  sortOrder = "",
) {
  return request<Site[], number>(
    `/api/get-sites${query({
      owner: owner,
      p: page,
      pageSize: pageSize,
      field: field,
      value: value,
      sortField: sortField,
      sortOrder: sortOrder,
    })}`,
  );
}

export function getSite(owner: string, name: string) {
  return request<Site>(`/api/get-site${query({id: itemId(owner, name)})}`);
}

export function updateSite(owner: string, name: string, site: Site) {
  return request(`/api/update-site${query({id: itemId(owner, name)})}`, "POST", site);
}

export function addSite(site: Site) {
  return request("/api/add-site", "POST", site);
}

export function deleteSite(site: Site) {
  return request("/api/delete-site", "POST", site);
}
