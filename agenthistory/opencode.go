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
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // read another agent's session database
)

// opencodeAgent is the id every opencode session is listed under. The CLI and
// the desktop app share one database, as they share one configuration.
const opencodeAgent = "opencode"

// opencodeSessions bounds a read the same way maxTranscripts bounds a walk.
const opencodeSessions = maxTranscripts

// scanOpencode reads the sessions opencode keeps in its own database. It is
// opened read-only and never written: this is somebody else's file, and it may
// be open in an agent that is running right now.
func scanOpencode(home string) []Session {
	path := filepath.Join(home, ".local", "share", "opencode", "opencode.db")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return nil
	}
	defer db.Close()

	sessions := readOpencodeSessions(db, path)
	if len(sessions) == 0 {
		return nil
	}
	counts := countOpencodeMessages(db)
	for i := range sessions {
		sessions[i].RecordCount = counts[sessions[i].SessionKey].messages
		for bucket := range sessions[i].Usage {
			sessions[i].Usage[bucket].Requests = counts[sessions[i].SessionKey].requests
		}
	}
	return sessions
}

func readOpencodeSessions(db *sql.DB, path string) []Session {
	// The token and model columns arrived in a later opencode; a database
	// written before that answers the shorter query.
	rows, err := db.Query(`SELECT id, directory, title, time_created, time_updated,
		model, tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write
		FROM session ORDER BY time_updated DESC LIMIT ?`, opencodeSessions)
	if err != nil {
		return readOldOpencodeSessions(db, path)
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var id, directory, title string
		var created, updated int64
		var model sql.NullString
		var input, output, reasoning, cacheRead, cacheWrite int64
		if err := rows.Scan(&id, &directory, &title, &created, &updated,
			&model, &input, &output, &reasoning, &cacheRead, &cacheWrite); err != nil {
			return nil
		}

		session := newOpencodeSession(id, directory, title, created, updated, path)
		if input+output+reasoning+cacheRead+cacheWrite > 0 {
			session.Usage = []UsageBucket{{
				Model:            opencodeModel(model.String),
				Day:              dayOf(session.LastTime),
				PromptTokens:     int(input),
				CompletionTokens: int(output),
				ReasoningTokens:  int(reasoning),
				CacheReadTokens:  int(cacheRead),
				CacheWriteTokens: int(cacheWrite),
			}}
		}
		sessions = append(sessions, session)
	}
	return sessions
}

func readOldOpencodeSessions(db *sql.DB, path string) []Session {
	rows, err := db.Query(`SELECT id, directory, title, time_created, time_updated
		FROM session ORDER BY time_updated DESC LIMIT ?`, opencodeSessions)
	if err != nil {
		return nil
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var id, directory, title string
		var created, updated int64
		if err := rows.Scan(&id, &directory, &title, &created, &updated); err != nil {
			return nil
		}
		sessions = append(sessions, newOpencodeSession(id, directory, title, created, updated, path))
	}
	return sessions
}

func newOpencodeSession(id, directory, title string, created, updated int64, path string) Session {
	return Session{
		Agent:      opencodeAgent,
		SessionKey: id,
		Title:      trimTitle(title),
		Cwd:        directory,
		Path:       path,
		FirstTime:  opencodeTime(created),
		LastTime:   opencodeTime(updated),
		Historical: true,
	}
}

type opencodeCounts struct {
	messages int
	requests int
}

// countOpencodeMessages reads the whole message table once rather than once per
// session: a busy database holds thousands of rows across a few hundred
// sessions, and the same walk answers both counts.
func countOpencodeMessages(db *sql.DB) map[string]opencodeCounts {
	counts := map[string]opencodeCounts{}
	rows, err := db.Query(`SELECT session_id, data FROM message`)
	if err != nil {
		return counts
	}
	defer rows.Close()

	for rows.Next() {
		var sessionId, data string
		if err := rows.Scan(&sessionId, &data); err != nil {
			return counts
		}
		count := counts[sessionId]
		count.messages++
		// A request is one call to the model, which is what an assistant
		// message is the answer to.
		var message struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal([]byte(data), &message); err == nil && message.Role == "assistant" {
			count.requests++
		}
		counts[sessionId] = count
	}
	return counts
}

