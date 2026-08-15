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

// Package agentmonitor tails supported local agent audit files and maintains
// Gateway's in-memory behaviour-monitoring window.
package agentmonitor

import (
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

// DefaultStateDir stores monitor declarations and tail cursors. Monitoring
// records themselves are never written here.
const DefaultStateDir = "./data/agent-patches"

var monitorConfig = struct {
	sync.RWMutex
	stateDir string
}{stateDir: DefaultStateDir}

// Configure selects the directory used for declaration and cursor JSON files.
// It must be called before Start when the default directory is unsuitable.
func Configure(stateDir string) {
	monitorConfig.Lock()
	if strings.TrimSpace(stateDir) == "" {
		monitorConfig.stateDir = DefaultStateDir
	} else {
		monitorConfig.stateDir = filepath.Clean(stateDir)
	}
	monitorConfig.Unlock()
}

func monitorStatePath(name string) string {
	monitorConfig.RLock()
	path := filepath.Join(monitorConfig.stateDir, name)
	monitorConfig.RUnlock()
	return path
}

// Start restores persisted monitor declarations and begins polling their local
// append-only logs. Each monitor starts independently.
func Start() error {
	return errors.Join(startCoworkMonitor(), codexMonitor.start(), openAgentMonitor.start())
}

// Stop stops every local tailer. It does not persist behaviour records.
func Stop() {
	stopCoworkMonitor()
	codexMonitor.stopMonitor()
	openAgentMonitor.stopMonitor()
}

func canonicalAgentID(agentID string) string {
	switch strings.ToLower(strings.TrimSpace(agentID)) {
	case "codex_vscode", "codex-vscode":
		return "codex-cli"
	default:
		return strings.ToLower(strings.TrimSpace(agentID))
	}
}
