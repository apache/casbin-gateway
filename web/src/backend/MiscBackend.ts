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
import type {Application, GatewayStatus, MetricPoint, Provider} from "@/types";

export function getProviders() {
  return request<Provider[]>("/api/get-providers");
}

export function getApplications(owner: string) {
  return request<Application[]>(`/api/get-applications${query({owner: owner})}`);
}

export function getGatewayStatus() {
  return request<GatewayStatus>("/api/get-gateway-status");
}

export function getMetric(type: string, rangeType: string, count: number, top?: number) {
  return request<MetricPoint[]>(
    `/api/get-metrics${query({type: type, rangeType: rangeType, count: count, top: top})}`,
  );
}

export function getMetricOverTime(rangeType: string, count: number, granularity: string) {
  return request<MetricPoint[], number>(
    `/api/get-metrics-over-time${query({
      rangeType: rangeType,
      count: count,
      granularity: granularity,
    })}`,
  );
}
