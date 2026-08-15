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

package agentmonitor

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/apache/casbin-gateway/auditutil"
)

type codexRolloutEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexRolloutPayload struct {
	ID            string         `json:"id"`
	Originator    string         `json:"originator"`
	TurnID        string         `json:"turn_id"`
	Model         string         `json:"model"`
	Type          string         `json:"type"`
	Role          string         `json:"role"`
	Name          string         `json:"name"`
	CallID        string         `json:"call_id"`
	Status        string         `json:"status"`
	ExitCode      int            `json:"exit_code"`
	Arguments     any            `json:"arguments"`
	Input         string         `json:"input"`
	Action        map[string]any `json:"action"`
	Query         string         `json:"query"`
	RevisedPrompt string         `json:"revised_prompt"`
	Content       []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Info struct {
		LastTokenUsage struct {
			Input           json.Number `json:"input_tokens"`
			CachedInput     json.Number `json:"cached_input_tokens"`
			Output          json.Number `json:"output_tokens"`
			ReasoningOutput json.Number `json:"reasoning_output_tokens"`
			Total           json.Number `json:"total_tokens"`
		} `json:"last_token_usage"`
		ContextWindow json.Number `json:"model_context_window"`
	} `json:"info"`
	Invocation struct {
		Server    string         `json:"server"`
		Tool      string         `json:"tool"`
		Arguments map[string]any `json:"arguments"`
	} `json:"invocation"`
	Result any `json:"result"`
}

func parseCodexHeader(line []byte) codexHeaderMeta {
	entry, payload, ok := decodeCodexLine(line)
	if !ok || entry.Type != "session_meta" {
		return codexHeaderMeta{}
	}
	return codexHeaderMeta{SessionKey: payload.ID, AgentID: codexAgentForSource(payload.Originator)}
}

func parseCodexRolloutLine(line []byte, cursor *codexCursor, claim *codexClaim) []*Record {
	if cursor == nil {
		return nil
	}
	if cursor.Pending == nil {
		cursor.Pending = map[string]codexPendingCall{}
	}
	entry, payload, ok := decodeCodexLine(line)
	if !ok {
		return nil
	}
	when := codexTimestamp(entry.Timestamp)

	switch entry.Type {
	case "session_meta":
		cursor.SessionKey = payload.ID
		cursor.AgentID = codexAgentForSource(payload.Originator)
	case "turn_context":
		cursor.TurnID, cursor.Model = payload.TurnID, payload.Model
	case "event_msg":
		switch payload.Type {
		case "task_started", "task_complete", "turn_aborted":
			resetCodexCursorTurn(cursor)
		case "token_count":
			return codexUsageRecord(payload, cursor, claim, when)
		case "exec_command_end":
			return codexFinishTool(payload, cursor, claim, when, "exec")
		case "mcp_tool_call_end":
			return codexFinishTool(payload, cursor, claim, when, "mcp")
		case "web_search_end":
			payload.Status = "completed"
			if payload.Action == nil {
				payload.Action = map[string]any{}
			}
			payload.Action["query"] = payload.Query
			return codexHostedTool(payload, "web_search", cursor, claim, when)
		}
	case "response_item":
		switch payload.Type {
		case "message":
			return codexMessageRecord(payload, cursor, claim, when)
		case "function_call", "custom_tool_call":
			codexRememberTool(payload, cursor, when)
		case "function_call_output":
			if call, exists := cursor.Pending[payload.CallID]; exists && call.Name == "exec_command" {
				return nil
			}
			return codexFinishTool(payload, cursor, claim, when, "")
		case "custom_tool_call_output", "tool_search_output":
			return codexFinishTool(payload, cursor, claim, when, "")
		case "tool_search_call", "image_generation_call", "local_shell_call":
			return codexHostedTool(payload, strings.TrimSuffix(payload.Type, "_call"), cursor, claim, when)
		}
	}
	return nil
}

func decodeCodexLine(line []byte) (codexRolloutEntry, codexRolloutPayload, bool) {
	var entry codexRolloutEntry
	var payload codexRolloutPayload
	if len(line) == 0 || json.Unmarshal(line, &entry) != nil || entry.Type == "" || json.Unmarshal(entry.Payload, &payload) != nil {
		return entry, payload, false
	}
	return entry, payload, true
}

