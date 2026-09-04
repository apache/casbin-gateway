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

// QwenEvents are the hook events Gateway installs. Qwen Code is a Gemini CLI
// fork that took Claude Code's event names, so these read like the latter's.
// MessageDisplay is left out: it fires on every chunk of a streaming reply and
// says nothing about what the agent did.
var QwenEvents = []string{
	"SessionStart", "SessionEnd", "UserPromptSubmit", "Stop",
	"PreToolUse", "PostToolUse", "PostToolUseFailure",
	"SubagentStart", "SubagentStop", "Notification",
	"PreCompact", "PostCompact",
}

// NormalizeQwen maps Qwen Code's hook event schema to Gateway's agent
// monitoring record. Unknown events are not monitoring events and are ignored.
func NormalizeQwen(event map[string]any, agentPath string, now time.Time) *agentmonitor.Record {
	record := &agentmonitor.Record{
		Agent:       "qwen-code",
		AgentPath:   agentPath,
		CreatedTime: now.Format(time.RFC3339Nano),
		SessionKey:  stringValue(event["session_id"]),
		ToolUseId:   stringValue(event["tool_use_id"]),
		ToolName:    stringValue(event["tool_name"]),
		Model:       stringValue(event["model"]),
	}

	switch stringValue(event["hook_event_name"]) {
	case "SessionStart":
		record.EventType, record.Action = "session", "start"
	case "SessionEnd":
		record.EventType, record.Action = "session", "end"
		record.Detail = auditutil.SanitizeString(stringValue(event["reason"]))
	case "Stop":
		record.EventType, record.Action, record.Outcome = "session", "stop", "success"
	case "UserPromptSubmit":
		record.EventType, record.Action, record.Outcome = "prompt", "submitted", "attempted"
		record.Title = auditutil.SanitizeString(stringValue(event["prompt"]))
	case "PreToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "attempted"
	case "PostToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "success"
	case "PostToolUseFailure":
		record.EventType, record.Action, record.Outcome = "tool", "call", "failure"
		record.Detail = auditutil.SanitizeString(stringValue(event["error"]))
	case "SubagentStart":
		record.EventType, record.Action = "subagent", "start"
	case "SubagentStop":
		record.EventType, record.Action, record.Outcome = "subagent", "stop", "success"
	case "Notification":
		record.EventType, record.Action = "notification", stringValue(event["notification_type"])
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
	record.Object = auditutil.EncodeBoundedJSON(qwenPayload(event, record.ToolName), auditutil.MaxPayloadBytes)
	return record
}

// qwenPayload drops what a record cannot carry: the transcript is a path into
// the agent's own storage, and a tool response or an assistant turn is the
// content itself rather than a description of the action.
func qwenPayload(event map[string]any, toolName string) map[string]any {
	payload := make(map[string]any, len(event))
	for key, value := range event {
		switch key {
		case "transcript_path", "tool_response":
			continue
		case "last_assistant_message", "compact_summary":
			payload[key+"_length"] = len(stringValue(value))
		case "tool_input":
			payload[key] = auditutil.SanitizeToolInput(toolName, value)
		default:
			payload[key] = auditutil.SanitizeValue(key, value)
		}
	}
	return payload
}
