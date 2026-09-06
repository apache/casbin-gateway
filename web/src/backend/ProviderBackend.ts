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
import type {
  Provider,
  ProviderHealth,
  ProviderProbe,
  ProviderProbeMode,
  ProviderQuota,
  ProviderSignin,
  ProviderTestResult,
} from "@/types";

export function getProviders(
  owner: string,
  page: string | number = "",
  pageSize: string | number = "",
  sortField = "",
  sortOrder = "",
) {
  return request<Provider[], number>(
    `/api/get-providers${query({
      owner: owner,
      p: page,
      pageSize: pageSize,
      sortField: sortField,
      sortOrder: sortOrder,
    })}`,
  );
}

export function getProvider(owner: string, name: string) {
  return request<Provider>(`/api/get-provider${query({id: itemId(owner, name)})}`);
}

export function addProvider(provider: Provider) {
  return request("/api/add-provider", "POST", provider);
}

/** What a vendor's "add this provider" link says, without storing any of it. */
export function parseProviderLink(link: string) {
  return request<Provider>("/api/parse-provider-link", "POST", {link: link});
}

export function updateProvider(owner: string, name: string, provider: Provider) {
  return request(`/api/update-provider${query({id: itemId(owner, name)})}`, "POST", provider);
}

export function deleteProvider(provider: Provider) {
  return request("/api/delete-provider", "POST", provider);
}

/** The models the provider's upstream reports. The whole provider is posted, not
 * an id: the new-provider form has nothing saved to look up yet. */
export function getProviderModels(provider: Provider) {
  return request<string[]>("/api/get-provider-models", "POST", provider);
}

/** Probes the provider's upstream. The whole provider is posted rather than an
 * id, so a form can be checked before it is saved. */
export function testProvider(provider: Provider) {
  return request<ProviderTestResult>("/api/test-provider", "POST", provider);
}

/** Starts a browser sign-in for a vendor whose subscription Gateway can hold.
 *  The token stays on the server; what comes back is the id of the sign-in,
 *  which the save of the provider redeems. */
export function signInProvider(vendor: string) {
  return request<ProviderSignin>(`/api/sign-in-provider${query({vendor: vendor})}`, "POST");
}

/** How a sign-in that was started is getting on. Approving one takes as long as
 *  whoever is at the browser, so the page polls. */
export function getProviderSignin(id: string) {
  return request<ProviderSignin>(`/api/get-provider-signin${query({id: id})}`);
}

/** What the proxy has seen of each provider, which is what says why a request
 * went to a fallback rather than to the bound provider. */
export function getProviderHealth() {
  return request<ProviderHealth[]>("/api/get-provider-health");
}

/** The vendor balances already known. Asks no vendor anything. */
export function getProviderQuotas() {
  return request<ProviderQuota[]>("/api/get-provider-quotas");
}

/** Asks the vendors what is left. Without an id every provider the caller can
 * see is refreshed, and without force only the ones with a stale answer. */
export function refreshProviderQuotas(id = "", force = false) {
  return request<ProviderQuota[]>("/api/refresh-provider-quotas", "POST", {id: id, force: force});
}

/**
 * The newest probe of every provider that has one. The second payload is how
 * probes are started, so an unprobed provider can say whether it is waiting for
 * a sweep or for a button.
 */
export function getProviderProbes() {
  return request<ProviderProbe[], ProviderProbeMode>("/api/get-provider-probes");
}

/** Every kept run for one provider, newest first. */
export function getProviderProbeHistory(id: string) {
  return request<ProviderProbe[]>(`/api/get-provider-probe-history${query({id: id})}`);
}

/**
 * Runs the probe suite now. It sends four short requests to the upstream and so
 * spends a little of that provider's credit, which is why nothing calls this on
 * a page load.
 */
export function probeProvider(id: string) {
  return request<ProviderProbe>(`/api/probe-provider${query({id: id})}`, "POST");
}
