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

package agenthistory

import (
	"encoding/json"
	"strings"
)

// Transcript is one session read in full: the description a scan already has,
// and the conversation inside the file.
type Transcript struct {
	Session   Session   `json:"session"`
	Messages  []Message `json:"messages"`
	Truncated bool      `json:"truncated"`
}

// Message is one turn of the conversation, in the order the agent wrote it.
type Message struct {
	Role   string  `json:"role"`
	Time   string  `json:"time"`
	Blocks []Block `json:"blocks"`
}

// Block is one part of a message: prose, the model thinking aloud, a tool call
// or what that call returned.
type Block struct {
	Kind    string `json:"kind"`
	Text    string `json:"text"`
	Tool    string `json:"tool"`
	IsError bool   `json:"isError"`
}

const (
	blockText       = "text"
	blockThinking   = "thinking"
	blockToolUse    = "toolUse"
	blockToolResult = "toolResult"
	blockImage      = "image"
)

const (
	// maxMessages bounds one transcript, so that a session someone left running
	// for a week is not asked for in a single response.
	maxMessages = 2000
	// maxTextRunes and maxToolRunes cap what one block carries. The prose is
	// what the reader came for; a tool call's arguments and its output only have
	// to be recognisable.
	maxTextRunes = 20000
	maxToolRunes = 2000
)

// ReadTranscript reads the conversation out of a session a scan already found.
// The path comes from that scan rather than from the request, so this cannot be
// pointed at an arbitrary file.
//
// The messages are the same ones Scan counts, so the number the Sessions page
// shows is the number of messages this returns. An agent that keeps its
// sessions in something other than a plain JSONL file is read by the same
// reader the scan used, rather than by the one below.
func ReadTranscript(session Session) (Transcript, error) {
	switch session.Agent {
	case opencodeAgent:
		return readOpencodeTranscript(session)
	case cursorAgent:
		return readCursorTranscript(session)
	case dshAgent:
		return readDshTranscript(session)
	}
	return readJsonlTranscript(session)
}

func readJsonlTranscript(session Session) (Transcript, error) {
	transcript := Transcript{Session: session, Messages: []Message{}}

	// A line eachLine passed over is a message this cannot show, so it counts
	// towards the same warning as a session too long to send at once.
	skipped, err := eachLine(session.Path, func(data []byte) {
		var entry line
		if err := json.Unmarshal(data, &entry); err != nil {
			return
		}

		role, content := roleAndContent(entry)
		if role == "" {
			return
		}
		if len(transcript.Messages) >= maxMessages {
			transcript.Truncated = true
			return
		}

		transcript.Messages = append(transcript.Messages, Message{
			Role:   role,
			Time:   strings.TrimSpace(entry.Timestamp),
			Blocks: blocks(content),
		})
	})
	if err != nil {
		return Transcript{}, err
	}

	if skipped > 0 {
		transcript.Truncated = true
	}
	return transcript, nil
}

// contentPart is the union of the parts both formats put in a message body.
type contentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
	Content  json.RawMessage `json:"content"`
	IsError  bool            `json:"is_error"`
}

// blocks turns the body of a message into the parts a reader sees. Both formats
// allow a plain string or a list of typed parts.
func blocks(content json.RawMessage) []Block {
	if len(content) == 0 {
		return []Block{}
	}

	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return []Block{{Kind: blockText, Text: clip(text, maxTextRunes)}}
	}

	var parts []contentPart
	if err := json.Unmarshal(content, &parts); err != nil {
		return []Block{{Kind: blockText, Text: clip(string(content), maxToolRunes)}}
	}

	result := []Block{}
	for _, part := range parts {
		if block, ok := blockOf(part); ok {
			result = append(result, block)
		}
	}
	return result
}

func blockOf(part contentPart) (Block, bool) {
	switch part.Type {
	case "text", "input_text", "output_text":
		// An empty part carries nothing to read. Claude Code writes one for
		// thinking it did not keep, and rendering those would be 200 headings
		// with nothing under them.
		if strings.TrimSpace(part.Text) == "" {
			return Block{}, false
		}
		return Block{Kind: blockText, Text: clip(part.Text, maxTextRunes)}, true
	case "thinking", "reasoning":
		// Claude Code keeps the signature of a thinking block and not the words,
		// so this is usually empty. The block still stands for a turn that
		// happened, which is why it is not dropped.
		return Block{Kind: blockThinking, Text: clip(firstNonEmpty(part.Thinking, part.Text), maxTextRunes)}, true
	case "tool_use", "function_call":
		return Block{Kind: blockToolUse, Tool: part.Name, Text: clip(string(part.Input), maxToolRunes)}, true
	case "tool_result", "function_call_output":
		return Block{Kind: blockToolResult, Text: clip(plainText(part.Content), maxToolRunes), IsError: part.IsError}, true
	case "image", "input_image":
		return Block{Kind: blockImage}, true
	}
	return Block{}, false
}

// plainText reads the text out of a tool result, which either is one string or
// carries its text in a list of parts.
func plainText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return text
	}

	var parts []contentPart
	if err := json.Unmarshal(content, &parts); err != nil {
		return string(content)
	}

	pieces := []string{}
	for _, part := range parts {
		if strings.TrimSpace(part.Text) != "" {
			pieces = append(pieces, part.Text)
		}
	}
	return strings.Join(pieces, "\n")
}

// clip keeps one block to something a page can render, marking what it dropped.
func clip(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "\n..."
}
