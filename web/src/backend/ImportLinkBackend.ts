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
import type {ImportLink} from "@/types";

/** What a vendor's "add this to Gateway" link says, without storing any of it. */
export function parseImportLink(link: string) {
  return request<ImportLink>("/api/parse-import-link", "POST", {link: link});
}

/**
 * The link the URL scheme handler left, read once. It answers with nothing when
 * the page was opened by hand rather than by a click on a link.
 */
export function getPendingImportLink() {
  return request<ImportLink | null>("/api/get-pending-import-link");
}
