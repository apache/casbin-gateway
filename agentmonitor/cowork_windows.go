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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	claudeDesktopAgentID   = "claude-desktop"
	coworkMonitorStateFile = "claude-desktop-monitor.json"
)

type monitorTarget struct {
	Path  string `json:"path"`
	Owner string `json:"owner,omitempty"`
}

type coworkMonitorState struct {
	Targets []monitorTarget `json:"targets"`
}

type coworkMonitorManager struct {
	mutex      sync.Mutex
	targets    map[string]monitorTarget
	transcript *transcriptMonitor
}

var desktopMonitor = coworkMonitorManager{targets: map[string]monitorTarget{}}

func startCoworkMonitor() error {
	desktopMonitor.mutex.Lock()
	defer desktopMonitor.mutex.Unlock()
	data, err := os.ReadFile(monitorStatePath(coworkMonitorStateFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var saved coworkMonitorState
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("cannot parse Claude Desktop monitor state: %w", err)
	}
	for _, target := range saved.Targets {
		target.Path = filepath.Clean(target.Path)
		desktopMonitor.targets[coworkTargetKey(target.Path)] = target
	}
	if len(desktopMonitor.targets) != 0 && desktopMonitor.transcript == nil {
		desktopMonitor.transcript = newTranscriptMonitor(desktopMonitor.targets)
	}
	return nil
}

func stopCoworkMonitor() {
	desktopMonitor.mutex.Lock()
	transcript := desktopMonitor.transcript
	desktopMonitor.transcript = nil
	desktopMonitor.mutex.Unlock()
	if transcript != nil {
		transcript.Stop()
	}
}

// EnableCoworkMonitor persists a Claude Desktop installation and starts Cowork
// monitoring.
func EnableCoworkMonitor(path, owner string) error {
	target := monitorTarget{Path: filepath.Clean(path), Owner: strings.TrimSpace(owner)}
	desktopMonitor.mutex.Lock()
	defer desktopMonitor.mutex.Unlock()
	key := coworkTargetKey(target.Path)
	previous, existed := desktopMonitor.targets[key]
	desktopMonitor.targets[key] = target
	if err := desktopMonitor.saveLocked(); err != nil {
		if existed {
			desktopMonitor.targets[key] = previous
		} else {
			delete(desktopMonitor.targets, key)
		}
		return err
	}
	if desktopMonitor.transcript == nil {
		desktopMonitor.transcript = newTranscriptMonitor(desktopMonitor.targets)
	} else {
		desktopMonitor.transcript.SetTargets(desktopMonitor.targets)
	}
	return nil
}

// DisableCoworkMonitor removes a Cowork monitor declaration.
func DisableCoworkMonitor(path string) error {
	desktopMonitor.mutex.Lock()
	defer desktopMonitor.mutex.Unlock()
	key := coworkTargetKey(path)
	previous, existed := desktopMonitor.targets[key]
	if !existed {
		return nil
	}
	delete(desktopMonitor.targets, key)
	if err := desktopMonitor.saveLocked(); err != nil {
		desktopMonitor.targets[key] = previous
		return err
	}
	if desktopMonitor.transcript == nil {
		return nil
	}
	if len(desktopMonitor.targets) == 0 {
		desktopMonitor.transcript.Stop()
		desktopMonitor.transcript = nil
	} else {
		desktopMonitor.transcript.SetTargets(desktopMonitor.targets)
	}
	return nil
}

// CoworkMonitorStatus reports the Cowork transcript monitor state for one
// installation.
func CoworkMonitorStatus(path string) (bool, string) {
	desktopMonitor.mutex.Lock()
	defer desktopMonitor.mutex.Unlock()
	if _, found := desktopMonitor.targets[coworkTargetKey(path)]; !found {
		return false, "not patched"
	}
	if desktopMonitor.transcript == nil {
		return true, "Cowork transcript monitor enabled but inactive"
	}
	status := desktopMonitor.transcript.Status()
	if status.lastErr != nil {
		detail := "Cowork transcript monitor error: " + status.lastErr.Error()
		if len(status.existingRoots) != 0 {
			detail += "; discovered paths: " + strings.Join(status.existingRoots, ", ")
		}
		return true, detail
	}
	if len(status.existingRoots) == 0 {
		return true, "Cowork monitor enabled, but no audit directory was found; checked: " + strings.Join(status.configuredRoots, ", ")
	}
	if status.auditFileCount == 0 {
		return true, "Cowork monitor enabled, but no audit.jsonl was found; paths: " + strings.Join(status.existingRoots, ", ")
	}
	return true, fmt.Sprintf("Cowork transcript monitor active: %d audit.jsonl file(s); last successful poll %s; paths: %s", status.auditFileCount, status.lastSuccessfulPoll.Format(time.RFC3339Nano), strings.Join(status.existingRoots, ", "))
}

func (manager *coworkMonitorManager) saveLocked() error {
	path := monitorStatePath(coworkMonitorStateFile)
	if len(manager.targets) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	saved := coworkMonitorState{Targets: make([]monitorTarget, 0, len(manager.targets))}
	for _, target := range manager.targets {
		saved.Targets = append(saved.Targets, target)
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func coworkTargetKey(path string) string {
	return strings.ToLower(filepath.Clean(path))
}
