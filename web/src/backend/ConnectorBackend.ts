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
import type {AgentConfigPlanItem, ConnectRequest, ConnectorCatalog} from "@/types";

/** The catalog, what is connected here, and the agents it can be written into. */
export function getConnectors(owner: string) {
  return request<ConnectorCatalog>(`/api/get-connectors${query({owner: owner})}`);
}

/** One connection with its secrets masked, which is what fills the dialog. */
export function getConnection(owner: string, name: string) {
  return request<{credentials: Record<string, string>; agents: string[]} | null>(
    `/api/get-connection${query({owner: owner, name: name})}`,
  );
}

/**
 * Stores the credentials and writes the server into the agents named. Sending
 * it again is how a connection is edited: the agents it names become the agents
 * it is installed in, and the ones left out have it taken away.
 */
export function connect(body: ConnectRequest) {
  return request<AgentConfigPlanItem[]>("/api/connect", "POST", body);
}

/** Removes the server from every agent and forgets the credentials. */
export function disconnect(owner: string, name: string) {
  return request<AgentConfigPlanItem[]>("/api/disconnect", "POST", {owner: owner, name: name});
}
