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

import (
	"bufio"
	"encoding/json"
	"errors"
	"strings"
)

// codexLine is one record of "codex exec --json". Codex has written two shapes
// of these: the thread-and-item one, and an older one that wrapped everything
// in "msg". Both are read, because which one an installation speaks depends on
// its version rather than on anything Gateway controls.
type codexLine struct {
	Type     string      `json:"type"`
	ThreadId string      `json:"thread_id"`
	Item     *codexItem  `json:"item"`
	Usage    *codexUsage `json:"usage"`
	Error    *codexError `json:"error"`
	Message  string      `json:"message"`

	Msg *codexItem `json:"msg"`
}

type codexItem struct {
	Type     string `json:"type"`
	ItemType string `json:"item_type"`

	Text    string `json:"text"`
	Message string `json:"message"`

	Command          string `json:"command"`
	AggregatedOutput string `json:"aggregated_output"`

	Server string          `json:"server"`
	Tool   string          `json:"tool"`
	Args   json.RawMessage `json:"arguments"`
}

type codexUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type codexError struct {
	Message string `json:"message"`
}

func parseCodex(scanner *bufio.Scanner, emit func(Event)) error {
	failure := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != '{' {
			continue
		}

		record := codexLine{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		switch {
		case record.Type == "thread.started" && record.ThreadId != "":
			event := newEvent(EventUsage)
			event.NativeId = record.ThreadId
			emit(event)
		// Only a completed item is reported: the started and updated ones carry
		// the same text again as it grows, and showing each would repeat it.
		case record.Type == "item.completed" && record.Item != nil:
			emitCodexItem(record.Item, emit)
		case record.Type == "turn.completed":
			if record.Usage != nil {
				event := newEvent(EventUsage)
				event.InputTokens = record.Usage.InputTokens
				event.OutputTokens = record.Usage.OutputTokens
				emit(event)
			}
		case record.Type == "turn.failed" || record.Type == "error":
			failure = codexFailure(record)
		case record.Msg != nil:
			emitCodexItem(record.Msg, emit)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if failure != "" {
		return errors.New(failure)
	}
	return nil
}

func emitCodexItem(item *codexItem, emit func(Event)) {
	itemType := item.ItemType
	if itemType == "" {
		itemType = item.Type
	}

	switch itemType {
	case "agent_message":
		if text := firstOf(item.Text, item.Message); text != "" {
			emit(textEvent(EventText, text))
		}
	case "reasoning", "agent_reasoning":
		if text := firstOf(item.Text, item.Message); text != "" {
			emit(textEvent(EventThinking, text))
		}
	case "command_execution", "exec_command_begin":
		event := newEvent(EventToolUse)
		event.ToolName = "Shell"
		event.ToolInput = item.Command
		emit(event)
		if item.AggregatedOutput != "" {
			emit(textEvent(EventToolResult, item.AggregatedOutput))
		}
	case "mcp_tool_call":
		event := newEvent(EventToolUse)
		event.ToolName = strings.TrimPrefix(item.Server+"."+item.Tool, ".")
		event.ToolInput = jsonString(item.Args)
		emit(event)
	case "file_change", "patch_apply_begin":
		event := newEvent(EventToolUse)
		event.ToolName = "Edit"
		event.ToolInput = firstOf(item.Text, item.Message)
		emit(event)
	// An error item is Codex saying something went sideways without the turn
	// ending: a setting it dropped, a budget it trimmed to fit. A turn that
	// really failed says so through turn.failed, which fails the run.
	case "error":
		if text := firstOf(item.Message, item.Text); text != "" {
			emit(textEvent(EventNotice, text))
		}
	}
}

func codexFailure(record codexLine) string {
	if record.Error != nil && record.Error.Message != "" {
		return record.Error.Message
	}
	if record.Message != "" {
		return record.Message
	}
	return "the agent ended with an error"
}

func firstOf(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
