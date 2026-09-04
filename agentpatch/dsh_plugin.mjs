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

// Audit-only dsh plugin. It listens on the harness's own session bus and
// reports what the agent did; it registers no service and answers no call, so
// nothing in a session waits on it and nothing it does can change a result.
//
// It deliberately does not mount on the `sessionTelemetry` seam: the harness
// accepts one backend there, `dsh-base` already mounts the OpenTelemetry one,
// and a second registration fails the whole plugin tree at boot.

const AGENT = __CASBIN_GATEWAY_AGENT__;
const RECORDS_URL = __CASBIN_GATEWAY_RECORDS_URL__;
const AGENT_PATH = __CASBIN_GATEWAY_AGENT_PATH__;
const OWNER = __CASBIN_GATEWAY_OWNER__;
const INGEST_TOKEN = __CASBIN_GATEWAY_INGEST_TOKEN__;
const INGEST_TOKEN_HEADER = __CASBIN_GATEWAY_INGEST_TOKEN_HEADER__;
const POST_TIMEOUT_MS = 3000;
const MAX_OBJECT_CHARS = 16 * 1024;
const MAX_TEXT_CHARS = 4 * 1024;

// The session events worth a record. The bus also carries the assembled
// assistant message, the raw stream chunks and the request header holding the
// whole system prompt and tool schemas; those are the conversation itself
// rather than what the agent did with it.
const EVENTS = {
  "user/message": ["prompt", "submitted", "attempted"],
  "tool/call": ["tool", "call", "attempted"],
  "tool/result": ["tool", "call", "success"],
  "turn/end": ["session", "stop", "success"],
  "compaction/start": ["compact", "before", "attempted"],
  "compaction/end": ["compact", "after", "success"],
};

const SENSITIVE_KEY = /secret|token|password|passwd|credential|private.?key|api.?key|access.?key|authorization|cookie/i;

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
    const value = JSON.stringify(sanitize(payload ?? {}));
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
      body: JSON.stringify({ agent: AGENT, agentPath: AGENT_PATH, user: OWNER, ...record }),
      signal: controller.signal,
    })
      .catch(() => {})
      .finally(() => clearTimeout(timer));
  } catch {
    // Monitoring is best effort and must never reach the session.
  }
}

function sessionKey(session) {
  try {
    return String(session?.id ?? "");
  } catch {
    return "";
  }
}

// messageText joins the text of a user message. Attachments and file references
// are left out: the record says what was asked, not what was pasted in.
function messageText(data) {
  try {
    const content = data?.content ?? data?.message?.content;
    if (typeof content === "string") return content.slice(0, MAX_TEXT_CHARS);
    if (!Array.isArray(content)) return "";
    return content
      .filter((part) => part && part.type === "text" && typeof part.text === "string")
      .map((part) => part.text)
      .join("\n")
      .slice(0, MAX_TEXT_CHARS);
  } catch {
    return "";
  }
}

function reportEvent(session, event) {
  const mapping = EVENTS[event?.type];
  if (!mapping) return;

  const [eventType, action, outcome] = mapping;
  const data = event.data ?? {};
  const record = {
    createdTime: new Date(event.time ?? Date.now()).toISOString(),
    eventType,
    action,
    outcome,
    sessionKey: sessionKey(session),
  };

  switch (event.type) {
  case "user/message":
    record.title = messageText(data);
    record.object = objectFor({ text: record.title });
    break;
  case "tool/call":
    record.toolName = data.name ?? "";
    record.toolUseId = data.callId ?? "";
    record.object = objectFor({ turn: data.turn, step: data.step, arguments: data.arguments });
    break;
  case "tool/result":
    // The result message is the tool's own output, which can be a whole file.
    record.outcome = data.error ? "failure" : "success";
    record.detail = data.error ? `${data.error.name}: ${data.error.code}` : "";
    record.object = objectFor({ turn: data.turn, step: data.step, error: data.error });
    break;
  case "turn/end":
    record.outcome = data.reason && data.reason !== "completed" ? "failure" : "success";
    record.detail = typeof data.reason === "string" ? data.reason : "";
    record.object = objectFor({ turn: data.turn, reason: data.reason });
    break;
  default:
    record.object = objectFor(data);
  }
  post(record);
}

export default {
  name: "casbin-gateway-agent-monitor",
  inject: ["sessions"],
  apply(ctx) {
    const guard = (handler) => (...args) => {
      try {
        handler(...args);
      } catch {
        // A shape Gateway cannot read is dropped rather than thrown back into
        // the harness, which is running a session on this call.
      }
    };

    ctx.on("session/created", guard((session) => post({
      createdTime: new Date().toISOString(),
      eventType: "session",
      action: "start",
      sessionKey: sessionKey(session),
    })));

    ctx.on("session/disposed", guard((session) => post({
      createdTime: new Date().toISOString(),
      eventType: "session",
      action: "end",
      sessionKey: sessionKey(session),
    })));

    ctx.on("session/event", guard(reportEvent));

    ctx.on("agent/error", guard((error) => post({
      createdTime: new Date().toISOString(),
      eventType: "session",
      action: "error",
      outcome: "failure",
      detail: String(error?.message ?? error ?? "").slice(0, MAX_TEXT_CHARS),
    })));
  },
};
