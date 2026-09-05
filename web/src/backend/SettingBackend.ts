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

import {request} from "@/backend/request";
import type {ProxyCheck, Setting} from "@/types";

export function getSetting() {
  return request<Setting>("/api/get-setting");
}

export function updateSetting(setting: Setting) {
  return request<Setting>("/api/update-setting", "POST", setting);
}

/** Tries the outbound proxy, the address as typed rather than as stored. */
export function testOutboundProxy(address: string) {
  return request<ProxyCheck>("/api/test-outbound-proxy", "POST", {address: address});
}
