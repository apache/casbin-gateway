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

const cursorAgent = "cursor"

// scanCursor reads the chats Cursor keeps in the editor's own state database.
// A chat is a "composer" there: one row of metadata, and one row per message
// under a key of its own.
func scanCursor(home string) []Session {
	path := cursorStatePath(home)
	if path == "" {
		return nil
	}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return nil
	}
	defer db.Close()

	// The rows are keyed rather than columned, so the ordering and the filter
	// read the JSON: there are a few hundred chats beside the tens of thousands
	// of message rows the key prefix keeps this off. Empty chats are dropped
	// here rather than after reading, so the limit counts sessions somebody had
	// rather than boxes they opened.
	rows, err := db.Query(`SELECT key, value FROM cursorDiskKV WHERE key LIKE 'composerData:%'
		AND json_array_length(json_extract(value, '$.fullConversationHeadersOnly')) > 0
		ORDER BY COALESCE(json_extract(value, '$.lastUpdatedAt'), json_extract(value, '$.createdAt')) DESC
		LIMIT ?`, maxTranscripts)
	if err != nil {
		return nil
	}
	defer rows.Close()

	sessions := []Session{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return sessions
		}
		if session, ok := parseCursorComposer(key, value, path); ok {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

// cursorComposer is one chat as the editor stores it. The messages themselves
// are rows of their own; the headers here are what says how many there are and
// who sent each.
type cursorComposer struct {
	ComposerId    string `json:"composerId"`
	Name          string `json:"name"`
	CreatedAt     int64  `json:"createdAt"`
	LastUpdatedAt int64  `json:"lastUpdatedAt"`
	Headers       []struct {
		BubbleId string `json:"bubbleId"`
		Type     int    `json:"type"`
	} `json:"fullConversationHeadersOnly"`
}

func parseCursorComposer(key, value, path string) (Session, bool) {
	var composer cursorComposer
	if err := json.Unmarshal([]byte(value), &composer); err != nil {
		return Session{}, false
	}
	// A composer with no messages is a chat box that was opened and left, of
	// which the editor keeps a great many.
	if len(composer.Headers) == 0 {
		return Session{}, false
	}

	id := composer.ComposerId
	if id == "" {
		id = key[len("composerData:"):]
	}
	session := Session{
		Agent:       cursorAgent,
		SessionKey:  id,
		Title:       trimTitle(composer.Name),
		RecordCount: len(composer.Headers),
		FirstTime:   cursorTime(composer.CreatedAt),
		LastTime:    cursorTime(composer.LastUpdatedAt),
		Path:        path,
		Historical:  true,
	}
	if session.LastTime == "" {
		session.LastTime = session.FirstTime
	}
	if session.FirstTime == "" {
		session.FirstTime = session.LastTime
	}
	// Cursor bills its own subscription and records no token counts to read:
	// the message rows carry the fields, always at zero. A session with no
	// usage is listed for what it was, without inventing an amount for it.
	return session, session.SessionKey != ""
}

// cursorStatePath is the editor's global state database, in the application
// directory each platform keeps it under.
func cursorStatePath(home string) string {
	candidates := [][]string{
		{"AppData", "Roaming", "Cursor", "User", "globalStorage", "state.vscdb"},
		{"Library", "Application Support", "Cursor", "User", "globalStorage", "state.vscdb"},
		{".config", "Cursor", "User", "globalStorage", "state.vscdb"},
	}
	for _, parts := range candidates {
		path := filepath.Join(append([]string{home}, parts...)...)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

// cursorTime reads the epoch milliseconds the editor stores.
func cursorTime(value int64) string {
	if value <= 0 {
		return ""
	}
	return time.UnixMilli(value).Local().Format(time.RFC3339)
}

// readCursorTranscript reads one chat out of the state database. The chat row
// holds only the order of the messages; each one is a row of its own.
func readCursorTranscript(session Session) (Transcript, error) {
	transcript := Transcript{Session: session, Messages: []Message{}}

	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(session.Path)+"?mode=ro")
	if err != nil {
		return Transcript{}, err
	}
	defer db.Close()

	// A row the editor emptied rather than deleted holds a NULL, which is not a
	// string and would end the read.
	var value sql.NullString
	if err := db.QueryRow(`SELECT value FROM cursorDiskKV WHERE key = ?`,
		"composerData:"+session.SessionKey).Scan(&value); err != nil {
		return Transcript{}, err
	}
	var composer cursorComposer
	if err := json.Unmarshal([]byte(value.String), &composer); err != nil {
		return Transcript{}, err
	}

	bubbles, err := readCursorBubbles(db, session.SessionKey)
	if err != nil {
		return Transcript{}, err
	}

	for _, header := range composer.Headers {
		bubble, found := bubbles[header.BubbleId]
		if !found {
			continue
		}
		if len(transcript.Messages) >= maxMessages {
			transcript.Truncated = true
			break
		}

		kind := header.Type
		if kind == 0 {
			kind = bubble.Type
		}
		role := "assistant"
		if kind == cursorUserBubble {
			role = "user"
		}
		transcript.Messages = append(transcript.Messages, Message{Role: role, Blocks: cursorBlocks(bubble)})
	}
	return transcript, nil
}

// cursorUserBubble is the message kind the editor gives what a person typed.
const cursorUserBubble = 1

// cursorBubble is one message: the prose, what the model thought before it, and
// the tool call it made, all optional.
type cursorBubble struct {
	Type     int    `json:"type"`
	Text     string `json:"text"`
	RichText string `json:"richText"`
	Thinking struct {
		Text string `json:"text"`
	} `json:"thinking"`
	Tool *struct {
		Name    string `json:"name"`
		Params  string `json:"params"`
		RawArgs string `json:"rawArgs"`
		Result  string `json:"result"`
		Error   string `json:"error"`
		Status  string `json:"status"`
	} `json:"toolFormerData"`
}

// readCursorBubbles reads every message of one chat at once: they are keyed by
// the chat they belong to, and there is no other way to find them.
func readCursorBubbles(db *sql.DB, composerId string) (map[string]cursorBubble, error) {
	rows, err := db.Query(`SELECT key, value FROM cursorDiskKV WHERE key LIKE ?`, "bubbleId:"+composerId+":%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bubbles := map[string]cursorBubble{}
	for rows.Next() {
		var key string
		var value sql.NullString
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		var bubble cursorBubble
		if err := json.Unmarshal([]byte(value.String), &bubble); err != nil {
			continue
		}
		bubbles[key[strings.LastIndex(key, ":")+1:]] = bubble
	}
	return bubbles, nil
}

func cursorBlocks(bubble cursorBubble) []Block {
	blocks := []Block{}
	if thinking := strings.TrimSpace(bubble.Thinking.Text); thinking != "" {
		blocks = append(blocks, Block{Kind: blockThinking, Text: clip(thinking, maxTextRunes)})
	}
	// What a person typed is kept as an editor document, and only the older
	// messages carry the same words as plain text.
	if text := firstNonEmpty(bubble.Text, cursorRichText(bubble.RichText)); strings.TrimSpace(text) != "" {
		blocks = append(blocks, Block{Kind: blockText, Text: clip(text, maxTextRunes)})
	}
	if call := bubble.Tool; call != nil {
		blocks = append(blocks, Block{
			Kind: blockToolUse,
			Tool: call.Name,
			Text: clip(firstNonEmpty(call.Params, call.RawArgs), maxToolRunes),
		})
		if output := firstNonEmpty(call.Error, call.Result); output != "" {
			blocks = append(blocks, Block{
				Kind:    blockToolResult,
				Text:    clip(output, maxToolRunes),
				IsError: call.Status == "error",
			})
		}
	}
	return blocks
}

// cursorRichText reads the words out of the editor document a prompt is stored
// as: a tree of nodes, of which only the text ones are the message.
func cursorRichText(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	var document any
	if err := json.Unmarshal([]byte(value), &document); err != nil {
		return ""
	}

	pieces := []string{}
	var walk func(node any)
	walk = func(node any) {
		switch value := node.(type) {
		case map[string]any:
			if text, ok := value["text"].(string); ok && text != "" {
				pieces = append(pieces, text)
			}
			if children, ok := value["content"]; ok {
				walk(children)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(document)
	return strings.Join(pieces, "\n")
}
