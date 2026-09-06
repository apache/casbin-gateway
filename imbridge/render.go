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

package imbridge

import (
	"strings"

	"github.com/apache/casbin-gateway/agentsession"
)

// answerBuffer gathers one turn into the single message a chat shows. Tool calls
// are kept in the order they happened, between the prose around them, because
// what an agent did is most of what somebody watching wants to see.
type answerBuffer struct {
	parts []string
	// dirty says something has arrived since the last time this was written out.
	dirty bool
}

func (b *answerBuffer) add(event agentsession.Event) {
	switch event.Type {
	case agentsession.EventText:
		if event.Text == "" {
			return
		}
		// Prose arrives in fragments, so it is joined onto the piece before it
		// rather than starting a new one.
		if len(b.parts) > 0 && !strings.HasPrefix(b.parts[len(b.parts)-1], toolMarker) {
			b.parts[len(b.parts)-1] += event.Text
		} else {
			b.parts = append(b.parts, event.Text)
		}
	case agentsession.EventToolUse:
		b.parts = append(b.parts, toolMarker+toolLine(event))
	case agentsession.EventError:
		if event.Text != "" {
			b.parts = append(b.parts, "\n⚠ "+event.Text)
		}
	default:
		return
	}
	b.dirty = true
}

// toolMarker starts the pieces that are a tool rather than prose, so a fragment
// of prose is not appended onto one.
const toolMarker = "· "

// toolInputLimit keeps a command in the message without the message becoming the
// command.
const toolInputLimit = 120

func toolLine(event agentsession.Event) string {
	name := event.ToolName
	if name == "" {
		name = "tool"
	}
	input := strings.TrimSpace(strings.ReplaceAll(event.ToolInput, "\n", " "))
	if runes := []rune(input); len(runes) > toolInputLimit {
		input = string(runes[:toolInputLimit]) + "..."
	}
	if input == "" {
		return name
	}
	return name + "  " + input
}

// changed reports whether anything new has arrived, and clears the flag.
func (b *answerBuffer) changed() bool {
	was := b.dirty
	b.dirty = false
	return was
}

func (b *answerBuffer) text() string {
	return strings.TrimSpace(strings.Join(b.parts, "\n"))
}
