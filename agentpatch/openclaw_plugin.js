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

// Gateway's OpenClaw plugin. It registers one hook and nothing else:
// before_tool_call, which is the only seam OpenClaw waits on before running a
// tool. The audit hook beside it, under ~/.openclaw/hooks, still does the
// reporting - the event stream it listens on is not something a session waits
// for, so it can never refuse anything.

const AGENT = __CASBIN_GATEWAY_AGENT__;
const DECISION_URL = __CASBIN_GATEWAY_DECISION_URL__;
const INGEST_TOKEN = __CASBIN_GATEWAY_INGEST_TOKEN__;
const INGEST_TOKEN_HEADER = __CASBIN_GATEWAY_INGEST_TOKEN_HEADER__;
// The tool call waits on the verdict, so a Gateway that is slow to answer is
// treated as one that is not running.
const DECIDE_TIMEOUT_MS = 2000;

// decide asks Gateway whether a tool call may go ahead, answering with the
// reason it may not. Everything that can go wrong answers "": a plugin that
// cannot get a verdict must not stop OpenClaw working.
async function decide(toolName, sessionKey, toolCallId) {
  if (!DECISION_URL || !toolName) return "";
  try {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), DECIDE_TIMEOUT_MS);
    const headers = { "Content-Type": "application/json" };
    if (INGEST_TOKEN) headers[INGEST_TOKEN_HEADER] = INGEST_TOKEN;
    const response = await fetch(DECISION_URL, {
      method: "POST",
      headers,
      body: JSON.stringify({
        agent: AGENT,
        tool: toolName,
        sessionKey: sessionKey || "",
        toolUseId: toolCallId || "",
      }),
      signal: controller.signal,
    }).finally(() => clearTimeout(timer));
    const answer = await response.json();
    if (answer?.status !== "ok" || answer?.data?.allow !== false) return "";
    return answer.data.reason || `the permissions of agent ${AGENT} do not allow ${toolName}`;
  } catch {
    return "";
  }
}

// The plugin entry. definePluginEntry is not importable from outside the
// OpenClaw bundle, so this is the object it would have built: a permissive
// config schema, since nothing here is configured from openclaw.json.
export default {
  id: __CASBIN_GATEWAY_PLUGIN_ID__,
  name: "Casbin Gateway Agent Permissions",
  description: "Refuses a tool call the agent's Casbin Gateway permissions do not allow",
  configSchema: {
    safeParse: (value) => ({ success: true, data: value }),
    jsonSchema: { type: "object", additionalProperties: true },
  },
  register(api) {
    // block is terminal in OpenClaw: once a handler sets it, the call is
    // refused and blockReason is what the model is told.
    api.on("before_tool_call", async (event, ctx) => {
      const reason = await decide(
        event?.toolName || ctx?.toolName,
        ctx?.sessionKey || event?.sessionKey,
        event?.toolCallId || ctx?.toolCallId,
      );
      return reason ? { block: true, blockReason: reason } : {};
    });
  },
};
