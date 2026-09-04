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

// Package agenthistory reads the conversation transcripts agents already keep
// on disk. Monitoring only ever sees what happens after it is turned on, while
// these files are the sessions that already happened, so the Sessions page has
// something to show on the first day.
package agenthistory

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Session is one transcript, described without its contents.
type Session struct {
	Agent       string `json:"agent"`
	SessionKey  string `json:"sessionKey"`
	Title       string `json:"title"`
	RecordCount int    `json:"recordCount"`
	FirstTime   string `json:"firstTime"`
	LastTime    string `json:"lastTime"`
	// Path is where the transcript is, so the page can say where it came from.
	Path string `json:"path"`
	// Cwd is the directory the agent worked in, when the transcript records one.
	Cwd string `json:"cwd"`
	// Historical marks a session read off disk rather than collected by monitoring.
	Historical bool `json:"historical"`
	// Usage is what the transcript says the session spent. LLM Records only ever
	// sees what went through Gateway, so this is the only account of an agent
	// that talks to its vendor directly.
	Usage []UsageBucket `json:"usage"`
}

const (
	// maxTranscripts bounds a scan: a heavy user has thousands of these, and the
	// newest are the ones worth reading.
	maxTranscripts = 500
	// maxLineBytes is the longest line eachLine reads. A single tool result can
	// be megabytes, and one of those is passed over rather than held in memory.
	maxLineBytes = 1 << 20
	// readBufferBytes is how much of a line is read at a time.
	readBufferBytes = 64 << 10
	// maxTitleRunes keeps a title to something a table cell can show.
	maxTitleRunes = 120
)

// transcriptDirs are where each agent keeps its own transcripts, relative to a
// home directory. Both write one JSONL file per session.
var transcriptDirs = []struct {
	agent string
	parts []string
	// keyOf reads the session id out of a transcript's path, for an agent that
	// names the directory holding the file rather than the file itself.
	keyOf func(path string) string
}{
	{agent: "claude-code", parts: []string{".claude", "projects"}},
	{agent: "codex-cli", parts: []string{".codex", "sessions"}},
	// The Gemini CLI keeps one directory per project hash, and OpenClaw one per
	// agent id; both hold the sessions a level or two further down, which is
	// where the walk finds them.
	{agent: "gemini-cli", parts: []string{".gemini", "tmp"}},
	{agent: "openclaw", parts: []string{".openclaw", "agents"}},
	// Claude Desktop writes one directory per Cowork session, named by the
	// session id, with the transcript inside it under a fixed name.
	{agent: "claude-desktop", parts: []string{"AppData", "Roaming", "Claude", "local-agent-mode-sessions"}, keyOf: parentDirName},
	{agent: "claude-desktop", parts: []string{"AppData", "Local", "Claude-3p", "local-agent-mode-sessions"}, keyOf: parentDirName},
	{agent: "claude-desktop", parts: []string{"AppData", "Local", "Packages", "Claude_pzs8sxrjxfjjc", "LocalCache", "Roaming", "Claude", "local-agent-mode-sessions"}, keyOf: parentDirName},
}

type cacheKey struct {
	path string
	size int64
	unix int64
}

var (
	cacheMutex sync.Mutex
	cache      = map[cacheKey]Session{}
	scans      = map[string]scanResult{}
)

// scanTTL is how long a whole scan stands. The Sessions page polls for live
// records, and walking thousands of files at that rate would be the most
// expensive thing Gateway does.
const scanTTL = 30 * time.Second

type scanResult struct {
	sessions []Session
	at       time.Time
}

// Scan reads the transcripts under one home directory. Results are cached per
// file until it changes, and the whole walk is cached briefly, so polling the
// page re-reads nothing.
func Scan(home string) []Session {
	cacheMutex.Lock()
	if previous, found := scans[home]; found && time.Since(previous.at) < scanTTL {
		cacheMutex.Unlock()
		return previous.sessions
	}
	cacheMutex.Unlock()

	sessions := scan(home)

	cacheMutex.Lock()
	scans[home] = scanResult{sessions: sessions, at: time.Now()}
	cacheMutex.Unlock()
	return sessions
}

func scan(home string) []Session {
	sessions := []Session{}
	for _, source := range transcriptDirs {
		root := filepath.Join(append([]string{home}, source.parts...)...)
		agent, keyOf := source.agent, source.keyOf
		for _, file := range newestTranscripts(root, isJSONL) {
			if session, ok := read(file, func(file transcript) (Session, bool) {
				return parse(agent, file, keyOf)
			}); ok {
				sessions = append(sessions, session)
			}
		}
	}
	// opencode keeps its sessions in a database rather than a transcript file,
	// and already totals what each one spent. dsh writes a transcript, but a
	// compressed one its own reader has to open.
	sessions = append(sessions, scanOpencode(home)...)
	sessions = append(sessions, scanDsh(home)...)
	sessions = append(sessions, scanCursor(home)...)

	sort.SliceStable(sessions, func(left, right int) bool {
		return sessions[left].LastTime > sessions[right].LastTime
	})
	return sessions
}

