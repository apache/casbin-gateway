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

package agentsession

import "time"

// EventType is what one piece of an agent's answer is.
type EventType string

const (
	// EventPrompt is what was asked. It is on the same feed as the answer so a
	// page that reconnects reads the whole conversation back from one place.
	EventPrompt     EventType = "prompt"
	EventText       EventType = "text"
	EventThinking   EventType = "thinking"
	EventToolUse    EventType = "toolUse"
	EventToolResult EventType = "toolResult"
	EventUsage      EventType = "usage"
	EventDone       EventType = "done"
	EventError      EventType = "error"
)

// Event is one piece of what an agent said, in a shape every agent is flattened
// into. The parsers own the differences so nothing downstream - the page, the
// chat bridges, the records - has to know which agent produced it.
type Event struct {
	Type EventType `json:"type"`
	// Seq counts the events of one session from the first. A page that lost its
	// connection says which one it had, and is sent only what came after, so a
	// reconnect does not repeat the conversation.
	Seq         int64  `json:"seq"`
	CreatedTime string `json:"createdTime"`

	// Text is the answer for EventText and EventThinking, and the message for
	// EventError.
	Text string `json:"text,omitempty"`

	ToolName  string `json:"toolName,omitempty"`
	ToolUseId string `json:"toolUseId,omitempty"`
	// ToolInput is the arguments the agent called the tool with, as JSON. It is
	// what an approval prompt has to show for a decision to mean anything.
	ToolInput string `json:"toolInput,omitempty"`

	Model string `json:"model,omitempty"`
	// NativeId is the agent's own id for this conversation, which is what a
	// later turn resumes. Reported by the parser for an agent Gateway cannot
	// name a session for up front.
	NativeId string `json:"nativeId,omitempty"`

	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	CostUsd      float64 `json:"costUsd,omitempty"`
	DurationMs   int64   `json:"durationMs,omitempty"`
}

func newEvent(eventType EventType) Event {
	return Event{Type: eventType, CreatedTime: time.Now().Format(time.RFC3339Nano)}
}

func textEvent(eventType EventType, text string) Event {
	event := newEvent(eventType)
	event.Text = text
	return event
}

func errorEvent(err error) Event {
	return textEvent(EventError, err.Error())
}
