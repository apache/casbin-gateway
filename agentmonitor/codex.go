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

package agentmonitor

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	codexMonitorStateFile = "codex-rollout-monitor.json"
	codexPollInterval     = time.Second
	maxCodexRolloutLine   = 8 * 1024 * 1024
)

type codexClaim struct {
	AgentID   string `json:"agentId"`
	Path      string `json:"path"`
	Owner     string `json:"owner"`
	CodexHome string `json:"codexHome"`
}

type codexPendingCall struct {
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	TurnID    string    `json:"turnId,omitempty"`
	Object    string    `json:"object,omitempty"`
}

type codexCursor struct {
	Path       string                      `json:"path"`
	Root       string                      `json:"root"`
	Offset     int64                       `json:"offset"`
	SessionKey string                      `json:"sessionKey,omitempty"`
	AgentID    string                      `json:"agentId,omitempty"`
	Model      string                      `json:"model,omitempty"`
	TurnID     string                      `json:"turnId,omitempty"`
	Pending    map[string]codexPendingCall `json:"pending,omitempty"`
}

type codexSavedState struct {
	Claims  []codexClaim            `json:"claims"`
	Cursors map[string]*codexCursor `json:"cursors"`
}

type codexMonitorManager struct {
	mutex     sync.Mutex
	statePath string
	addRecord func(*Record)
	claims    map[string]codexClaim
	cursors   map[string]*codexCursor
	lastErr   map[string]error
	dirty     bool
	stop      chan struct{}
	done      chan struct{}
}

var codexMonitor = newCodexMonitorManager("", AddRecord)

func newCodexMonitorManager(statePath string, addRecord func(*Record)) *codexMonitorManager {
	return &codexMonitorManager{
		statePath: statePath,
		addRecord: addRecord,
		claims:    map[string]codexClaim{},
		cursors:   map[string]*codexCursor{},
		lastErr:   map[string]error{},
	}
}

func (manager *codexMonitorManager) start() error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.stop != nil {
		return nil
	}
	if manager.statePath == "" {
		manager.statePath = monitorStatePath(codexMonitorStateFile)
	}
	if err := manager.loadLocked(); err != nil {
		return err
	}
	manager.startPollerLocked()
	return nil
}

