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

	"github.com/apache/casbin-gateway/auditutil"
)

type openAgentAuditEntry struct {
	Timestamp       string         `json:"timestamp"`
	SessionID       string         `json:"sessionId"`
	Type            string         `json:"type"`
	Tool            string         `json:"tool"`
	Server          string         `json:"server"`
	Model           string         `json:"model"`
	Arguments       map[string]any `json:"arguments"`
	ArgumentsLength int            `json:"argumentsLength"`
	ContentLength   int            `json:"contentLength"`
	Effect          string         `json:"effect"`
	Reason          string         `json:"reason"`
	Outcome         string         `json:"outcome"`
	Action          string         `json:"action"`
	DurationMs      int64          `json:"durationMs"`
}

func parseOpenAgentAuditLine(line []byte, cursor *openAgentCursor, claim *openAgentClaim) []*Record {
	if cursor == nil || claim == nil || len(line) == 0 {
		return nil
	}
	var entry openAgentAuditEntry
	if json.Unmarshal(line, &entry) != nil || entry.Type == "" {
		return nil
	}
	if entry.SessionID != "" {
		cursor.SessionKey = entry.SessionID
	}
	when := openAgentTimestamp(entry.Timestamp)
	switch entry.Type {
	case "tool_call":
		return openAgentToolRecord(entry, cursor, claim, when)
	case "llm_call":
		return openAgentLlmRecord(entry, cursor, claim, when)
	case "session":
		return openAgentSessionRecord(entry, cursor, claim, when)
	default:
		return nil
	}
}

func openAgentToolRecord(entry openAgentAuditEntry, cursor *openAgentCursor, claim *openAgentClaim, when time.Time) []*Record {
	if entry.Tool == "" {
		return nil
	}
	record := openAgentBaseRecord(cursor, claim, when)
	record.EventType, record.Action = "tool", "call"
	record.Outcome = openAgentOutcome(entry.Outcome, entry.Effect)
	record.Model, record.ToolName, record.DurationMs = entry.Model, entry.Tool, openAgentDuration(entry.DurationMs)
	if entry.Server != "" {
		record.EventType, record.McpServer, record.McpTool = "mcp", entry.Server, entry.Tool
		record.ToolName = "mcp__" + entry.Server + "__" + entry.Tool
	}
	body := map[string]any{}
	if len(entry.Arguments) > 0 {
		body["arguments"] = auditutil.SanitizeToolInput(record.ToolName, entry.Arguments)
	}
	if entry.ArgumentsLength > 0 {
		body["argumentsLength"] = entry.ArgumentsLength
	}
	if entry.Effect != "" {
		body["effect"] = entry.Effect
	}
	if len(body) > 0 {
		record.Object = auditutil.EncodeBoundedJSON(body, auditutil.MaxPayloadBytes)
	}
	record.Detail = auditutil.SanitizeString(entry.Reason)
	return []*Record{record}
}

func openAgentLlmRecord(entry openAgentAuditEntry, cursor *openAgentCursor, claim *openAgentClaim, when time.Time) []*Record {
	record := openAgentBaseRecord(cursor, claim, when)
	record.EventType, record.Action = "llm", openAgentActionOr(entry.Action, "call")
	record.Outcome, record.Model, record.DurationMs = openAgentOutcome(entry.Outcome, entry.Effect), entry.Model, openAgentDuration(entry.DurationMs)
	if entry.ContentLength > 0 {
		record.Object = auditutil.EncodeBoundedJSON(map[string]any{"contentLength": entry.ContentLength}, auditutil.MaxPayloadBytes)
	}
	return []*Record{record}
}

func openAgentSessionRecord(entry openAgentAuditEntry, cursor *openAgentCursor, claim *openAgentClaim, when time.Time) []*Record {
	action := openAgentActionOr(entry.Action, "start")
	if action != "start" && action != "end" {
		return nil
	}
	record := openAgentBaseRecord(cursor, claim, when)
	record.EventType, record.Action = "session", action
	return []*Record{record}
}

func openAgentBaseRecord(cursor *openAgentCursor, claim *openAgentClaim, when time.Time) *Record {
	return &Record{
		CreatedTime: when.Format(time.RFC3339Nano), Agent: openAgentAgentID,
		AgentPath: claim.Path, User: claim.Owner, SessionKey: cursor.SessionKey,
	}
}

func openAgentOutcome(outcome, effect string) string {
	if strings.EqualFold(effect, "deny") {
		return "denied"
	}
	if strings.TrimSpace(outcome) == "" {
		return "attempted"
	}
	return strings.ToLower(strings.TrimSpace(outcome))
}

func openAgentActionOr(action, fallback string) string {
	if action = strings.ToLower(strings.TrimSpace(action)); action != "" {
		return action
	}
	return fallback
}

func openAgentDuration(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func openAgentTimestamp(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	return time.Now()
}