func codexAgentForSource(originator string) string {
	source := strings.ToLower(strings.TrimSpace(originator))
	source = strings.NewReplacer("_", "-", " ", "-").Replace(source)
	switch source {
	case "codex-desktop":
		return "codex"
	case "codex-tui", "codex-exec", "codex-vscode":
		return "codex-cli"
	default:
		return ""
	}
}

func codexMessageRecord(payload codexRolloutPayload, cursor *codexCursor, claim *codexClaim, when time.Time) []*Record {
	if !codexClaimMatches(cursor, claim) || cursor.TurnID == "" {
		return nil
	}
	role := strings.ToLower(payload.Role)
	if role != "user" && role != "assistant" {
		return nil
	}
	length, images := 0, 0
	for _, item := range payload.Content {
		switch item.Type {
		case "input_text", "output_text":
			length += utf8.RuneCountInString(item.Text)
		case "input_image", "output_image":
			images++
		}
	}
	if length == 0 && images == 0 {
		return nil
	}
	action, outcome := "request", "attempted"
	if role == "assistant" {
		action, outcome = "response", "success"
	}
	body := map[string]any{"contentLength": length}
	if images > 0 {
		body["imageCount"] = images
	}
	record := codexBaseRecord(cursor, claim, when)
	record.EventType, record.Action, record.Outcome = "llm", action, outcome
	record.PromptId = cursor.TurnID
	record.Object = auditutil.EncodeBoundedJSON(body, auditutil.MaxPayloadBytes)
	return []*Record{record}
}

func codexUsageRecord(payload codexRolloutPayload, cursor *codexCursor, claim *codexClaim, when time.Time) []*Record {
	if !codexClaimMatches(cursor, claim) || cursor.TurnID == "" {
		return nil
	}
	usage := payload.Info.LastTokenUsage
	safe := map[string]any{}
	for _, value := range []struct {
		key   string
		value json.Number
	}{
		{"input_tokens", usage.Input}, {"cached_input_tokens", usage.CachedInput},
		{"output_tokens", usage.Output}, {"reasoning_output_tokens", usage.ReasoningOutput},
		{"total_tokens", usage.Total}, {"model_context_window", payload.Info.ContextWindow},
	} {
		if value.value != "" {
			safe[value.key] = value.value
		}
	}
	if len(safe) == 0 {
		return nil
	}
	record := codexBaseRecord(cursor, claim, when)
	record.EventType, record.Action, record.Outcome = "llm", "usage", "success"
	record.PromptId = cursor.TurnID
	record.Object = auditutil.EncodeBoundedJSON(safe, auditutil.MaxPayloadBytes)
	return []*Record{record}
}

func codexRememberTool(payload codexRolloutPayload, cursor *codexCursor, when time.Time) {
	if payload.CallID != "" && payload.Name != "" {
		cursor.Pending[payload.CallID] = codexPendingCall{
			Name: payload.Name, StartedAt: when, TurnID: cursor.TurnID,
			Object: codexToolObject(payload, payload.Name),
		}
	}
}

func codexToolObject(payload codexRolloutPayload, toolName string) string {
	var input any
	switch arguments := payload.Arguments.(type) {
	case string:
		var decoded map[string]any
		if json.Unmarshal([]byte(arguments), &decoded) == nil {
			input = decoded
		}
	case map[string]any:
		input = arguments
	}
	if payload.Input != "" {
		value := payload.Input
		if toolName == "apply_patch" || strings.Contains(value, "tools.apply_patch(") {
			value = "[OMITTED: patch content]"
		}
		input = map[string]any{"input": value}
	}
	if payload.Action != nil {
		input = payload.Action
	}
	if payload.RevisedPrompt != "" {
		input = map[string]any{"prompt": payload.RevisedPrompt}
	}
	if payload.Invocation.Arguments != nil {
		input = payload.Invocation.Arguments
	}
	if input == nil {
		return ""
	}
	return auditutil.EncodeBoundedJSON(map[string]any{"input": auditutil.SanitizeToolInput(toolName, input)}, auditutil.MaxPayloadBytes)
}