func (manager *codexMonitorManager) stopMonitor() {
	manager.mutex.Lock()
	stop, done := manager.stop, manager.done
	manager.stop, manager.done = nil, nil
	manager.mutex.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

// ResolveCodexHome finds the Codex state directory belonging to an installed
// Codex application. CODEX_HOME is meaningful only for the current user.
func ResolveCodexHome(agentPath, ownerName string) (string, error) {
	ownerName = strings.TrimSpace(ownerName)
	current, _ := user.Current()
	if current != nil && strings.EqualFold(accountName(ownerName), accountName(current.Username)) {
		if configured := strings.TrimSpace(os.Getenv("CODEX_HOME")); configured != "" {
			if !filepath.IsAbs(configured) {
				return "", errors.New("CODEX_HOME must be an absolute path")
			}
			return filepath.Clean(configured), nil
		}
		if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
			return filepath.Join(home, ".codex"), nil
		}
	}

	candidates := []string{ownerName}
	if index := strings.LastIndexAny(ownerName, `\\/`); index >= 0 && index+1 < len(ownerName) {
		candidates = append(candidates, ownerName[index+1:])
	}
	for _, candidate := range candidates {
		account, err := user.Lookup(candidate)
		if err == nil && filepath.IsAbs(account.HomeDir) {
			return filepath.Join(account.HomeDir, ".codex"), nil
		}
	}

	normalized := strings.ReplaceAll(filepath.Clean(agentPath), `\`, "/")
	lower := strings.ToLower(normalized)
	if index := strings.Index(lower, "/users/"); index >= 0 {
		remainder := normalized[index+len("/users/"):]
		if slash := strings.Index(remainder, "/"); slash > 0 {
			return filepath.Clean(normalized[:index+len("/users/")+slash] + "/.codex"), nil
		}
	}
	return "", fmt.Errorf("cannot resolve a home directory for owner %q", ownerName)
}

func accountName(value string) string {
	return filepath.Base(strings.ReplaceAll(strings.TrimSpace(value), `\`, "/"))
}

// EnableCodexMonitor declares one Codex installation for read-only rollout
// tailing. Existing rollout history is skipped on the first enable.
func EnableCodexMonitor(agentID, path, ownerName, codexHome string) error {
	return codexMonitor.enable(codexClaim{
		AgentID: canonicalAgentID(agentID),
		Path:    filepath.Clean(path), Owner: strings.TrimSpace(ownerName),
		CodexHome: filepath.Clean(codexHome),
	})
}

func (manager *codexMonitorManager) enable(claim codexClaim) error {
	if claim.AgentID != "codex" && claim.AgentID != "codex-cli" {
		return fmt.Errorf("unsupported Codex rollout agent %q", claim.AgentID)
	}
	if claim.Path == "" || claim.Owner == "" || !filepath.IsAbs(claim.CodexHome) {
		return errors.New("agent path, owner and an absolute CODEX_HOME are required")
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.statePath == "" {
		manager.statePath = monitorStatePath(codexMonitorStateFile)
	}
	key := codexClaimKey(claim.AgentID, claim.Path, claim.Owner)
	if _, exists := manager.claims[key]; exists {
		return nil
	}
	root := codexSessionsRoot(claim.CodexHome)
	if !manager.hasClaimForRootLocked(root) {
		if err := manager.seedRootLocked(root); err != nil {
			return err
		}
	}
	manager.claims[key] = claim
	manager.dirty = true
	if err := manager.saveLocked(); err != nil {
		delete(manager.claims, key)
		return err
	}
	return nil
}

// DisableCodexMonitor removes a Codex rollout declaration.
func DisableCodexMonitor(agentID, path, ownerName string) error {
	return codexMonitor.disable(canonicalAgentID(agentID), path, ownerName)
}

func (manager *codexMonitorManager) disable(agentID, path, ownerName string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	key := codexClaimKey(agentID, path, ownerName)
	claim, exists := manager.claims[key]
	if !exists {
		return nil
	}
	delete(manager.claims, key)
	if len(manager.claims) == 0 {
		manager.cursors = map[string]*codexCursor{}
	}
	manager.dirty = true
	if err := manager.saveLocked(); err != nil {
		manager.claims[key] = claim
		return err
	}
	return nil
}

// CodexMonitorStatus reports the declaration and local tailing state.
func CodexMonitorStatus(agentID, path, ownerName string) (bool, string) {
	return codexMonitor.status(canonicalAgentID(agentID), path, ownerName)
}

func (manager *codexMonitorManager) status(agentID, path, ownerName string) (bool, string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	claim, found := manager.claims[codexClaimKey(agentID, path, ownerName)]
	if !found {
		return false, "not patched"
	}
	root := codexSessionsRoot(claim.CodexHome)
	files, exists, err := codexRolloutFiles(root)
	if err == nil {
		err = manager.lastErr[codexPathKey(root)]
	}
	if err != nil {
		return true, "monitor error: " + err.Error()
	}
	if !exists {
		return true, "waiting for activity: sessions directory not found"
	}
	if len(files) == 0 {
		return true, "waiting for activity: no rollout files"
	}
	matchingFiles, knownFiles, unsupportedFiles := 0, 0, 0
	for _, cursor := range manager.cursors {
		if codexPathKey(cursor.Root) != codexPathKey(root) {
			continue
		}
		if cursor.AgentID == "" {
			unsupportedFiles++
			continue
		}
		knownFiles++
		if cursor.AgentID == claim.AgentID {
			matchingFiles++
		}
	}
	if knownFiles == 0 && unsupportedFiles > 0 {
		return true, fmt.Sprintf("unsupported source: %d rollout file(s) did not identify as Codex", unsupportedFiles)
	}
	if matchingFiles == 0 {
		return true, "waiting for activity: no matching source"
	}
	return true, fmt.Sprintf("active: monitoring %d rollout file(s)", matchingFiles)
}

func (manager *codexMonitorManager) startPollerLocked() {
	stop := make(chan struct{})
	done := make(chan struct{})
	manager.stop, manager.done = stop, done
	go manager.run(stop, done)
}

func (manager *codexMonitorManager) run(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	manager.poll()
	ticker := time.NewTicker(codexPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			manager.poll()
		case <-stop:
			return
		}
	}
}

func (manager *codexMonitorManager) poll() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	roots := map[string]string{}
	for _, claim := range manager.claims {
		root := codexSessionsRoot(claim.CodexHome)
		roots[codexPathKey(root)] = root
	}
	for key, root := range roots {
		files, _, err := codexRolloutFiles(root)
		if err != nil {
			manager.lastErr[key] = err
			continue
		}
		delete(manager.lastErr, key)
		for _, path := range files {
			cursor, err := manager.cursorForFileLocked(root, path, false)
			if err != nil {
				manager.lastErr[key] = err
				continue
			}
			if err := manager.consumeFileLocked(cursor); err != nil {
				manager.lastErr[key] = err
			}
		}
	}
	if manager.dirty {
		if err := manager.saveLocked(); err != nil {
			for key := range roots {
				manager.lastErr[key] = err
			}
		}
	}
}

func (manager *codexMonitorManager) seedRootLocked(root string) error {
	files, _, err := codexRolloutFiles(root)
	if err != nil {
		return err
	}
	for _, path := range files {
		if _, err := manager.cursorForFileLocked(root, path, true); err != nil {
			return err
		}
	}
	return nil
}

func (manager *codexMonitorManager) cursorForFileLocked(root, path string, seedEOF bool) (*codexCursor, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	key := codexPathKey(path)
	cursor := manager.cursors[key]
	if cursor == nil {
		meta, err := codexFileHeader(path)
		if err != nil {
			return nil, err
		}
		cursor = &codexCursor{
			Path: path, Root: root, SessionKey: meta.SessionKey, AgentID: meta.AgentID,
			Pending: map[string]codexPendingCall{},
		}
		manager.cursors[key] = cursor
		manager.dirty = true
	}

	if seedEOF {
		cursor.Offset = info.Size()
		resetCodexCursorTurn(cursor)
		manager.dirty = true
	} else if info.Size() < cursor.Offset {
		cursor.Offset = 0
		cursor.SessionKey = ""
		cursor.AgentID = ""
		resetCodexCursorTurn(cursor)
		manager.dirty = true
	}
	if cursor.AgentID == "" && cursor.Offset == 0 && info.Size() > 0 {
		meta, err := codexFileHeader(path)
		if err != nil {
			return nil, err
		}
		cursor.SessionKey, cursor.AgentID = meta.SessionKey, meta.AgentID
	}
	return cursor, nil
}

func resetCodexCursorTurn(cursor *codexCursor) {
	cursor.Model = ""
	cursor.TurnID = ""
	cursor.Pending = map[string]codexPendingCall{}
}

func (manager *codexMonitorManager) consumeFileLocked(cursor *codexCursor) error {
	info, err := os.Stat(cursor.Path)
	if err != nil {
		return err
	}
	if info.Size() == cursor.Offset {
		return nil
	}

	file, err := os.Open(cursor.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(cursor.Offset, io.SeekStart); err != nil {
		return err
	}

	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		line, size, complete, readErr := readCompleteBoundedLine(reader, maxCodexRolloutLine)
		if readErr != nil {
			return readErr
		}
		if !complete {
			return nil
		}
		cursor.Offset += size
		manager.dirty = true
		if line != nil {
			claim := manager.claimForCursorLocked(cursor)
			for _, record := range parseCodexRolloutLine(bytes.TrimSpace(line), cursor, claim) {
				manager.addRecord(record)
			}
		}
	}
}

func (manager *codexMonitorManager) claimForCursorLocked(cursor *codexCursor) *codexClaim {
	for _, claim := range manager.claims {
		if claim.AgentID == cursor.AgentID && codexPathKey(codexSessionsRoot(claim.CodexHome)) == codexPathKey(cursor.Root) {
			return &claim
		}
	}
	return nil
}

func (manager *codexMonitorManager) hasClaimForRootLocked(root string) bool {
	for _, claim := range manager.claims {
		if codexPathKey(codexSessionsRoot(claim.CodexHome)) == codexPathKey(root) {
			return true
		}
	}
	return false
}

func codexRolloutFiles(root string) ([]string, bool, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, true, fmt.Errorf("Codex sessions root is not a directory: %s", root)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, true, err
	}

	files := []string{}
	err = filepath.WalkDir(canonicalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, true, err
}

type codexHeaderMeta struct {
	SessionKey string
	AgentID    string
}

func codexFileHeader(path string) (codexHeaderMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return codexHeaderMeta{}, err
	}
	defer file.Close()
	line, _, complete, err := readCompleteBoundedLine(bufio.NewReaderSize(file, 64*1024), maxCodexRolloutLine)
	if err != nil {
		return codexHeaderMeta{}, err
	}
	if !complete || line == nil {
		return codexHeaderMeta{}, nil
	}
	return parseCodexHeader(bytes.TrimSpace(line)), nil
}

func codexSessionsRoot(codexHome string) string {
	return filepath.Join(filepath.Clean(codexHome), "sessions")
}

func codexClaimKey(agentID, path, ownerName string) string {
	return strings.ToLower(canonicalAgentID(agentID) + "\x00" + filepath.Clean(path) + "\x00" + strings.TrimSpace(ownerName))
}

func codexPathKey(path string) string {
	key := filepath.Clean(path)
	if os.PathSeparator == '\\' {
		key = strings.ToLower(key)
	}
	return key
}

func (manager *codexMonitorManager) loadLocked() error {
	data, err := os.ReadFile(manager.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var saved codexSavedState
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("cannot parse Codex rollout monitor state: %w", err)
	}
	for _, claim := range saved.Claims {
		manager.claims[codexClaimKey(claim.AgentID, claim.Path, claim.Owner)] = claim
	}
	if saved.Cursors != nil {
		manager.cursors = saved.Cursors
	}
	for key, cursor := range manager.cursors {
		if cursor == nil {
			delete(manager.cursors, key)
			continue
		}
		if cursor.Pending == nil {
			cursor.Pending = map[string]codexPendingCall{}
		}
	}
	return nil
}

func (manager *codexMonitorManager) saveLocked() error {
	if len(manager.claims) == 0 {
		if err := os.Remove(manager.statePath); err != nil && !os.IsNotExist(err) {
			return err
		}
		manager.dirty = false
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(manager.statePath), 0o700); err != nil {
		return err
	}
	saved := codexSavedState{Claims: make([]codexClaim, 0, len(manager.claims)), Cursors: manager.cursors}
	for _, claim := range manager.claims {
		saved.Claims = append(saved.Claims, claim)
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manager.statePath, append(data, '\n'), 0o600); err != nil {
		return err
	}
	manager.dirty = false
	return nil
}
