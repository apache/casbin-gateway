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

import {ServerUrl} from "@/Setting";
import type {ApiResponse} from "@/types";

/**
 * One place where every call to the Go API is made. `credentials: include`
 * carries the beego session cookie, which is what the backend authenticates
 * with; nothing else about a request is special.
 */
export async function request<T = any, T2 = any>(
  path: string,
  method: "GET" | "POST" = "GET",
  body?: unknown,
): Promise<ApiResponse<T, T2>> {
  const options: RequestInit = {
    method: method,
    credentials: "include",
  };

  if (body !== undefined && body !== null) {
    options.headers = {"Content-Type": "application/json"};
    options.body = JSON.stringify(body);
  }

  const response = await fetch(`${ServerUrl}${path}`, options);
  return (await response.json()) as ApiResponse<T, T2>;
}

/** Builds a query string, dropping the parameters that were left empty. */
export function query(params: Record<string, string | number | undefined | null>) {
  const search = new URLSearchParams();
  Object.keys(params).forEach(key => {
    const value = params[key];
    if (value !== undefined && value !== null && value !== "") {
      search.set(key, String(value));
    }
  });
  const text = search.toString();
  return text === "" ? "" : `?${text}`;
}

/** The `owner/name` pair the API takes as a single `id` parameter. */
export function itemId(owner: string, name: string) {
  return `${owner}/${name}`;
}
