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

//go:build windows

package agentmonitor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/apache/casbin-gateway/auditutil"
)

const (
	transcriptPollInterval = time.Second
	maxTranscriptLine      = 8 * 1024 * 1024
)

type transcriptRoot struct {
	path   string
	target monitorTarget
}

type pendingToolCall struct {
	target    monitorTarget
	started   time.Time
	session   string
	title     string
	id        string
	name      string
	model     string
	mcpServer string
	mcpTool   string
}

func (call pendingToolCall) record(when time.Time, action, outcome string) *Record {
	record := &Record{
		CreatedTime: when.Format(time.RFC3339Nano),
		Agent:       claudeDesktopAgentID,
		AgentPath:   call.target.Path,
		User:        call.target.Owner,
		EventType:   "tool",
		Action:      action,
		Outcome:     outcome,
		SessionKey:  call.session,
		Title:       call.title,
		ToolUseId:   call.id,
		ToolName:    call.name,
		Model:       call.model,
	}
	if call.mcpServer != "" {
		record.EventType, record.McpServer, record.McpTool = "mcp", call.mcpServer, call.mcpTool
	}
	if action == "result" {
		record.DurationMs = when.Sub(call.started).Milliseconds()
		if record.DurationMs < 0 {
			record.DurationMs = 0
		}
	}
	return record
}

type transcriptMonitor struct {
	mutex              sync.Mutex
	roots              map[string]transcriptRoot
	seededRoots        map[string]bool
	offsets            map[string]int64
	pending            map[string]pendingToolCall
	metadata           coworkMetadataCache
	stop               chan struct{}
	done               chan struct{}
	existingRoots      []string
	auditFileCount     int
	lastSuccessfulPoll time.Time
	lastErr            error
}

type transcriptStatus struct {
	configuredRoots    []string
	existingRoots      []string
	auditFileCount     int
	lastSuccessfulPoll time.Time
	lastErr            error
}

func newTranscriptMonitor(targets map[string]monitorTarget) *transcriptMonitor {
	monitor := &transcriptMonitor{
		roots:       map[string]transcriptRoot{},
		seededRoots: map[string]bool{},
		offsets:     map[string]int64{},
		pending:     map[string]pendingToolCall{},
		metadata:    coworkMetadataCache{entries: map[string]coworkMetadataCacheEntry{}},
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	monitor.SetTargets(targets)
	monitor.poll()
	go monitor.run()
	return monitor
}

// SetTargets switches the Windows profiles whose Cowork audit logs are read.
// The first visible state for a newly configured root is seeded at EOF.
func (monitor *transcriptMonitor) SetTargets(targets map[string]monitorTarget) {
	currentHome, homeErr := os.UserHomeDir()
	next := map[string]transcriptRoot{}
	for _, target := range targets {
		profile := currentHome
		cleanPath := filepath.Clean(target.Path)
		marker := string(os.PathSeparator) + filepath.Join("AppData", "Local", "AnthropicClaude")
		if index := strings.Index(strings.ToLower(cleanPath), strings.ToLower(marker)); index >= 0 {
			profile = cleanPath[:index]
		}
		if profile == "" {
			continue
		}
		roaming := filepath.Join(profile, "AppData", "Roaming")
		local := filepath.Join(profile, "AppData", "Local")
		if homeErr == nil && strings.EqualFold(filepath.Clean(profile), filepath.Clean(currentHome)) {
			if configured := os.Getenv("APPDATA"); configured != "" {
				roaming = configured
			}
			if configured := os.Getenv("LOCALAPPDATA"); configured != "" {
				local = configured
			}
		}
		for _, root := range []string{
			filepath.Join(roaming, "Claude", "local-agent-mode-sessions"),
			filepath.Join(local, "Claude-3p", "local-agent-mode-sessions"),
			filepath.Join(local, "Packages", "Claude_pzs8sxrjxfjjc", "LocalCache", "Roaming", "Claude", "local-agent-mode-sessions"),
		} {
			root = filepath.Clean(root)
			next[strings.ToLower(root)] = transcriptRoot{path: root, target: target}
		}
	}

	monitor.mutex.Lock()
	monitor.roots = next
	if homeErr != nil {
		monitor.lastErr = homeErr
	}
	monitor.mutex.Unlock()
}

func (monitor *transcriptMonitor) Stop() {
	close(monitor.stop)
	<-monitor.done
}

func (monitor *transcriptMonitor) Status() transcriptStatus {
	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()
	status := transcriptStatus{
		existingRoots:      append([]string(nil), monitor.existingRoots...),
		auditFileCount:     monitor.auditFileCount,
		lastSuccessfulPoll: monitor.lastSuccessfulPoll,
		lastErr:            monitor.lastErr,
	}
	for _, root := range monitor.roots {
		status.configuredRoots = append(status.configuredRoots, root.path)
	}
	sort.Strings(status.configuredRoots)
	return status
}

func (monitor *transcriptMonitor) run() {
	defer close(monitor.done)
	ticker := time.NewTicker(transcriptPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.poll()
		case <-monitor.stop:
			return
		}
	}
}

