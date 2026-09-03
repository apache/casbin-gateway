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

// GeminiEvents are the hook events Gateway installs. The Gemini CLI also
// reports the model requests themselves, which carry the whole conversation on
// every turn and say nothing about what the agent did with it.
var GeminiEvents = []string{
	"SessionStart", "SessionEnd", "BeforeAgent", "AfterAgent",
	"BeforeTool", "AfterTool", "Notification", "PreCompress",
}

// NormalizeGemini maps the Gemini CLI's hook event schema to Gateway's agent
// monitoring record. Unknown events are not monitoring events and are ignored.
func NormalizeGemini(event map[string]any, agentPath string, now time.Time) *agentmonitor.Record {
	record := &agentmonitor.Record{
		Agent:       "gemini-cli",
		AgentPath:   agentPath,
		CreatedTime: now.Format(time.RFC3339Nano),
		SessionKey:  stringValue(event["session_id"]),
		ToolName:    stringValue(event["tool_name"]),
	}

	switch stringValue(event["hook_event_name"]) {
	case "SessionStart":
		record.EventType, record.Action = "session", "start"
	case "SessionEnd":
		record.EventType, record.Action = "session", "end"
		record.Detail = auditutil.SanitizeString(stringValue(event["reason"]))
	case "BeforeAgent":
		record.EventType, record.Action, record.Outcome = "prompt", "submitted", "attempted"
		record.Title = auditutil.SanitizeString(stringValue(event["prompt"]))
	case "AfterAgent":
		record.EventType, record.Action, record.Outcome = "session", "stop", "success"
	case "BeforeTool":
		record.EventType, record.Action, record.Outcome = "tool", "call", "attempted"
	case "AfterTool":
		record.EventType, record.Action, record.Outcome = "tool", "call", "success"
	case "Notification":
		record.EventType, record.Action = "notification", stringValue(event["notification_type"])
		record.Detail = auditutil.SanitizeString(stringValue(event["message"]))
	case "PreCompress":
		record.EventType, record.Action, record.Outcome = "compact", "before", "attempted"
	default:
		return nil
	}

	if record.Action == "" {
		record.Action = "observed"
	}
	record.Object = auditutil.EncodeBoundedJSON(geminiPayload(event, record.ToolName), auditutil.MaxPayloadBytes)
	return record
}

// geminiPayload drops what a record cannot carry: the transcript is a path into
// the agent's own storage, and a tool response or a model turn is the content
// itself rather than a description of the action.
func geminiPayload(event map[string]any, toolName string) map[string]any {
	payload := make(map[string]any, len(event))
	for key, value := range event {
		switch key {
		case "transcript_path", "tool_response", "llm_request", "llm_response":
			continue
		case "prompt_response":
			payload[key+"_length"] = len(stringValue(value))
		case "tool_input":
			payload[key] = auditutil.SanitizeToolInput(toolName, value)
		default:
			payload[key] = auditutil.SanitizeValue(key, value)
		}
	}
	return payload
}