type transcript struct {
	path string
	info os.FileInfo
}

// isJSONL is the transcript name rule of an agent that writes plain JSONL.
func isJSONL(name string) bool {
	return strings.HasSuffix(name, ".jsonl")
}

// newestTranscripts finds the transcripts under root, newest first and capped.
func newestTranscripts(root string, match func(name string) bool) []transcript {
	found := []transcript{}
	// A missing directory means the agent was never installed here, which is not
	// an error worth reporting from a page that lists whatever it finds.
	filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !match(entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		found = append(found, transcript{path: path, info: info})
		return nil
	})

	sort.SliceStable(found, func(left, right int) bool {
		return found[left].info.ModTime().After(found[right].info.ModTime())
	})
	if len(found) > maxTranscripts {
		found = found[:maxTranscripts]
	}
	return found
}

// read is one transcript, from the cache when the file has not changed since it
// was last parsed.
func read(file transcript, parse func(transcript) (Session, bool)) (Session, bool) {
	key := cacheKey{path: file.path, size: file.info.Size(), unix: file.info.ModTime().UnixNano()}
	cacheMutex.Lock()
	cached, found := cache[key]
	cacheMutex.Unlock()
	if found {
		return cached, true
	}

	session, ok := parse(file)
	if !ok {
		return Session{}, false
	}

	cacheMutex.Lock()
	cache[key] = session
	cacheMutex.Unlock()
	return session, true
}

// line is the union of the two formats: Claude Code writes the session id and
// the message at the top level, Codex wraps everything in "payload".
type line struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionId string          `json:"sessionId"`
	RequestId string          `json:"requestId"`
	Cwd       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
	// Role and Content are how the Cowork audit log and the Gemini CLI spell a
	// message: the line is the message rather than carrying one. AuditTimestamp
	// and StartTime are the same two files' names for when it happened.
	Role           string          `json:"role"`
	Content        json.RawMessage `json:"content"`
	AuditTimestamp string          `json:"_audit_timestamp"`
	StartTime      string          `json:"startTime"`
	Model          string          `json:"model"`
	// Directories are the roots a Gemini CLI session was started across; the
	// first is the one it worked in.
	Directories []string `json:"directories"`
	// Tokens is what a Gemini CLI turn spent, on the turn itself.
	Tokens *struct {
		Input    int `json:"input"`
		Output   int `json:"output"`
		Cached   int `json:"cached"`
		Thoughts int `json:"thoughts"`
	} `json:"tokens"`
	Payload struct {
		Id      string          `json:"id"`
		Type    string          `json:"type"`
		Role    string          `json:"role"`
		Cwd     string          `json:"cwd"`
		Content json.RawMessage `json:"content"`
		// Model names what a Codex turn runs on, and Info carries the token
		// counts of the turn that just ended.
		Model string          `json:"model"`
		Info  json.RawMessage `json:"info"`
	} `json:"payload"`
}

// eachLine visits every line of a JSONL transcript and reports how many were
// passed over for being longer than maxLineBytes. bufio.Scanner is not used
// here because it ends the whole read at the first such line, which would turn
// one huge tool result into a session that looks four messages long.
//
// The bytes handed to visit are reused for the next line, so a caller that
// keeps them has to copy them first.
func eachLine(path string, visit func(data []byte)) (int, error) {
	handle, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer handle.Close()

	return eachLineIn(handle, visit)
}

// eachLineIn is eachLine over an open stream, for a transcript that is decoded
// on the way in rather than read from the file as it lies.
func eachLineIn(source io.Reader, visit func(data []byte)) (int, error) {
	reader := bufio.NewReaderSize(source, readBufferBytes)
	buffer := []byte{}
	skipping := false
	skipped := 0
	for {
		chunk, err := reader.ReadSlice('\n')
		partial := errors.Is(err, bufio.ErrBufferFull)
		if err != nil && !partial && !errors.Is(err, io.EOF) {
			return skipped, err
		}

		if !skipping {
			buffer = append(buffer, chunk...)
			if len(buffer) > maxLineBytes {
				buffer, skipping = buffer[:0], true
				skipped++
			}
		}

		if partial {
			continue
		}

		if !skipping && len(buffer) > 0 {
			visit(buffer)
		}
		buffer, skipping = buffer[:0], false

		if errors.Is(err, io.EOF) {
			return skipped, nil
		}
	}
}

