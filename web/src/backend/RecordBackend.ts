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
import type {Record as GatewayRecord} from "@/types";

export function getRecords(
  owner: string,
  page: string | number = "",
  pageSize: string | number = "",
  sortField = "",
  sortOrder = "",
) {
  return request<GatewayRecord[], number>(
    `/api/get-records${query({
      owner: owner,
      p: page,
      pageSize: pageSize,
      sortField: sortField,
      sortOrder: sortOrder,
    })}`,
  );
}

export function getRecord(owner: string, id: string | number) {
  return request<GatewayRecord>(`/api/get-record${query({owner: owner, id: id})}`);
}

export function updateRecord(owner: string, id: string | number, record: GatewayRecord) {
  return request(`/api/update-record${query({owner: owner, id: id})}`, "POST", record);
}

export function addRecord(record: Partial<GatewayRecord>) {
  return request<GatewayRecord>("/api/add-record", "POST", record);
}

export function deleteRecord(record: GatewayRecord) {
  return request("/api/delete-record", "POST", record);
}
