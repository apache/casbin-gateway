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
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

const dshAgent = "dsh"

// dshTranscript is the fixed name every dsh session log is written under, in a
// directory of its own. The default encoding is a concatenation of Zstandard
// frames; a harness configured with compression off writes the same lines raw.
const (
	dshTranscript       = "session.jsonl"
	dshCompressedSuffix = ".zstd"
)

// scanDsh reads the sessions dsh keeps under its own home. The profile the
// harness ships points its persistence at $DSH_HOME/sessions, and every session
// is one directory under the project it ran in.
func scanDsh(home string) []Session {
	root := filepath.Join(home, ".dsh", "sessions")
	sessions := []Session{}
	for _, file := range newestTranscripts(root, isDshTranscript) {
		if session, ok := read(file, parseDsh); ok {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func isDshTranscript(name string) bool {
	return name == dshTranscript || name == dshTranscript+dshCompressedSuffix
}

// dshLine is the union of the two kinds of line a session log holds: the header
// that opens it, and the events appended after it. A packed chunk row carries
// neither an id nor an event type this reads, and is passed over.
type dshLine struct {
	Type      string          `json:"type"`
	Id        string          `json:"id"`
	Cwd       string          `json:"cwd"`
	CreatedAt json.RawMessage `json:"createdAt"`
	Time      int64           `json:"time"`
	Data      json.RawMessage `json:"data"`
}

func parseDsh(file transcript) (Session, bool) {
	session := Session{
		Agent:      dshAgent,
		Path:       file.path,
		LastTime:   file.info.ModTime().Local().Format(time.RFC3339),
		Historical: true,
	}
	usage := newUsageReader()

	source, closer, err := openDshTranscript(file.path)
	if err != nil {
		return Session{}, false
	}
	defer closer()

	// A session being written right now ends in a frame the encoder has not
	// closed, which is what the harness's own reader expects to find. The
	// records read before it are kept rather than thrown away with the error.
	_, _ = eachLineIn(source, func(data []byte) {
		var entry dshLine
		if err := json.Unmarshal(data, &entry); err != nil {
			return
		}

		switch entry.Type {
		case "session":
			session.SessionKey = entry.Id
			session.Cwd = entry.Cwd
			if when := dshHeaderTime(entry.CreatedAt); when != "" {
				session.FirstTime = when
			}
			return
		case "user/message", "assistant/message":
			session.RecordCount++
		default:
			return
		}

		if when := dshEventTime(entry.Time); when != "" {
			if session.FirstTime == "" {
				session.FirstTime = when
			}
			session.LastTime = when
		}
		if entry.Type == "user/message" {
			if session.Title == "" {
				session.Title = usableTitle(dshContent(entry.Data))
			}
			return
		}
		addDshUsage(usage, entry)
	})

	if session.SessionKey == "" {
		return Session{}, false
	}
	if session.FirstTime == "" {
		session.FirstTime = session.LastTime
	}
	session.Usage = usage.buckets(dayOf(session.LastTime))
	return session, true
}

// openDshTranscript reads the log as it was written. Zstandard frames are
// decoded on the way through, so a truncated tail ends the read with what came
// before it rather than failing the session.
func openDshTranscript(path string) (io.Reader, func(), error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	if !strings.HasSuffix(path, dshCompressedSuffix) {
		return handle, func() { handle.Close() }, nil
	}

	decoder, err := zstd.NewReader(handle)
	if err != nil {
		handle.Close()
		return nil, nil, err
	}
	return decoder.IOReadCloser(), func() {
		decoder.Close()
		handle.Close()
	}, nil
}

// dshContent is the model-facing blocks of one message, which is what the event
// data is for a user turn.
func dshContent(data json.RawMessage) json.RawMessage {
	var message struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &message); err != nil {
		return nil
	}
	return message.Content
}

// addDshUsage reads what one model call spent. The counts ride the assistant
// message the call produced, and the model that produced it is named there too.
func addDshUsage(reader *usageReader, entry dshLine) {
	var event struct {
		Message struct {
			Id     string `json:"id"`
			Source struct {
				Provider string `json:"provider"`
				Model    string `json:"model"`
			} `json:"source"`
		} `json:"message"`
		Usage *struct {
			InputTokens      int `json:"inputTokens"`
			OutputTokens     int `json:"outputTokens"`
			CacheReadTokens  int `json:"cacheReadTokens"`
			CacheWriteTokens int `json:"cacheWriteTokens"`
			ReasoningTokens  int `json:"reasoningTokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(entry.Data, &event); err != nil || event.Usage == nil {
		return
	}

	spent := event.Usage
	if spent.InputTokens == 0 && spent.OutputTokens == 0 && spent.CacheReadTokens == 0 && spent.CacheWriteTokens == 0 {
		return
	}
	model := event.Message.Source.Model
	if provider := event.Message.Source.Provider; provider != "" && model != "" {
		model = provider + "/" + model
	}
	reader.put(event.Message.Id, turn{
		model:      model,
		day:        dayOf(dshEventTime(entry.Time)),
		prompt:     spent.InputTokens,
		completion: spent.OutputTokens,
		cacheRead:  spent.CacheReadTokens,
		cacheWrite: spent.CacheWriteTokens,
		reasoning:  spent.ReasoningTokens,
	})
}

// dshEventTime reads the epoch milliseconds an event is stamped with.
func dshEventTime(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.UnixMilli(value).Local().Format(time.RFC3339)
}

// dshHeaderTime reads when the session opened, which the header spells either
// as epoch milliseconds or as a timestamp.
func dshHeaderTime(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	var milliseconds int64
	if err := json.Unmarshal(value, &milliseconds); err == nil {
		return dshEventTime(milliseconds)
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return strings.TrimSpace(text)
	}
	return ""
}
