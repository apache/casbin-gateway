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
import type {CcSwitchImport, CcSwitchResult, CcSwitchSelection} from "@/types";

/** What CC Switch holds on this machine, with every key masked. */
export function getCcSwitchImport(account = "") {
  return request<CcSwitchImport>(`/api/get-ccswitch-import?account=${encodeURIComponent(account)}`);
}

/**
 * Brings over the entries that were ticked. Only their keys are sent: the values
 * are read from CC Switch's own store on the server, so no credential travels
 * back through the browser to get here.
 */
export function importCcSwitch(selection: CcSwitchSelection) {
  return request<CcSwitchResult>("/api/import-ccswitch", "POST", selection);
}