func codexHostedTool(payload codexRolloutPayload, name string, cursor *codexCursor, claim *codexClaim, when time.Time) []*Record {
	outcome, terminal := codexStatus(payload.Status)
	callID := payload.CallID
	if callID == "" {
		callID = payload.ID
	}
	if callID == "" {
		if !terminal || !codexClaimMatches(cursor, claim) {
			return nil
		}
		call := codexPendingCall{Name: name, StartedAt: when, TurnID: cursor.TurnID, Object: codexToolObject(payload, name)}
		return codexToolPair(cursor, claim, "", call, when, outcome)
	}
	payload.CallID = callID
	if _, exists := cursor.Pending[callID]; !exists {
		payload.Name = name
		codexRememberTool(payload, cursor, when)
	}
	if !terminal {
		return nil
	}
	return codexFinishTool(payload, cursor, claim, when, "")
}

func codexFinishTool(payload codexRolloutPayload, cursor *codexCursor, claim *codexClaim, when time.Time, kind string) []*Record {
	call, found := cursor.Pending[payload.CallID]
	if !found {
		if kind != "mcp" || payload.CallID == "" || payload.Invocation.Server == "" || payload.Invocation.Tool == "" {
			return nil
		}
		call = codexPendingCall{Name: "mcp__" + payload.Invocation.Server + "__" + payload.Invocation.Tool, StartedAt: when, TurnID: cursor.TurnID}
	}
	delete(cursor.Pending, payload.CallID)
	if !codexClaimMatches(cursor, claim) {
		return nil
	}
	outcome, _ := codexStatus(payload.Status)
	if kind == "exec" && payload.ExitCode != 0 {
		outcome = "failure"
	}
	if kind == "mcp" {
		if payload.Invocation.Server != "" && payload.Invocation.Tool != "" {
			call.Name = "mcp__" + payload.Invocation.Server + "__" + payload.Invocation.Tool
		}
		if object := codexToolObject(payload, call.Name); object != "" {
			call.Object = object
		}
		if result, ok := payload.Result.(map[string]any); ok {
			if _, failed := result["Err"]; failed {
				outcome = "failure"
			}
		}
	}
	return codexToolPair(cursor, claim, payload.CallID, call, when, outcome)
}

func codexToolPair(cursor *codexCursor, claim *codexClaim, callID string, call codexPendingCall, when time.Time, outcome string) []*Record {
	return []*Record{
		codexToolRecord(cursor, claim, call.StartedAt, callID, call, "call", "attempted"),
		codexToolRecord(cursor, claim, when, callID, call, "result", outcome),
	}
}

func codexToolRecord(cursor *codexCursor, claim *codexClaim, when time.Time, callID string, call codexPendingCall, action, outcome string) *Record {
	record := codexBaseRecord(cursor, claim, when)
	record.EventType, record.Action, record.Outcome = "tool", action, outcome
	record.ToolUseId, record.ToolName, record.PromptId = callID, call.Name, call.TurnID
	if action == "call" {
		record.Object = call.Object
	}
	if server, tool, ok := auditutil.ParseMcpTool(call.Name, "mcp__"); ok {
		record.EventType, record.McpServer, record.McpTool = "mcp", server, tool
	}
	if action == "result" && !call.StartedAt.IsZero() {
		record.DurationMs = when.Sub(call.StartedAt).Milliseconds()
		if record.DurationMs < 0 {
			record.DurationMs = 0
		}
	}
	return record
}

func codexStatus(status string) (string, bool) {
	switch strings.ToLower(status) {
	case "failed", "error", "cancelled", "incomplete":
		return "failure", true
	case "denied":
		return "denied", true
	case "completed":
		return "success", true
	default:
		return "success", false
	}
}

func codexClaimMatches(cursor *codexCursor, claim *codexClaim) bool {
	return claim != nil && cursor.AgentID != "" && cursor.AgentID == claim.AgentID
}

func codexBaseRecord(cursor *codexCursor, claim *codexClaim, when time.Time) *Record {
	return &Record{
		CreatedTime: when.Format(time.RFC3339Nano),
		Agent:       cursor.AgentID, AgentPath: claim.Path, User: claim.Owner,
		SessionKey: cursor.SessionKey, Model: cursor.Model,
	}
}

func codexTimestamp(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	return time.Now()
}
