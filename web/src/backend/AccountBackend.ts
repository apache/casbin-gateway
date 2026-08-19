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
import type {Account, SigninOptions} from "@/types";

export function getSigninOptions() {
  return request<SigninOptions>("/api/get-signin-options");
}

export function getAccount() {
  // fromPath lets the backend skip its auto-login of the default admin while
  // the sign-in page itself is what asked.
  return request<Account | null, string>(
    `/api/get-account${query({fromPath: window.location.pathname})}`,
  );
}

export function signin(code: string, state: string) {
  return request(`/api/signin${query({code: code, state: state})}`, "POST");
}

export function signinWithPassword(username: string, password: string) {
  return request("/api/signin", "POST", {username: username, password: password});
}

export function updateAccount(account: {
  displayName?: string;
  avatar?: string;
  currentPassword?: string;
  newPassword?: string;
}) {
  return request("/api/update-account", "POST", account);
}

export function signout() {
  return request("/api/signout", "POST");
}
