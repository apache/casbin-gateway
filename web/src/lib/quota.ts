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

import type {QuotaConfig} from "@/types";

const currencySymbols: Record<string, string> = {USD: "$", CNY: "¥", EUR: "€", GBP: "£", JPY: "¥"};

/** An amount as the vendor bills it. A currency with a symbol gets it; anything
 * else keeps its own name, because a made-up symbol would be a guess. */
export function formatQuota(amount: number, unit: string) {
  const rounded = Math.abs(amount) < 1 && amount !== 0 ? amount.toFixed(4) : amount.toFixed(2);
  const symbol = currencySymbols[unit.toUpperCase()];
  if (symbol) {
    return amount < 0 ? `-${symbol}${rounded.slice(1)}` : `${symbol}${rounded}`;
  }
  return unit === "" ? rounded : `${rounded} ${unit}`;
}

export function emptyQuotaConfig(): QuotaConfig {
  return {
    url: "",
    headers: {},
    token: "",
    remaining: "",
    used: "",
    total: "",
    unit: "USD",
    scale: 0,
    manual: false,
    initial: 0,
    since: "",
  };
}

/** A ready-made endpoint for a family of sites, so the paths do not have to be
 * worked out from the vendor's API documentation. */
export interface QuotaPreset {
  key: string;
  label: string;
  config: QuotaConfig;
}

export const quotaPresets: QuotaPreset[] = [
  {
    key: "newapi",
    label: "New API",
    config: {
      ...emptyQuotaConfig(),
      url: "/api/user/self",
      headers: {Authorization: "Bearer {{token}}", "New-Api-User": ""},
      remaining: "data.quota",
      used: "data.used_quota",
      // New API counts in units of 1/500000 USD.
      scale: 500000,
    },
  },
  {
    key: "oneapi",
    label: "One API",
    config: {
      ...emptyQuotaConfig(),
      url: "/api/user/self",
      headers: {Authorization: "Bearer {{token}}"},
      remaining: "data.quota",
      used: "data.used_quota",
      scale: 500000,
    },
  },
];
