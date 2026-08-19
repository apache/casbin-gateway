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
import type {Node} from "@/types";

export function getGlobalNodes() {
  return request<Node[]>("/api/get-global-nodes");
}

export function getNodes(
  owner: string,
  page = "",
  pageSize = "",
  field = "",
  value = "",
  sortField = "",
  sortOrder = "",
) {
  return request<Node[], number>(
    `/api/get-nodes${query({
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

export function getNode(owner: string, name: string) {
  return request<Node>(`/api/get-node${query({id: itemId(owner, name)})}`);
}

export function updateNode(owner: string, name: string, node: Node) {
  return request(`/api/update-node${query({id: itemId(owner, name)})}`, "POST", node);
}

export function addNode(node: Node) {
  return request("/api/add-node", "POST", node);
}

export function deleteNode(node: Node) {
  return request("/api/delete-node", "POST", node);
}
