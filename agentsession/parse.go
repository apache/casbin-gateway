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
	"io"
)

// The parsers, named by a fingerprint's "format".
const (
	// FormatClaudeStreamJson is "--output-format stream-json", which the Claude
	// Code family all speak.
	FormatClaudeStreamJson = "claude-stream-json"
	// FormatCodexJson is "codex exec --json".
	FormatCodexJson = "codex-json"
	// FormatText is whatever the agent prints, taken as the answer. It is the
	// fallback for an agent with no machine-readable mode, and loses the tool
	// calls: those are still recorded by the monitor, just not shown here.
	FormatText = "text"
)

// maxLine is the longest line a parser accepts. One line can carry a whole file
// an agent just read, so the default scanner limit is far too small.
const maxLine = 8 * 1024 * 1024

func parse(format string, reader io.Reader, emit func(Event)) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLine)

	switch format {
	case FormatClaudeStreamJson:
		return parseClaude(scanner, emit)
	case FormatCodexJson:
		return parseCodex(scanner, emit)
	default:
		return parseText(scanner, emit)
	}
}

// jsonString renders a value that an agent may write either as a string or as
// something structured, which is how tool arguments and tool results arrive.
func jsonString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	text := ""
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return string(raw)
}