func (monitor *transcriptMonitor) poll() {
	monitor.mutex.Lock()
	defer monitor.mutex.Unlock()

	monitor.lastErr = nil
	monitor.existingRoots = nil
	monitor.auditFileCount = 0
	for rootKey, root := range monitor.roots {
		files, exists, err := auditFiles(root.path)
		if exists {
			monitor.existingRoots = append(monitor.existingRoots, root.path)
		}
		if err != nil {
			monitor.lastErr = err
			continue
		}
		if !monitor.seededRoots[rootKey] {
			for _, path := range files {
				if info, err := os.Stat(path); err == nil {
					monitor.offsets[strings.ToLower(filepath.Clean(path))] = info.Size()
				}
			}
			if exists {
				monitor.seededRoots[rootKey] = true
			}
			monitor.auditFileCount += len(files)
			continue
		}
		monitor.auditFileCount += len(files)
		for _, path := range files {
			metadata, _, _ := monitor.metadata.load(path)
			sessionKey := metadata.sessionKey(path)
			if err := monitor.consumeFile(path, root.target, sessionKey, metadata.Title); err != nil {
				monitor.lastErr = err
			}
		}
	}
	sort.Strings(monitor.existingRoots)
	if monitor.lastErr == nil {
		monitor.lastSuccessfulPoll = time.Now()
	}
}

func auditFiles(root string) ([]string, bool, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, true, errors.New("Cowork audit root is not a directory: " + root)
	}
	result := []string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.EqualFold(entry.Name(), ".claude") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(entry.Name(), "audit.jsonl") {
			result = append(result, path)
		}
		return nil
	})
	sort.Strings(result)
	return result, true, err
}

func (monitor *transcriptMonitor) consumeFile(path string, target monitorTarget, sessionKey, title string) error {
	key := strings.ToLower(filepath.Clean(path))
	offset := monitor.offsets[key]
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() < offset {
		offset = 0
		monitor.offsets[key] = 0
	}
	if info.Size() == offset {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	reader := bufio.NewReaderSize(file, 64*1024)
	var line []byte
	var consumed int64
	oversized := false
	for {
		fragment, readErr := reader.ReadSlice('\n')
		consumed += int64(len(fragment))
		if !oversized {
			if len(line)+len(fragment) <= maxTranscriptLine {
				line = append(line, fragment...)
			} else {
				line = nil
				oversized = true
			}
		}
		if errors.Is(readErr, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		offset += consumed
		monitor.offsets[key] = offset
		if !oversized {
			monitor.consumeLine(target, sessionKey, title, bytes.TrimSpace(line))
		}
		line = line[:0]
		consumed = 0
		oversized = false
	}
}

func (monitor *transcriptMonitor) consumeLine(target monitorTarget, sessionKey, title string, line []byte) {
	if len(line) == 0 {
		return
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	entry := map[string]any{}
	if decoder.Decode(&entry) != nil {
		return
	}
	message, _ := entry["message"].(map[string]any)
	content := entry["content"]
	if nested, exists := message["content"]; exists {
		content = nested
	}
	blocks := []map[string]any{}
	contentLength := 0
	switch value := content.(type) {
	case string:
		contentLength = utf8.RuneCountInString(value)
	case []any:
		for _, item := range value {
			switch item := item.(type) {
			case string:
				contentLength += utf8.RuneCountInString(item)
			case map[string]any:
				blocks = append(blocks, item)
				if transcriptField(item, "type") == "text" {
					contentLength += utf8.RuneCountInString(transcriptField(item, "text"))
				}
			}
		}
	case map[string]any:
		blocks = append(blocks, value)
		if transcriptField(value, "type") == "text" {
			contentLength = utf8.RuneCountInString(transcriptField(value, "text"))
		}
	}
	model := transcriptField(message, "model")
	if model == "" {
		model = transcriptField(entry, "model")
	}
	when := time.Now()
	if timestamp := transcriptField(entry, "timestamp", "_audit_timestamp"); timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
			when = parsed
		}
	}
	role := transcriptField(message, "role")
	if role == "" {
		role = transcriptField(entry, "role", "type")
	}
	if contentLength > 0 && (role == "user" || role == "assistant") {
		action, outcome := "request", "attempted"
		if role == "assistant" {
			action, outcome = "response", "success"
		}
		AddRecord(&Record{
			CreatedTime: when.Format(time.RFC3339Nano), Agent: claudeDesktopAgentID,
			AgentPath: target.Path, User: target.Owner, EventType: "llm", Action: action,
			Outcome: outcome, SessionKey: sessionKey, Title: title, Model: model,
			Object: auditutil.EncodeBoundedJSON(map[string]int{"contentLength": contentLength}, auditutil.MaxPayloadBytes),
		})
	}
	for _, block := range blocks {
		switch transcriptField(block, "type") {
		case "tool_use":
			name := transcriptField(block, "name")
			if name == "" {
				continue
			}
			call := pendingToolCall{target: target, started: when, session: sessionKey, title: title, id: transcriptField(block, "id"), name: name, model: model}
			if server, tool, ok := auditutil.ParseMcpTool(name, "mcp__"); ok {
				call.mcpServer, call.mcpTool = server, tool
			}
			record := call.record(when, "call", "attempted")
			if input, exists := block["input"]; exists {
				record.Object = auditutil.EncodeBoundedJSON(map[string]any{"input": auditutil.SanitizeToolInput(name, input)}, auditutil.MaxPayloadBytes)
			}
			AddRecord(record)
			if call.id != "" {
				monitor.pending[call.id] = call
			}
		case "tool_result":
			id := transcriptField(block, "tool_use_id")
			call, exists := monitor.pending[id]
			if !exists {
				continue
			}
			delete(monitor.pending, id)
			outcome := "success"
			if failed, _ := block["is_error"].(bool); failed {
				outcome = "failure"
			}
			AddRecord(call.record(when, "result", outcome))
		}
	}
}

func transcriptField(value map[string]any, names ...string) string {
	for _, name := range names {
		if text, ok := value[name].(string); ok && text != "" {
			return text
		}
	}
	return ""
}
