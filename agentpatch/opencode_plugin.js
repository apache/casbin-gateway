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

// Audit-only opencode plugin. Every hook observes and returns; none of them
// reads or writes the output object opencode passes in, so no tool call,
// permission or message is changed by monitoring being on.

const AGENT = __CASBIN_GATEWAY_AGENT__;
const RECORDS_URL = __CASBIN_GATEWAY_RECORDS_URL__;
const AGENT_PATH = __CASBIN_GATEWAY_AGENT_PATH__;
const OWNER = __CASBIN_GATEWAY_OWNER__;
const INGEST_TOKEN = __CASBIN_GATEWAY_INGEST_TOKEN__;
const INGEST_TOKEN_HEADER = __CASBIN_GATEWAY_INGEST_TOKEN_HEADER__;
const POST_TIMEOUT_MS = 3000;
const MAX_OBJECT_CHARS = 16 * 1024;
const MAX_TEXT_CHARS = 4 * 1024;

const SENSITIVE_KEY = /secret|token|password|passwd|credential|private.?key|api.?key|access.?key|authorization|cookie/i;

// The events worth a record, by the type opencode publishes them under.
const EVENTS = {
  "session.created": ["session", "start", ""],
  "session.deleted": ["session", "end", ""],
  "session.idle": ["session", "stop", "success"],
  "session.error": ["session", "stop", "failure"],
  "session.compacted": ["compact", "after", "success"],
  "permission.replied": ["permission", "replied", ""],
  "command.executed": ["command", "executed", "success"],
  "file.edited": ["file", "edited", "success"],
};

function sanitize(value, key = "", depth = 0) {
  if (SENSITIVE_KEY.test(key)) return "[REDACTED]";
  if (depth > 6) return undefined;
  if (Array.isArray(value)) return value.slice(0, 50).map((item) => sanitize(item, "", depth + 1));
  if (value && typeof value === "object") {
    const result = {};
    for (const [childKey, childValue] of Object.entries(value)) {
      result[childKey] = sanitize(childValue, childKey, depth + 1);
    }
    return result;
  }
  if (typeof value === "string" && value.length > MAX_TEXT_CHARS) {
    return `${value.slice(0, MAX_TEXT_CHARS)}...[truncated]`;
  }
  return value;
}

function objectFor(payload) {
  try {
    const value = JSON.stringify(sanitize(payload || {}));
    if (!value) return "";
    return value.length > MAX_OBJECT_CHARS ? `${value.slice(0, MAX_OBJECT_CHARS)}...[truncated]` : value;
  } catch {
    return "";
  }
}

function post(record) {
  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), POST_TIMEOUT_MS);
    const headers = { "Content-Type": "application/json" };
    if (INGEST_TOKEN) headers[INGEST_TOKEN_HEADER] = INGEST_TOKEN;
    fetch(RECORDS_URL, {
      method: "POST",
      headers,
      body: JSON.stringify({
        agent: AGENT,
        agentPath: AGENT_PATH,
        user: OWNER,
        createdTime: new Date().toISOString(),
        ...record,
      }),
      signal: controller.signal,
    })
      .catch(() => {})
      .finally(() => clearTimeout(timer));
  } catch {
    // Monitoring is best effort and must never reach the session.
  }
}

// promptText joins the text parts of a message. Attachments and file contents
// are left out: the record says what was asked, not what was pasted in.
function promptText(parts) {
  try {
    return (parts || [])
      .filter((part) => part && part.type === "text" && typeof part.text === "string")
      .map((part) => part.text)
      .join("\n")
      .slice(0, MAX_TEXT_CHARS);
  } catch {
    return "";
  }
}

export const CasbinGatewayMonitor = async () => {
  return {
    event: async ({ event }) => {
      const mapping = EVENTS[event?.type];
      if (!mapping) return;
      const [eventType, action, outcome] = mapping;
      const properties = event.properties || {};
      post({
        eventType,
        action,
        outcome,
        sessionKey: properties.sessionID || properties.info?.id || "",
        object: objectFor(properties),
      });
    },

    "chat.message": async (input, output) => {
      post({
        eventType: "prompt",
        action: "submitted",
        outcome: "attempted",
        sessionKey: input?.sessionID || "",
        promptId: input?.messageID || "",
        model: input?.model ? `${input.model.providerID}/${input.model.modelID}` : "",
        title: promptText(output?.parts).slice(0, 512),
        object: objectFor({
          agent: input?.agent,
          variant: input?.variant,
          text: promptText(output?.parts),
        }),
      });
    },

    "tool.execute.before": async (input, output) => {
      post({
        eventType: "tool",
        action: "call",
        outcome: "attempted",
        sessionKey: input?.sessionID || "",
        toolUseId: input?.callID || "",
        toolName: input?.tool || "",
        object: objectFor({ args: output?.args }),
      });
    },

    "tool.execute.after": async (input, output) => {
      post({
        eventType: "tool",
        action: "call",
        outcome: "success",
        sessionKey: input?.sessionID || "",
        toolUseId: input?.callID || "",
        toolName: input?.tool || "",
        title: typeof output?.title === "string" ? output.title.slice(0, 512) : "",
        // The output itself is not recorded: its length is enough to see that a
        // call returned, and a tool result can be megabytes of file content.
        object: objectFor({ outputLength: (output?.output || "").length }),
      });
    },

    "permission.ask": async (input) => {
      post({
        eventType: "permission",
        action: "requested",
        outcome: "attempted",
        sessionKey: input?.sessionID || "",
        toolUseId: input?.callID || "",
        toolName: input?.type || "",
        title: typeof input?.title === "string" ? input.title.slice(0, 512) : "",
        object: objectFor({ pattern: input?.pattern, metadata: input?.metadata }),
      });
    },
  };
};