func parse(agent string, file transcript, keyOf func(string) string) (Session, bool) {
	session := Session{
		Agent:      agent,
		Path:       file.path,
		LastTime:   file.info.ModTime().Local().Format(time.RFC3339),
		Historical: true,
	}
	// The file name is the session id for Claude Code, and carries it for Codex;
	// the transcript itself overrides this when it names one.
	session.SessionKey = sessionKeyFromName(file.info.Name())
	if keyOf != nil {
		session.SessionKey = keyOf(file.path)
	}

	// Reading the file is the expensive part, so what it spent is added up on
	// the same pass rather than by walking every transcript a second time.
	usage := newUsageReader()

	if _, err := eachLine(file.path, func(data []byte) {
		var entry line
		if err := json.Unmarshal(data, &entry); err != nil {
			return
		}

		if entry.SessionId != "" {
			session.SessionKey = entry.SessionId
		}
		if entry.Type == "session_meta" && entry.Payload.Id != "" {
			session.SessionKey = entry.Payload.Id
		}
		if session.Cwd == "" {
			session.Cwd = firstNonEmpty(entry.Cwd, entry.Payload.Cwd)
			if session.Cwd == "" && len(entry.Directories) > 0 {
				session.Cwd = entry.Directories[0]
			}
		}
		if when := strings.TrimSpace(firstNonEmpty(entry.Timestamp, entry.AuditTimestamp, entry.StartTime)); when != "" {
			if session.FirstTime == "" {
				session.FirstTime = when
			}
			session.LastTime = when
		}

		usage.add(entry)

		if role, content := roleAndContent(entry); role != "" {
			session.RecordCount++
			if session.Title == "" && role == "user" {
				session.Title = usableTitle(content)
			}
		}
	}); err != nil {
		return Session{}, false
	}

	if session.SessionKey == "" {
		return Session{}, false
	}
	if session.FirstTime == "" {
		session.FirstTime = session.LastTime
	}
	session.Usage = usage.buckets(dayOf(session.LastTime))
	return session, true
}

// roleAndContent picks the messages out of a transcript, in either format. Only
// the conversation counts: the tool calls and events around it are what
// monitoring is for.
func roleAndContent(entry line) (string, json.RawMessage) {
	// The Gemini CLI calls the model's turn by its own name.
	if entry.Type == "gemini" {
		return "assistant", entry.Content
	}

	if entry.Type == "user" || entry.Type == "assistant" {
		var message struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(entry.Message, &message); err != nil || len(message.Content) == 0 {
			// The Cowork audit log and the Gemini CLI put the content on the
			// line itself rather than under a message.
			return entry.Type, entry.Content
		}
		return firstNonEmpty(message.Role, entry.Type), message.Content
	}

	if entry.Type == "response_item" && entry.Payload.Type == "message" {
		// A Codex transcript replays the whole prompt, including the instructions
		// the agent sends itself. Those are not part of the conversation.
		if entry.Payload.Role == "developer" || entry.Payload.Role == "system" {
			return "", nil
		}
		return entry.Payload.Role, entry.Payload.Content
	}

	// A line that names a role and carries content is the message itself,
	// which is how the transcripts that declare no type spell one.
	if entry.Role != "" && len(entry.Content) > 0 {
		return entry.Role, entry.Content
	}
	return "", nil
}

// preambles start the context an agent injects into the first user turn. A
// session is remembered by what was asked, so a turn opening with one of these
// is passed over and the next one is tried.
var preambles = []string{
	"<",
	"Here is a list of plugins",
	"Caveat:",
	"This session is being continued",
	"<system-reminder>",
	"<environment_details>",
}

// usableTitle returns the message only when it reads like something a person
// typed, so that the injected context does not become every session's name.
func usableTitle(content json.RawMessage) string {
	text := title(content)
	for _, preamble := range preambles {
		if strings.HasPrefix(text, preamble) {
			return ""
		}
	}
	return text
}

// title turns a message body into one line. Both formats allow a plain string
// or a list of typed parts, and only the text parts are worth showing.
func title(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		return trimTitle(text)
	}

	type part struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	var parts []part
	if err := json.Unmarshal(content, &parts); err != nil {
		// A Gemini CLI message may be one part rather than a list of them.
		var single part
		if err := json.Unmarshal(content, &single); err != nil {
			return ""
		}
		parts = []part{single}
	}
	for _, part := range parts {
		if strings.TrimSpace(part.Text) != "" {
			return trimTitle(part.Text)
		}
	}
	return ""
}

func trimTitle(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r", " "), "\n", " "))
	// A pasted file or a command wrapper starts with a tag the reader does not
	// need, and it would fill the whole line.
	if strings.HasPrefix(text, "<") {
		if end := strings.Index(text, ">"); end > 0 && end < len(text)-1 {
			text = strings.TrimSpace(text[end+1:])
		}
	}

	runes := []rune(text)
	if len(runes) > maxTitleRunes {
		return string(runes[:maxTitleRunes]) + "..."
	}
	return string(runes)
}

// sessionKeyFromName reads the id out of a transcript's file name:
// "<uuid>.jsonl" for Claude Code, "rollout-<date>-<uuid>.jsonl" for Codex.
func sessionKeyFromName(name string) string {
	base := strings.TrimSuffix(name, ".jsonl")
	const uuidLength = 36
	if len(base) >= uuidLength {
		if candidate := base[len(base)-uuidLength:]; strings.Count(candidate, "-") == 4 {
			return candidate
		}
	}
	return base
}

// parentDirName is the name of the directory a transcript sits in, which is the
// session id for an agent that writes every session's file under a fixed name.
func parentDirName(path string) string {
	return filepath.Base(filepath.Dir(path))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
