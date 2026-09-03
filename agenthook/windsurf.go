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

// WindsurfEvents are the hook events Gateway installs. Cascade also reports
// every file Cascade reads and can write a whole transcript after each turn;
// both are the content rather than the action, and arrive far too often.
var WindsurfEvents = []string{
	"pre_user_prompt", "pre_run_command", "post_run_command",
	"pre_write_code", "post_write_code",
	"pre_mcp_tool_use", "post_mcp_tool_use", "post_cascade_response",
}

// NormalizeWindsurf maps Cascade's hook event schema to Gateway's agent
// monitoring record. Every event carries its own fields in tool_info.
func NormalizeWindsurf(event map[string]any, agentPath string, now time.Time) *agentmonitor.Record {
	info, _ := event["tool_info"].(map[string]any)
	if info == nil {
		info = map[string]any{}
	}

	record := &agentmonitor.Record{
		Agent:       "windsurf",
		AgentPath:   agentPath,
		CreatedTime: now.Format(time.RFC3339Nano),
		SessionKey:  stringValue(event["trajectory_id"]),
		PromptId:    stringValue(event["execution_id"]),
		Model:       stringValue(event["model_name"]),
	}
	payload := map[string]any{}

	switch stringValue(event["agent_action_name"]) {
	case "pre_user_prompt":
		record.EventType, record.Action, record.Outcome = "prompt", "submitted", "attempted"
		record.Title = auditutil.SanitizeString(stringValue(info["user_prompt"]))
		payload["user_prompt"] = record.Title
	case "pre_run_command", "post_run_command":
		record.EventType, record.Action, record.ToolName = "tool", "call", "shell"
		record.Outcome = windsurfOutcome(stringValue(event["agent_action_name"]))
		record.Title = auditutil.SanitizeString(stringValue(info["command_line"]))
		payload["command_line"], payload["cwd"] = record.Title, auditutil.SanitizeValue("cwd", info["cwd"])
	case "pre_write_code", "post_write_code":
		record.EventType, record.Action = "file", "edited"
		record.Outcome = windsurfOutcome(stringValue(event["agent_action_name"]))
		// The edits carry the file's own text on both sides of the change,
		// which is the file rather than a description of the action.
		edits, _ := info["edits"].([]any)
		payload["file_path"], payload["edits"] = auditutil.SanitizeValue("file_path", info["file_path"]), len(edits)
	case "pre_mcp_tool_use", "post_mcp_tool_use":
		record.EventType, record.Action = "mcp", "call"
		record.Outcome = windsurfOutcome(stringValue(event["agent_action_name"]))
		record.McpServer = stringValue(info["mcp_server_name"])
		record.McpTool = stringValue(info["mcp_tool_name"])
		record.ToolName = record.McpTool
		payload["mcp_tool_arguments"] = auditutil.SanitizeToolInput(record.McpTool, info["mcp_tool_arguments"])
	case "post_cascade_response":
		record.EventType, record.Action, record.Outcome = "session", "stop", "success"
		payload["response_length"] = len(stringValue(info["response"]))
	default:
		return nil
	}

	record.Object = auditutil.EncodeBoundedJSON(payload, auditutil.MaxPayloadBytes)
	return record
}

// windsurfOutcome reads the half of the pair an event belongs to: a pre hook
// reports an action that is about to run, a post hook one that has.
func windsurfOutcome(event string) string {
	if len(event) >= 4 && event[:4] == "pre_" {
		return "attempted"
	}
	return "success"
}
