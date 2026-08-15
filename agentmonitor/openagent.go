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
	openAgentMonitorStateFile = "openagent-audit-monitor.json"
	openAgentPollInterval     = time.Second
	maxOpenAgentAuditLine     = 8 * 1024 * 1024
	openAgentAgentID          = "openagent"
	openAgentAuditSubdir      = "audit"
)

type openAgentClaim struct {
	Path     string `json:"path"`
	Owner    string `json:"owner"`
	AuditDir string `json:"auditDir"`
}

type openAgentCursor struct {
	Path       string `json:"path"`
	Root       string `json:"root"`
	Offset     int64  `json:"offset"`
	SessionKey string `json:"sessionKey,omitempty"`
}

type openAgentSavedState struct {
	Claims  []openAgentClaim            `json:"claims"`
	Cursors map[string]*openAgentCursor `json:"cursors"`
}

type openAgentMonitorManager struct {
	mutex     sync.Mutex
	statePath string
	addRecord func(*Record)
	claims    map[string]openAgentClaim
	cursors   map[string]*openAgentCursor
	lastErr   map[string]error
	dirty     bool
	stop      chan struct{}
	done      chan struct{}
}

var openAgentMonitor = newOpenAgentMonitorManager("", AddRecord)

func newOpenAgentMonitorManager(statePath string, addRecord func(*Record)) *openAgentMonitorManager {
	return &openAgentMonitorManager{
		statePath: statePath,
		addRecord: addRecord,
		claims:    map[string]openAgentClaim{},
		cursors:   map[string]*openAgentCursor{},
		lastErr:   map[string]error{},
	}
}

func (manager *openAgentMonitorManager) start() error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.stop != nil {
		return nil
	}
	if manager.statePath == "" {
		manager.statePath = monitorStatePath(openAgentMonitorStateFile)
	}
	if err := manager.loadLocked(); err != nil {
		return err
	}
	manager.startPollerLocked()
	return nil
}

func (manager *openAgentMonitorManager) stopMonitor() {
	manager.mutex.Lock()
	stop, done := manager.stop, manager.done
	manager.stop, manager.done = nil, nil
	manager.mutex.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
}

// ResolveOpenAgentAuditDir locates the append-only audit directory beside the
// OpenAgent binary. OPENAGENT_AUDIT_DIR applies to the current process user.
func ResolveOpenAgentAuditDir(agentPath, ownerName string) (string, error) {
	ownerName = strings.TrimSpace(ownerName)
	if configured := strings.TrimSpace(os.Getenv("OPENAGENT_AUDIT_DIR")); configured != "" {
		current, _ := user.Current()
		if current != nil && strings.EqualFold(accountName(ownerName), accountName(current.Username)) {
			if !filepath.IsAbs(configured) {
				return "", errors.New("OPENAGENT_AUDIT_DIR must be an absolute path")
			}
			return filepath.Clean(configured), nil
		}
	}
	if agentPath == "" {
		return "", errors.New("agent path is required to locate the audit directory")
	}
	binary := agentPath
	if resolved, err := filepath.EvalSymlinks(agentPath); err == nil {
		binary = resolved
	}
	return filepath.Join(filepath.Dir(filepath.Clean(binary)), openAgentAuditSubdir), nil
}

// EnableOpenAgentMonitor declares one OpenAgent audit directory for read-only
// tailing. Existing history is skipped when the declaration is first created.
func EnableOpenAgentMonitor(path, ownerName, auditDir string) error {
	return openAgentMonitor.enable(openAgentClaim{Path: filepath.Clean(path), Owner: strings.TrimSpace(ownerName), AuditDir: filepath.Clean(auditDir)})
}

