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
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/auditutil"
)

// CursorEvents are the hook events Gateway installs. Cursor also reports every
// file read and every sentence the model produces; those say what the agent saw
// and thought rather than what it did, and arrive far too often to record.
var CursorEvents = []string{
	"sessionStart", "sessionEnd", "beforeSubmitPrompt", "stop",
	"preToolUse", "postToolUse", "postToolUseFailure",
	"beforeShellExecution", "afterShellExecution",
	"beforeMCPExecution", "afterMCPExecution",
	"afterFileEdit", "subagentStart", "subagentStop", "preCompact",
}

// NormalizeCursor maps Cursor's hook event schema to Gateway's agent monitoring
// record. The Cursor CLI reports a subset of these events, so a record arrives
// for whichever of them the front end in use actually fires.
func NormalizeCursor(event map[string]any, agentPath string, now time.Time) *agentmonitor.Record {
	record := &agentmonitor.Record{
		Agent:       "cursor",
		AgentPath:   agentPath,
		CreatedTime: now.Format(time.RFC3339Nano),
		SessionKey:  firstString(event, "conversation_id", "session_id"),
		PromptId:    stringValue(event["generation_id"]),
		ToolUseId:   stringValue(event["tool_use_id"]),
		ToolName:    stringValue(event["tool_name"]),
		Model:       stringValue(event["model"]),
		DurationMs:  int64Value(event["duration_ms"]),
	}

	switch stringValue(event["hook_event_name"]) {
	case "sessionStart":
		record.EventType, record.Action = "session", "start"
	case "sessionEnd":
		record.EventType, record.Action = "session", "end"
		record.Detail = auditutil.SanitizeString(stringValue(event["reason"]))
	case "stop":
		record.EventType, record.Action = "session", "stop"
		record.Outcome = cursorStopOutcome(stringValue(event["status"]))
	case "beforeSubmitPrompt":
		record.EventType, record.Action, record.Outcome = "prompt", "submitted", "attempted"
		record.Title = auditutil.SanitizeString(stringValue(event["prompt"]))
	case "preToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "attempted"
	case "postToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "success"
		record.DurationMs = int64Value(event["duration"])
	case "postToolUseFailure":
		record.EventType, record.Action, record.Outcome = "tool", "call", "failure"
		record.Detail = auditutil.SanitizeString(stringValue(event["error"]))
	case "beforeShellExecution":
		record.EventType, record.Action, record.Outcome = "tool", "call", "attempted"
		record.ToolName, record.Title = "shell", auditutil.SanitizeString(stringValue(event["command"]))
	case "afterShellExecution":
		record.EventType, record.Action, record.Outcome = "tool", "call", "success"
		record.ToolName, record.Title = "shell", auditutil.SanitizeString(stringValue(event["command"]))
	case "beforeMCPExecution":
		record.EventType, record.Action, record.Outcome = "mcp", "call", "attempted"
	case "afterMCPExecution":
		record.EventType, record.Action, record.Outcome = "mcp", "call", "success"
	case "afterFileEdit":
		record.EventType, record.Action, record.Outcome = "file", "edited", "success"
		record.Object = auditutil.EncodeBoundedJSON(cursorEditPayload(event), auditutil.MaxPayloadBytes)
		return record
	case "subagentStart":
		record.EventType, record.Action = "subagent", "start"
	case "subagentStop":
		record.EventType, record.Action, record.Outcome = "subagent", "stop", "success"
	case "preCompact":
		record.EventType, record.Action, record.Outcome = "compact", "before", "attempted"
	default:
		return nil
	}

	record.Object = auditutil.EncodeBoundedJSON(cursorPayload(event, record.ToolName), auditutil.MaxPayloadBytes)
	return record
}

func cursorStopOutcome(status string) string {
	switch status {
	case "completed":
		return "success"
	case "":
		return ""
	default:
		return "failure"
	}
}

func cursorPayload(event map[string]any, toolName string) map[string]any {
	payload := make(map[string]any, len(event))
	for key, value := range event {
		switch key {
		case "transcript_path", "tool_output":
			continue
		case "tool_input":
			payload[key] = auditutil.SanitizeToolInput(toolName, value)
		default:
			payload[key] = auditutil.SanitizeValue(key, value)
		}
	}
	return payload
}

// cursorEditPayload records that a file changed, not how. The edits carry the
// file's own text on both sides of the change, which is the file rather than a
// description of the action.
func cursorEditPayload(event map[string]any) map[string]any {
	edits, _ := event["edits"].([]any)
	return map[string]any{
		"file_path": auditutil.SanitizeValue("file_path", event["file_path"]),
		"edits":     len(edits),
	}
}

func firstString(event map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(event[key]); value != "" {
			return value
		}
	}
	return ""
}
