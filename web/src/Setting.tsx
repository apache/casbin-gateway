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

import i18next from "i18next";
import Sdk from "casdoor-js-sdk";
import {toast} from "sonner";

import * as Conf from "@/Conf";
import type {Account, ApiResponse} from "@/types";

/**
 * Where the REST API lives. It stays empty on purpose: the dev server proxies
 * /api and /v1 to the Go backend and production serves this bundle from that
 * same backend, so every request is same-origin and the session cookie needs no
 * cross-site handling. VITE_SERVER_URL overrides it for a split deployment.
 */
export const ServerUrl: string = import.meta.env.VITE_SERVER_URL ?? "";

export const StaticBaseUrl = "https://cdn.casbin.org";

export let CasdoorSdk: Sdk;

export const Countries = [
  {label: "English", key: "en", country: "US", alt: "English"},
  {label: "中文", key: "zh", country: "CN", alt: "中文"},
];

export function initCasdoorSdk(config: typeof Conf.AuthConfig) {
  CasdoorSdk = new Sdk(config);
}

// Casdoor is optional: with no "casdoorEndpoint" in app.conf the backend reports
// an empty server URL and the app signs in against its own user table instead.
export function isCasdoorAvailable() {
  return Conf.AuthConfig.serverUrl !== "";
}

// A session created by the built-in username/password login, as opposed to one
// issued by Casdoor. Such a user has no Casdoor profile page to link to.
export function isBasicLoginMode(account: Account | null | undefined) {
  return account?.owner === "basic";
}

function getUrlWithLanguage(url: string) {
  if (url.includes("?")) {
    return `${url}&language=${getLanguage()}`;
  }
  return `${url}?language=${getLanguage()}`;
}

export function getSignupUrl() {
  if (!isCasdoorAvailable()) {
    return "";
  }
  return getUrlWithLanguage(CasdoorSdk.getSignupUrl());
}

export function getSigninUrl() {
  if (!isCasdoorAvailable()) {
    return "";
  }
  return getUrlWithLanguage(CasdoorSdk.getSigninUrl());
}

export function getMyProfileUrl(account: Account | null | undefined) {
  if (!isCasdoorAvailable() || isBasicLoginMode(account)) {
    return "";
  }
  return getUrlWithLanguage(CasdoorSdk.getMyProfileUrl(account as any));
}

export function signin(): Promise<ApiResponse> {
  // The SDK types this as Response, but it resolves the parsed JSON body.
  return CasdoorSdk.signin(ServerUrl) as unknown as Promise<ApiResponse>;
}

export function myParseInt(value: unknown) {
  const result = parseInt(String(value), 10);
  return isNaN(result) ? 0 : result;
}

export function openLink(link: string) {
  const w = window.open("about:blank");
  if (w) {
    w.location.href = link;
  }
}

export function goToLink(link: string) {
  window.location.href = link;
}

export function showMessage(type: "" | "success" | "error" | "info", text: string) {
  if (type === "") {
    return;
  } else if (type === "success") {
    toast.success(text);
  } else if (type === "error") {
    toast.error(text);
  } else {
    toast(text);
  }
}

export function isAdminUser(account: Account | null | undefined) {
  return account?.isAdmin === true;
}

export function deepCopy<T>(obj: T): T {
  return Object.assign({}, obj);
}

export function addRow<T>(array: T[] | undefined, row: T): T[] {
  return [...(array ?? []), row];
}

export function prependRow<T>(array: T[] | undefined, row: T): T[] {
  return [row, ...(array ?? [])];
}

export function deleteRow<T>(array: T[], i: number): T[] {
  return [...array.slice(0, i), ...array.slice(i + 1)];
}

export function swapRow<T>(array: T[], i: number, j: number): T[] {
  return [...array.slice(0, i), array[j], ...array.slice(i + 1, j), array[i], ...array.slice(j + 1)];
}

export function isMobile() {
  return typeof window !== "undefined" && window.innerWidth < 768;
}

export function getFormattedDate(date: string | undefined | null) {
  if (date === undefined || date === null) {
    return null;
  }

  return date.replace("T", " ").replace("+08:00", " ");
}

export function getShortName(s: string | undefined) {
  if (!s) {
    return "";
  }
  return s.split("/").slice(-1)[0];
}

export function getShortText(s: string | undefined, maxLength = 35) {
  if (!s) {
    return "";
  }
  if (s.length > maxLength) {
    return `${s.slice(0, maxLength)}...`;
  }
  return s;
}

export function getRandomName() {
  return Math.random().toString(36).slice(-6);
}

function getRandomInt(s: string) {
  let hash = 0;
  for (let i = 0; i < s.length; i++) {
    const char = s.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash;
  }
  return hash;
}

export function getAvatarColor(s: string) {
  const colorList = ["#f56a00", "#7265e6", "#ffbf00", "#00a2ae"];
  let random = getRandomInt(s);
  if (random < 0) {
    random = -random;
  }
  return colorList[random % 4];
}

export function getLanguage() {
  return i18next.language;
}

export function setLanguage(language: string) {
  localStorage.setItem("language", language);
  i18next.changeLanguage(language);
}

export function changeLanguage(language: string) {
  setLanguage(language);
  window.location.reload();
}

export function isResponseDenied(data: {msg?: string}) {
  return data.msg === "Unauthorized operation" || data.msg === "未授权的操作";
}

function getOriginalName(name: string) {
  const tokens = name.split("_");
  return tokens.length > 0 ? tokens[0] : name;
}

export function getRepoUrl(name: string) {
  const original = getOriginalName(name);
  if (original === "casdoor") {
    return "https://github.com/casdoor/casdoor";
  }
  return `https://github.com/casbin/${original}`;
}

export function getVersionInfo(text: string | undefined, siteName: string) {
  if (!text) {
    return null;
  }

  let versionInfo: {version?: string; commitOffset?: number};
  try {
    versionInfo = JSON.parse(text);
  } catch {
    return null;
  }

  const link = versionInfo?.version ? `${getRepoUrl(siteName)}/releases/tag/${versionInfo.version}` : "";
  let versionText = versionInfo?.version ? versionInfo.version : "Unknown version";
  if ((versionInfo?.commitOffset ?? 0) > 0) {
    versionText += ` (ahead+${versionInfo.commitOffset})`;
  }

  return {text: versionText, link: link};
}

export function getDeduplicatedArray<T extends Record<string, any>>(
  sourceTable: T[] | undefined,
  filterTable: T[] | undefined,
  key: string,
): T[] {
  if (!sourceTable) {
    return [];
  }
  return sourceTable.filter(item => !(filterTable ?? []).some(arrayItem => arrayItem[key] === item[key]));
}

export function getItemId(item: {owner: string; name: string}) {
  return `${item.owner}/${item.name}`;
}