func (manager *openAgentMonitorManager) enable(claim openAgentClaim) error {
	if claim.Path == "" || claim.Owner == "" || !filepath.IsAbs(claim.AuditDir) {
		return errors.New("agent path, owner and an absolute audit directory are required")
	}

	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	if manager.statePath == "" {
		manager.statePath = monitorStatePath(openAgentMonitorStateFile)
	}
	key := openAgentClaimKey(claim.Path, claim.Owner)
	if _, exists := manager.claims[key]; exists {
		return nil
	}
	if !manager.hasClaimForRootLocked(claim.AuditDir) {
		if err := manager.seedRootLocked(claim.AuditDir); err != nil {
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

// DisableOpenAgentMonitor removes one OpenAgent audit declaration.
func DisableOpenAgentMonitor(path, ownerName string) error {
	return openAgentMonitor.disable(path, ownerName)
}

func (manager *openAgentMonitorManager) disable(path, ownerName string) error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	key := openAgentClaimKey(path, ownerName)
	claim, exists := manager.claims[key]
	if !exists {
		return nil
	}
	delete(manager.claims, key)
	if len(manager.claims) == 0 {
		manager.cursors = map[string]*openAgentCursor{}
	}
	manager.dirty = true
	if err := manager.saveLocked(); err != nil {
		manager.claims[key] = claim
		return err
	}
	return nil
}

// OpenAgentMonitorStatus reports the declaration and local tailing state.
func OpenAgentMonitorStatus(path, ownerName string) (bool, string) {
	return openAgentMonitor.status(path, ownerName)
}

func (manager *openAgentMonitorManager) status(path, ownerName string) (bool, string) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	claim, found := manager.claims[openAgentClaimKey(path, ownerName)]
	if !found {
		return false, "not patched"
	}
	files, exists, err := openAgentAuditFiles(claim.AuditDir)
	if err == nil {
		err = manager.lastErr[openAgentPathKey(claim.AuditDir)]
	}
	if err != nil {
		return true, "monitor error: " + err.Error()
	}
	if !exists {
		return true, "waiting for activity: audit directory not found"
	}
	if len(files) == 0 {
		return true, "waiting for activity: no audit files"
	}
	return true, fmt.Sprintf("active: monitoring %d audit file(s)", len(files))
}

func (manager *openAgentMonitorManager) startPollerLocked() {
	stop := make(chan struct{})
	done := make(chan struct{})
	manager.stop, manager.done = stop, done
	go manager.run(stop, done)
}

func (manager *openAgentMonitorManager) run(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	manager.poll()
	ticker := time.NewTicker(openAgentPollInterval)
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

func (manager *openAgentMonitorManager) poll() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	roots := map[string]string{}
	for _, claim := range manager.claims {
		roots[openAgentPathKey(claim.AuditDir)] = claim.AuditDir
	}
	for key, root := range roots {
		files, _, err := openAgentAuditFiles(root)
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

func (manager *openAgentMonitorManager) seedRootLocked(root string) error {
	files, _, err := openAgentAuditFiles(root)
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

func (manager *openAgentMonitorManager) cursorForFileLocked(root, path string, seedEOF bool) (*openAgentCursor, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	key := openAgentPathKey(path)
	cursor := manager.cursors[key]
	if cursor == nil {
		cursor = &openAgentCursor{Path: path, Root: root, SessionKey: openAgentSessionKeyFromPath(path)}
		manager.cursors[key] = cursor
		manager.dirty = true
	}
	if seedEOF {
		cursor.Offset = info.Size()
		manager.dirty = true
	} else if info.Size() < cursor.Offset {
		cursor.Offset = 0
		manager.dirty = true
	}
	return cursor, nil
}

func (manager *openAgentMonitorManager) consumeFileLocked(cursor *openAgentCursor) error {
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

	claim := manager.claimForCursorLocked(cursor)
	reader := bufio.NewReaderSize(file, 64*1024)
	for {
		line, size, complete, readErr := readCompleteBoundedLine(reader, maxOpenAgentAuditLine)
		if readErr != nil {
			return readErr
		}
		if !complete {
			return nil
		}
		cursor.Offset += size
		manager.dirty = true
		if line != nil {
			for _, record := range parseOpenAgentAuditLine(bytes.TrimSpace(line), cursor, claim) {
				manager.addRecord(record)
			}
		}
	}
}

func (manager *openAgentMonitorManager) claimForCursorLocked(cursor *openAgentCursor) *openAgentClaim {
	for _, claim := range manager.claims {
		if openAgentPathKey(claim.AuditDir) == openAgentPathKey(cursor.Root) {
			return &claim
		}
	}
	return nil
}

func (manager *openAgentMonitorManager) hasClaimForRootLocked(root string) bool {
	for _, claim := range manager.claims {
		if openAgentPathKey(claim.AuditDir) == openAgentPathKey(root) {
			return true
		}
	}
	return false
}

func openAgentAuditFiles(root string) ([]string, bool, error) {
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.IsDir() {
		return nil, true, fmt.Errorf("OpenAgent audit root is not a directory: %s", root)
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
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jsonl") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	sort.Strings(files)
	return files, true, err
}

func openAgentSessionKeyFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func openAgentClaimKey(path, ownerName string) string {
	return strings.ToLower(filepath.Clean(path) + "\x00" + strings.TrimSpace(ownerName))
}

func openAgentPathKey(path string) string {
	key := filepath.Clean(path)
	if os.PathSeparator == '\\' {
		key = strings.ToLower(key)
	}
	return key
}

func (manager *openAgentMonitorManager) loadLocked() error {
	data, err := os.ReadFile(manager.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var saved openAgentSavedState
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("cannot parse OpenAgent audit monitor state: %w", err)
	}
	for _, claim := range saved.Claims {
		manager.claims[openAgentClaimKey(claim.Path, claim.Owner)] = claim
	}
	if saved.Cursors != nil {
		manager.cursors = saved.Cursors
	}
	return nil
}

func (manager *openAgentMonitorManager) saveLocked() error {
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
	saved := openAgentSavedState{Claims: make([]openAgentClaim, 0, len(manager.claims)), Cursors: manager.cursors}
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
