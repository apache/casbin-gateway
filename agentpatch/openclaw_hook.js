// Copyright 2023 The casbin Authors. All Rights Reserved.
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

const RECORDS_URL = __CASBIN_GATEWAY_RECORDS_URL__;
const AGENT_PATH = __CASBIN_GATEWAY_AGENT_PATH__;
const OWNER = __CASBIN_GATEWAY_OWNER__;
const POST_TIMEOUT_MS = 3000;
const MAX_OBJECT_CHARS = 64 * 1024;

const SENSITIVE_KEY = /secret|token|password|passwd|credential|private.?key|api.?key|access.?key|authorization|cookie/i;
const OMIT_CONTEXT_KEYS = new Set(["cfg", "bootstrapFiles", "sessionEntry", "previousSessionEntry"]);

function sanitize(value, key = "") {
  if (SENSITIVE_KEY.test(key)) return "[REDACTED]";
  if (Array.isArray(value)) return value.map((item) => sanitize(item));
  if (value && typeof value === "object") {
    const result = {};
    for (const [childKey, childValue] of Object.entries(value)) {
      if (!OMIT_CONTEXT_KEYS.has(childKey)) result[childKey] = sanitize(childValue, childKey);
    }
    return result;
  }
  return value;
}

function objectFor(context) {
  try {
    const value = JSON.stringify(sanitize(context || {}));
    return value.length > MAX_OBJECT_CHARS ? `${value.slice(0, MAX_OBJECT_CHARS)}...[truncated]` : value;
  } catch {
    return "";
  }
}

function post(record) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), POST_TIMEOUT_MS);
  fetch(RECORDS_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(record),
    signal: controller.signal,
  }).catch(() => {}).finally(() => clearTimeout(timer));
}

export default function handler(event) {
  const context = event.context || {};
  const toolName = context.tool || context.toolName || context.tool_name || context.command || context.commandName || "";
  post({
    agent: "openclaw",
    agentPath: AGENT_PATH,
    createdTime: (event.timestamp instanceof Date ? event.timestamp : new Date()).toISOString(),
    eventType: toolName ? "tool" : (event.type || "event"),
    action: toolName ? "call" : (event.action || "observed"),
    outcome: event.outcome || "attempted",
    sessionKey: event.sessionKey || context.sessionKey || "",
    user: context.senderId || context.from || context.to || OWNER,
    toolName,
    object: objectFor(context),
  });
}
