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
