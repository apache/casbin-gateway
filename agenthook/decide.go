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
	"fmt"
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
	// Cursor asks separately for a shell command, an MCP call, a file read and
	// a subagent, and none of those carries a tool name the general event would
	// have given.
	"cursor": {
		"preToolUse", "beforeShellExecution", "beforeMCPExecution",
		"beforeReadFile", "subagentStart",
	},
	// Cascade names an action rather than a tool, and asks about each kind of
	// action separately.
	"windsurf": {"pre_run_command", "pre_write_code", "pre_mcp_tool_use"},
}

// eventName is what the agent calls the event, read from the field it puts it
// in. Cascade is the one that names it something else.
func eventName(agentID string, event map[string]any) string {
	if agentID == "windsurf" {
		return stringValue(event["agent_action_name"])
	}
	return stringValue(event["hook_event_name"])
}

// preToolEvent reports whether this event is one the agent is waiting on, and
// the tool it is about, named the way the permissions catalogue names tools.
func preToolEvent(agentID string, event map[string]any) (string, bool) {
	name := eventName(agentID, event)
	if name == "" || !listed(preToolEvents[agentID], name) {
		return "", false
	}

	switch name {
	case "beforeShellExecution", "pre_run_command":
		// Only the command is carried, and running one is the whole of what
		// this event is.
		return "shell", true
	case "beforeReadFile":
		// Cursor asks this for every file whose content is about to enter the
		// context, an attachment as much as a tool call.
		return "Read", true
	case "subagentStart":
		return "Task", true
	case "pre_write_code":
		// Cascade edits a file in place, so this is the edit switch rather than
		// the one for creating a file.
		return "edit_file", true
	case "beforeMCPExecution":
		return mcpToolName(stringValue(event["mcp_server_name"]), stringValue(event["tool_name"]))
	case "pre_mcp_tool_use":
		info, _ := event["tool_info"].(map[string]any)
		if info == nil {
			info = map[string]any{}
		}
		return mcpToolName(stringValue(info["mcp_server_name"]), stringValue(info["mcp_tool_name"]))
	}

	tool := stringValue(event["tool_name"])
	return tool, tool != ""
}

// mcpToolName is how an MCP call is named where the agent reports the server
// and the tool apart: the permissions catalogue reads the server out of the
// combined name.
func mcpToolName(server string, tool string) (string, bool) {
	if server == "" {
		return tool, tool != ""
	}
	return "mcp__" + server + "__" + tool, true
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

// blockedExitCode is what Cascade reads a refusal as: it takes no answer on
// stdout, and blocks the action on this code alone.
const blockedExitCode = 2

// writeDenial tells the agent to refuse the call, in the shape it reads, and
// answers with the exit code that goes with it. An agent Gateway has no shape
// for is left alone: a hook that only observes must print nothing at all.
func writeDenial(out io.Writer, errOut io.Writer, agentID string, reason string) int {
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
	case "windsurf":
		// Cascade shows what the hook wrote on stderr and blocks on the code.
		_, _ = fmt.Fprintln(errOut, reason)
		return blockedExitCode
	default:
		return 0
	}
	_ = json.NewEncoder(out).Encode(answer)
	return 0
}
