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

// Package agenthook normalizes Claude Code command-hook events and sends them
// to the Gateway process that owns the in-memory monitoring buffer.
package agenthook

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/auditutil"
)

const (
	// Subcommand is passed from Claude Code's hook configuration to Gateway.
	Subcommand    = "agent-hook"
	OwnershipFlag = "--casbin-gateway-agent-monitor"

	maxHookInput  = 8 * 1024 * 1024
	reportTimeout = 5 * time.Second
	// hookLifetime bounds the whole process: the report has a timeout, reading
	// the event does not, and an event that never arrives leaks a copy of this
	// executable.
	hookLifetime = 30 * time.Second
)

// ServeIfInvoked handles an agent-launched hook process before Gateway starts
// its normal HTTP server. Hook delivery is best effort and must never affect a
// Claude Code action, so errors are intentionally ignored here.
func ServeIfInvoked() {
	if len(os.Args) < 2 || os.Args[1] != Subcommand {
		return
	}
	timer := time.AfterFunc(hookLifetime, terminateSelf)
	defer timer.Stop()
	code, _ := Run(os.Args[2:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}

// Run reads one hook event, decides it where the agent is waiting on a verdict,
// and reports its normalized record. The exit code it answers with is the one
// the agent reads: some take a refusal on stdout, Cascade takes it as a code.
func Run(args []string, input io.Reader, output io.Writer, errOutput io.Writer) (int, error) {
	flags := flag.NewFlagSet(Subcommand, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	managed := flags.Bool("casbin-gateway-agent-monitor", false, "Gateway-managed monitor hook")
	agentID := flags.String("agent", "", "agent that invoked this hook")
	recordsURL := flags.String("records-url", "", "Gateway agent record endpoint")
	agentPath := flags.String("agent-path", "", "agent installation path")
	owner := flags.String("user", "", "agent installation owner")
	ingestToken := flags.String("ingest-token", "", "credential presented to the Gateway record endpoint")
	decisionURL := flags.String("decision-url", "", "Gateway endpoint that decides a tool call")
	if err := flags.Parse(args); err != nil {
		return 0, err
	}
	normalize, known := normalizers[*agentID]
	if !*managed || !known {
		return 0, fmt.Errorf("unsupported hook agent %q", *agentID)
	}

	decoder := json.NewDecoder(io.LimitReader(input, maxHookInput))
	decoder.UseNumber()
	var event map[string]any
	if err := decoder.Decode(&event); err != nil {
		return 0, err
	}
	// The verdict comes before the record: the agent is held up until this hook
	// answers, and the report is best effort either way, so nothing below may
	// lose the code the verdict decided.
	code := 0
	if tool, ok := preToolEvent(*agentID, event); ok {
		request := map[string]any{
			"agent":      *agentID,
			"tool":       tool,
			"sessionKey": firstString(event, "session_id", "conversation_id", "trajectory_id"),
			"toolUseId":  stringValue(event["tool_use_id"]),
		}
		if allow, reason := allowed(*decisionURL, *ingestToken, request); !allow {
			code = writeDenial(output, errOutput, *agentID, reason)
		}
	}

	record := normalize(event, *agentPath, time.Now())
	if record == nil || *recordsURL == "" {
		return code, nil
	}
	if *owner != "" {
		record.User = *owner
	}
	body, err := json.Marshal(record)
	if err != nil {
		return code, err
	}
	request, err := http.NewRequest(http.MethodPost, *recordsURL, bytes.NewReader(body))
	if err != nil {
		return code, err
	}
	request.Header.Set("Content-Type", "application/json")
	if *ingestToken != "" {
		request.Header.Set(agentmonitor.IngestTokenHeader, *ingestToken)
	}
	response, err := (&http.Client{Timeout: reportTimeout}).Do(request)
	if err != nil {
		return code, err
	}
	return code, response.Body.Close()
}

// normalizers maps each agent that reports through a command hook to the
// reader of its own event schema. An agent absent from here has no hook Gateway
// installs, so a process claiming to be one is rejected.
var normalizers = map[string]func(map[string]any, string, time.Time) *agentmonitor.Record{
	"claude-code": Normalize,
	"cursor":      NormalizeCursor,
	"gemini-cli":  NormalizeGemini,
	"qwen-code":   NormalizeQwen,
	"windsurf":    NormalizeWindsurf,
}

// Normalize maps Claude Code's command-hook event schema to Gateway's agent
// monitoring record. Unknown events are not monitoring events and are ignored.
func Normalize(event map[string]any, agentPath string, now time.Time) *agentmonitor.Record {
	eventName, _ := event["hook_event_name"].(string)
	record := &agentmonitor.Record{
		Agent:       "claude-code",
		AgentPath:   agentPath,
		User:        stringValue(event["user"]),
		CreatedTime: now.Format(time.RFC3339Nano),
		SessionKey:  stringValue(event["session_id"]),
		PromptId:    stringValue(event["prompt_id"]),
		ToolUseId:   stringValue(event["tool_use_id"]),
		ToolName:    stringValue(event["tool_name"]),
		Model:       stringValue(event["model"]),
		DurationMs:  int64Value(event["duration_ms"]),
	}

	switch eventName {
	case "SessionStart":
		record.EventType, record.Action = "session", "start"
	case "SessionEnd":
		record.EventType, record.Action = "session", "end"
	case "Stop":
		record.EventType, record.Action, record.Outcome = "session", "stop", "success"
	case "StopFailure":
		record.EventType, record.Action, record.Outcome = "session", "stop", "failure"
		record.Detail = auditutil.SanitizeString(stringValue(event["error"]))
	case "UserPromptSubmit":
		record.EventType, record.Action, record.Outcome = "prompt", "submitted", "attempted"
	case "PreToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "attempted"
	case "PostToolUse":
		record.EventType, record.Action, record.Outcome = "tool", "call", "success"
	case "PostToolUseFailure":
		record.EventType, record.Action, record.Outcome = "tool", "call", "failure"
		record.Detail = auditutil.SanitizeString(stringValue(event["error"]))
	case "PermissionRequest":
		record.EventType, record.Action, record.Outcome = "permission", "requested", "attempted"
	case "PermissionDenied":
		record.EventType, record.Action, record.Outcome = "permission", "denied", "denied"
		record.Detail = auditutil.SanitizeString(stringValue(event["reason"]))
	case "SubagentStart":
		record.EventType, record.Action = "subagent", "start"
	case "SubagentStop":
		record.EventType, record.Action, record.Outcome = "subagent", "stop", "success"
	case "PreCompact":
		record.EventType, record.Action, record.Outcome = "compact", "before", "attempted"
	case "PostCompact":
		record.EventType, record.Action, record.Outcome = "compact", "after", "success"
	default:
		return nil
	}

	if server, tool, ok := auditutil.ParseMcpTool(record.ToolName, "mcp__"); ok {
		record.McpServer, record.McpTool = server, tool
		if record.EventType == "tool" {
			record.EventType = "mcp"
		}
	}
	record.Object = auditutil.EncodeBoundedJSON(payloadFor(event, record.ToolName), auditutil.MaxPayloadBytes)
	return record
}

func payloadFor(event map[string]any, toolName string) map[string]any {
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

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func int64Value(value any) int64 {
	number, ok := value.(json.Number)
	if !ok {
		return 0
	}
	result, _ := number.Int64()
	return result
}
