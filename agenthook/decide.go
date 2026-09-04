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

// This file is the blocking half of the hook. A pre-tool event asks Gateway
// whether the call may go ahead, and an agent that speaks a refusal writes it
// on stdout in its own words. Everything else about the hook stays an
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

// preToolEvent reports whether this event is the one asked before a tool runs,
// by the name each agent gives it, and the tool it is about.
func preToolEvent(agentID string, event map[string]any) (string, bool) {
	if agentID != "claude-code" {
		return "", false
	}
	if name, _ := event["hook_event_name"].(string); name != "PreToolUse" {
		return "", false
	}
	tool, _ := event["tool_name"].(string)
	return tool, tool != ""
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

// writeDenial tells the agent to refuse the call, in the shape it reads. Claude
// Code takes a permission decision off the hook's stdout; an agent with no such
// shape is left alone, since a hook that only observes must print nothing.
func writeDenial(out io.Writer, agentID string, reason string) {
	if agentID != "claude-code" {
		return
	}
	_ = json.NewEncoder(out).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	})
}
