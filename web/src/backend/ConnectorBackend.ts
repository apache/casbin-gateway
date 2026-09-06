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
import type {AgentConfigPlanItem, ConnectRequest, ConnectorCatalog, ConnectorProbeResult} from "@/types";

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

/** The callback address the operator registers their application with. */
export function getConnectorRedirectUri() {
  return request<string>("/api/get-connector-redirect-uri");
}

/**
 * Stores the client application and answers with the vendor address to send the
 * operator to. Nothing is authorized until they come back through the callback.
 */
export function startConnectorAuth(owner: string, name: string, credentials: Record<string, string>) {
  return request<string>("/api/start-connector-auth", "POST", {
    owner: owner,
    name: name,
    credentials: credentials,
  });
}

/**
 * Starts this connection's server and asks what it offers. Slow the first time
 * a server is installed from npm, so callers should show that it is working.
 */
export function testConnection(owner: string, name: string) {
  return request<ConnectorProbeResult>("/api/test-connection", "POST", {owner: owner, name: name});
}

/**
 * Tests every connection again. It answers as soon as the work has started, so
 * the caller reloads the listing a little later to see what each one found.
 */
export function retestConnections(owner: string) {
  return request<number>("/api/retest-connections", "POST", {owner: owner});
}
