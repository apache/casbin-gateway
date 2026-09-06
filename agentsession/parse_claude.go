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

// claudeLine is one record of "--output-format stream-json". Only the fields
// Gateway shows are named; the rest are ignored, which is what keeps this
// working across the releases that add to them.
type claudeLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionId string `json:"session_id"`
	Model     string `json:"model"`

	Message *claudeMessage `json:"message"`

	// The closing record of a run.
	IsError      bool         `json:"is_error"`
	Result       string       `json:"result"`
	TotalCostUsd float64      `json:"total_cost_usd"`
	DurationMs   int64        `json:"duration_ms"`
	Usage        *claudeUsage `json:"usage"`
}

type claudeMessage struct {
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
	Usage   *claudeUsage    `json:"usage"`
}

type claudeBlock struct {
	Type string `json:"type"`

	Text     string `json:"text"`
	Thinking string `json:"thinking"`

	Id    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`

	ToolUseId string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func parseClaude(scanner *bufio.Scanner, emit func(Event)) error {
	failure := ""
	// The agent names its session on every record it writes, and says so several
	// times before it answers. It is reported once.
	named := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] != '{' {
			continue
		}

		record := claudeLine{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		if !named && record.SessionId != "" {
			named = true
			event := newEvent(EventUsage)
			event.NativeId = record.SessionId
			event.Model = record.Model
			emit(event)
		}

		switch record.Type {
		case "assistant", "user":
			emitClaudeBlocks(record, emit)
		case "result":
			failure = emitClaudeResult(record, emit)
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

func emitClaudeBlocks(record claudeLine, emit func(Event)) {
	if record.Message == nil {
		return
	}

	blocks := []claudeBlock{}
	if err := json.Unmarshal(record.Message.Content, &blocks); err != nil {
		// A message whose content is a bare string is the whole answer.
		if text := jsonString(record.Message.Content); text != "" {
			emit(textEvent(EventText, text))
		}
		return
	}

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				event := textEvent(EventText, block.Text)
				event.Model = record.Message.Model
				emit(event)
			}
		case "thinking":
			if block.Thinking != "" {
				emit(textEvent(EventThinking, block.Thinking))
			}
		case "tool_use":
			event := newEvent(EventToolUse)
			event.ToolName = block.Name
			event.ToolUseId = block.Id
			event.ToolInput = jsonString(block.Input)
			emit(event)
		case "tool_result":
			event := newEvent(EventToolResult)
			event.ToolUseId = block.ToolUseId
			event.Text = jsonString(block.Content)
			emit(event)
		}
	}
}

// emitClaudeResult reports the closing record and returns why the run failed,
// empty when it did not.
func emitClaudeResult(record claudeLine, emit func(Event)) string {
	event := newEvent(EventUsage)
	event.CostUsd = record.TotalCostUsd
	event.DurationMs = record.DurationMs
	if record.Usage != nil {
		event.InputTokens = record.Usage.InputTokens
		event.OutputTokens = record.Usage.OutputTokens
	}
	// A run that reported nothing it cost says nothing here either.
	if event.CostUsd > 0 || event.DurationMs > 0 || event.InputTokens > 0 || event.OutputTokens > 0 {
		emit(event)
	}

	if !record.IsError {
		return ""
	}
	if record.Result != "" {
		return record.Result
	}
	if record.Subtype != "" {
		return record.Subtype
	}
	return "the agent ended with an error"
}
