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

// This file is the blocking half of the hook. The event an agent fires before
// it runs a tool asks Gateway whether the call may go ahead, and a refusal is
// written on stdout in that agent's own words. Every other event stays an
// observation.

package agenthook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
)

// decideTimeout bounds the question. The agent is waiting on this hook before
// it runs the tool, so a Gateway that does not answer quickly is treated as one
// that is not there.
const decideTimeout = 2 * time.Second

// decision is what Gateway answers.
type decision struct {
	Allow  bool   `json:"allow"`
	Reason string `json:"reason"`
}

// preToolEvents are the events each agent fires before running a tool, and can
// be told to refuse. An agent absent from here only observes.
var preToolEvents = map[string][]string{
	"claude-code": {"PreToolUse"},
	"qwen-code":   {"PreToolUse"},
	"gemini-cli":  {"BeforeTool"},
	// Cursor asks separately for a shell command and an MCP call, and neither
	// carries a tool name the general event would have given.
	"cursor": {"preToolUse", "beforeShellExecution", "beforeMCPExecution"},
}

// preToolEvent reports whether this event is one the agent is waiting on, and
// the tool it is about, named the way the permissions catalogue names tools.
func preToolEvent(agentID string, event map[string]any) (string, bool) {
	name := stringValue(event["hook_event_name"])
	if name == "" || !listed(preToolEvents[agentID], name) {
		return "", false
	}

	switch name {
	case "beforeShellExecution":
		// Only the command is carried, and running one is the whole of what
		// this event is.
		return "shell", true
	case "beforeMCPExecution":
		server := stringValue(event["mcp_server_name"])
		tool := stringValue(event["tool_name"])
		if server == "" {
			return tool, tool != ""
		}
		return "mcp__" + server + "__" + tool, true
	}

	tool := stringValue(event["tool_name"])
	return tool, tool != ""
}

func listed(values []string, name string) bool {
	for _, value := range values {
		if value == name {
			return true
		}
	}
	return false
}

// allowed asks Gateway whether this tool call may go ahead. Anything that goes
// wrong - no endpoint, no answer, a body that will not parse - allows the call:
// a hook that cannot get a verdict must not stop the agent from working.
func allowed(url string, token string, request map[string]any) (bool, string) {
	if url == "" {
		return true, ""
	}
	body, err := json.Marshal(request)
	if err != nil {
		return true, ""
	}
	httpRequest, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return true, ""
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpRequest.Header.Set(agentmonitor.IngestTokenHeader, token)
	}

	response, err := (&http.Client{Timeout: decideTimeout}).Do(httpRequest)
	if err != nil {
		return true, ""
	}
	defer response.Body.Close()

	answer := struct {
		Status string   `json:"status"`
		Data   decision `json:"data"`
	}{}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxHookInput)).Decode(&answer); err != nil {
		return true, ""
	}
	if answer.Status != "ok" || answer.Data.Allow {
		return true, ""
	}
	return false, answer.Data.Reason
}

// writeDenial tells the agent to refuse the call, in the shape it reads. The
// three schemas below are the agents' own, and an agent Gateway has no schema
// for is left alone: a hook that only observes must print nothing at all.
func writeDenial(out io.Writer, agentID string, reason string) {
	var answer map[string]any
	switch agentID {
	case "claude-code", "qwen-code":
		answer = map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":            "PreToolUse",
				"permissionDecision":       "deny",
				"permissionDecisionReason": reason,
			},
		}
	case "gemini-cli":
		answer = map[string]any{"decision": "deny", "reason": reason}
	case "cursor":
		answer = map[string]any{"permission": "deny", "user_message": reason, "agent_message": reason}
	default:
		return
	}
	_ = json.NewEncoder(out).Encode(answer)
}