// opencodeModel names the model a session ran on. The column holds the whole
// reference opencode resolved, which is the provider and the model in it.
func opencodeModel(value string) string {
	var model struct {
		Id         string `json:"id"`
		ProviderId string `json:"providerID"`
	}
	if err := json.Unmarshal([]byte(value), &model); err != nil || model.Id == "" {
		return strings.TrimSpace(value)
	}
	if model.ProviderId == "" {
		return model.Id
	}
	return model.ProviderId + "/" + model.Id
}

// opencodeTime reads the epoch milliseconds the database stores.
func opencodeTime(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.UnixMilli(value).Local().Format(time.RFC3339)
}

// readOpencodeTranscript reads one session out of the database. A message is a
// row of its own, and what it holds is the parts filed under it.
func readOpencodeTranscript(session Session) (Transcript, error) {
	transcript := Transcript{Session: session, Messages: []Message{}}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(session.Path)+"?mode=ro")
	if err != nil {
		return Transcript{}, err
	}
	defer db.Close()

	parts, err := readOpencodeParts(db, session.SessionKey)
	if err != nil {
		return Transcript{}, err
	}

	rows, err := db.Query(`SELECT id, time_created, data FROM message
		WHERE session_id = ? ORDER BY time_created, id LIMIT ?`, session.SessionKey, maxMessages+1)
	if err != nil {
		return Transcript{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, data string
		var created int64
		if err := rows.Scan(&id, &created, &data); err != nil {
			return Transcript{}, err
		}
		if len(transcript.Messages) >= maxMessages {
			transcript.Truncated = true
			break
		}

		var message struct {
			Role string `json:"role"`
		}
		json.Unmarshal([]byte(data), &message)
		transcript.Messages = append(transcript.Messages, Message{
			Role:   message.Role,
			Time:   opencodeTime(created),
			Blocks: parts[id],
		})
	}
	return transcript, nil
}

// opencodePart is one part of a message: the prose, the model thinking aloud,
// or a tool call with what it returned.
type opencodePart struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Tool  string `json:"tool"`
	State struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
		Error  string          `json:"error"`
	} `json:"state"`
	Filename string `json:"filename"`
	Mime     string `json:"mime"`
}

// readOpencodeParts reads every part of a session at once, keyed by the message
// it belongs to: one query rather than one per message.
func readOpencodeParts(db *sql.DB, sessionKey string) (map[string][]Block, error) {
	rows, err := db.Query(`SELECT message_id, data FROM part
		WHERE session_id = ? ORDER BY time_created, id`, sessionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	blocks := map[string][]Block{}
	for rows.Next() {
		var messageId, data string
		if err := rows.Scan(&messageId, &data); err != nil {
			return nil, err
		}
		var part opencodePart
		if err := json.Unmarshal([]byte(data), &part); err != nil {
			continue
		}
		blocks[messageId] = append(blocks[messageId], opencodeBlocks(part)...)
	}
	return blocks, nil
}

func opencodeBlocks(part opencodePart) []Block {
	switch part.Type {
	case "text":
		if strings.TrimSpace(part.Text) == "" {
			return nil
		}
		return []Block{{Kind: blockText, Text: clip(part.Text, maxTextRunes)}}
	case "reasoning":
		return []Block{{Kind: blockThinking, Text: clip(part.Text, maxTextRunes)}}
	case "file":
		if strings.HasPrefix(part.Mime, "image/") {
			return []Block{{Kind: blockImage}}
		}
		return []Block{{Kind: blockText, Text: part.Filename}}
	case "tool":
		blocks := []Block{{Kind: blockToolUse, Tool: part.Tool, Text: clip(string(part.State.Input), maxToolRunes)}}
		// A call still running has neither, and stands as the call alone.
		if output := firstNonEmpty(part.State.Error, part.State.Output); output != "" {
			blocks = append(blocks, Block{
				Kind:    blockToolResult,
				Text:    clip(output, maxToolRunes),
				IsError: part.State.Status == "error",
			})
		}
		return blocks
	}
	// step-start, step-finish and the snapshots around them are bookkeeping.
	return nil
}
