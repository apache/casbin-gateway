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

// Package agentsession drives an installed agent on somebody's behalf: it hands
// one prompt to the agent's own non-interactive mode and flattens what comes
// back into events. Nothing here knows where the prompt came from, so the same
// session is driven from the web page and from a chat platform.
//
// Driving an agent this way runs the real agent, with the configuration and the
// hooks Gateway already wrote into it. Its transcript, its usage and its tool
// permissions therefore keep working exactly as they do when somebody types at
// it, and none of that is reimplemented here.
package agentsession

import (
	"strings"
	"time"
)

// State is where a session is in its turn.
type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StateFailed  State = "failed"
)

// Spec is what a session is opened with.
type Spec struct {
	AgentId   string `json:"agentId"`
	AgentPath string `json:"agentPath"`
	Owner     string `json:"owner"`
	// WorkDir is the directory the agent runs in, which is the project it acts
	// on. Empty means the home of the account Gateway runs as.
	WorkDir string `json:"workDir"`
	// Model overrides the agent's own default for this session, empty to leave
	// it alone.
	Model string `json:"model"`
	// Source names who opened the session - "web", or "im:<platform>:<user>" -
	// and is what the records of this session are attributed to.
	Source string `json:"source"`
}

// Session is one conversation with one installed agent.
type Session struct {
	Id string `json:"id"`
	Spec

	// NativeId is the agent's own id for this conversation, which is what the
	// next turn resumes. It is either handed to the agent when the session is
	// opened, for one that takes it, or read out of the first turn's output.
	NativeId string `json:"nativeId"`
	// Resumable is false for an agent that carries nothing across turns, so the
	// page can say that every message stands alone rather than pretending.
	Resumable bool `json:"resumable"`

	Title       string `json:"title"`
	State       State  `json:"state"`
	Turns       int    `json:"turns"`
	CreatedTime string `json:"createdTime"`
	UpdatedTime string `json:"updatedTime"`
	// LastError is why the last turn failed, empty when it did not.
	LastError string `json:"lastError,omitempty"`
}

// titleLimit keeps a session's name to something a list and a chat window can
// both show on one line.
const titleLimit = 60

// titleFrom names a session after the first thing asked of it, which is what
// somebody looking at a list of them recognises.
func titleFrom(prompt string) string {
	title := ""
	for _, line := range strings.Split(prompt, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			title = trimmed
			break
		}
	}
	if len([]rune(title)) > titleLimit {
		title = string([]rune(title)[:titleLimit]) + "..."
	}
	return title
}

func now() string {
	return time.Now().Format(time.RFC3339Nano)
}
