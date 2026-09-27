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

package agenthook

import (
	"os"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/auditutil"
)

// grokHookEnv is set by Grok's hook runner on every hook it starts, and on
// nothing else.
const grokHookEnv = "GROK_HOOK_EVENT"

// GrokEvents are the hook events Gateway installs for Grok Build, in the
// Claude Code names its hook files take.
var GrokEvents = []string{
	"SessionStart", "SessionEnd", "UserPromptSubmit", "Stop", "StopFailure",
	"PreToolUse", "PostToolUse", "PostToolUseFailure", "PermissionDenied",
	"SubagentStart", "SubagentStop", "Notification", "PreCompact", "PostCompact",
}

// InsideGrok reports that Grok started this process as a hook. Grok also runs
// the hooks Gateway installs for Claude Code and Cursor, and those must leave
// a Grok call to the hook Grok has of its own.
func InsideGrok() bool {
	return os.Getenv(grokHookEnv) != ""
}

// NormalizeGrok maps Grok Build's hook event schema to Gateway's agent
// monitoring record. Grok sends each field twice, camelCase and Claude Code's
// snake_case, and the latter is read here.
func NormalizeGrok(event map[string]any, agentPath string, now time.Time) *agentmonitor.Record {
	record := &agentmonitor.Record{
		Agent:       "grok-build",
		AgentPath:   agentPath,
		CreatedTime: now.Format(time.RFC3339Nano),
		SessionKey:  stringValue(event["session_id"]),
		PromptId:    stringValue(event["promptId"]),
		ToolUseId:   stringValue(event["tool_use_id"]),
		ToolName:    grokToolName(event),
		DurationMs:  int64Value(event["duration_ms"]),
	}

	switch stringValue(event["hook_event_name"]) {
	case "SessionStart":
		record.EventType, record.Action = "session", "start"
	case "SessionEnd":
		record.EventType, record.Action = "session", "end"
		record.Detail = auditutil.SanitizeString(stringValue(event["reason"]))
	case "Stop":
		// Grok fires one more Stop as the session closes, which SessionEnd
		// already records.
		if reason := stringValue(event["reason"]); reason != "" && reason != "end_turn" {
			return nil
		}
		record.EventType, record.Action, record.Outcome = "session", "stop", "success"
	case "StopFailure":
		record.EventType, record.Action, record.Outcome = "session", "stop", "failure"
		record.Detail = auditutil.SanitizeString(stringValue(event["error"]))
	case "UserPromptSubmit":
		record.EventType, record.Action, record.Outcome = "prompt", "submitted", "attempted"
		record.Title = auditutil.SanitizeString(stringValue(event["prompt"]))
	case "PreToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "attempted"
	case "PostToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "success"
	case "PostToolUseFailure":
		record.EventType, record.Action, record.Outcome = "tool", "call", "failure"
		record.Detail = auditutil.SanitizeString(firstString(event, "error", "errorDetails"))
	case "PermissionDenied":
		record.EventType, record.Action, record.Outcome = "permission", "denied", "denied"
		record.Detail = auditutil.SanitizeString(stringValue(event["reason"]))
	case "SubagentStart":
		record.EventType, record.Action = "subagent", "start"
	case "SubagentStop":
		record.EventType, record.Action, record.Outcome = "subagent", "stop", "success"
	case "Notification":
		record.EventType, record.Action = "notification", firstString(event, "notification_type", "notificationType")
		record.Detail = auditutil.SanitizeString(stringValue(event["message"]))
	case "PreCompact":
		record.EventType, record.Action, record.Outcome = "compact", "before", "attempted"
	case "PostCompact":
		record.EventType, record.Action, record.Outcome = "compact", "after", "success"
	default:
		return nil
	}

	if record.Action == "" {
		record.Action = "observed"
	}
	if server, tool, ok := auditutil.ParseMcpTool(record.ToolName, "mcp__"); ok {
		record.McpServer, record.McpTool = server, tool
		if record.EventType == "tool" {
			record.EventType = "mcp"
		}
	}
	record.Object = auditutil.EncodeBoundedJSON(grokPayload(event, record.ToolName), auditutil.MaxPayloadBytes)
	return record
}

// grokToolName names an MCP call the way the permissions catalogue does. Grok
// calls one "<server>__<tool>", or dispatches it through use_tool.
func grokToolName(event map[string]any) string {
	name := stringValue(event["tool_name"])
	if name == "use_tool" {
		if input, ok := event["tool_input"].(map[string]any); ok && stringValue(input["tool_name"]) != "" {
			name = stringValue(input["tool_name"])
		}
	}
	if strings.Contains(name, "__") && !strings.HasPrefix(name, "mcp__") {
		return "mcp__" + name
	}
	return name
}

// grokPayload keeps one copy of each field, and drops what a record cannot
// carry: paths into Grok's own storage and the content a tool returned.
func grokPayload(event map[string]any, toolName string) map[string]any {
	payload := make(map[string]any, len(event))
	for key, value := range event {
		switch key {
		case "transcript_path", "transcriptPath", "tool_response", "toolResult", "toolInput",
			"hookEventName", "sessionId", "permissionMode", "toolName", "toolUseId", "durationMs":
			continue
		case "lastAssistantMessage", "compactSummary":
			payload[key+"Length"] = len(stringValue(value))
		case "tool_input":
			payload[key] = auditutil.SanitizeToolInput(toolName, value)
		default:
			payload[key] = auditutil.SanitizeValue(key, value)
		}
	}
	return payload
}
