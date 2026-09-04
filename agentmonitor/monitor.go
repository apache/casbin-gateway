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
	"time"
)

const (
	// DefaultStateDir stores monitor declarations and tail cursors. Monitoring
	// records themselves are never written here.
	DefaultStateDir = "./data/agent-patches"

	// DefaultPollInterval is how often file-tailed agents are rescanned. Agent
	// session directories only ever grow, so polling faster than this costs real
	// disk traffic for very little extra freshness.
	DefaultPollInterval = 5 * time.Second
)

var monitorConfig = struct {
	sync.RWMutex
	stateDir     string
	pollInterval time.Duration
}{stateDir: DefaultStateDir, pollInterval: DefaultPollInterval}

// Configure selects the directory used for declaration and cursor JSON files
// and how often local logs are rescanned. It must be called before Start when
// the defaults are unsuitable.
func Configure(stateDir string, pollInterval time.Duration) {
	monitorConfig.Lock()
	if strings.TrimSpace(stateDir) == "" {
		monitorConfig.stateDir = DefaultStateDir
	} else {
		monitorConfig.stateDir = filepath.Clean(stateDir)
	}
	if pollInterval <= 0 {
		monitorConfig.pollInterval = DefaultPollInterval
	} else {
		monitorConfig.pollInterval = pollInterval
	}
	monitorConfig.Unlock()
}

func monitorStatePath(name string) string {
	monitorConfig.RLock()
	path := filepath.Join(monitorConfig.stateDir, name)
	monitorConfig.RUnlock()
	return path
}

func monitorPollInterval() time.Duration {
	monitorConfig.RLock()
	interval := monitorConfig.pollInterval
	monitorConfig.RUnlock()
	return interval
}

// Start restores persisted monitor declarations and begins polling their local
// append-only logs. Each monitor starts independently.
func Start() error {
	return errors.Join(startCoworkMonitor(), codexMonitor.start(), openAgentMonitor.start())
}

// Stop stops every local tailer.
func Stop() {
	stopCoworkMonitor()
	codexMonitor.stopMonitor()
	openAgentMonitor.stopMonitor()
}

// MonitorAgentId is the agent one installation's records are reported under.
// Front ends that share a configuration share the monitoring it feeds: the
// Codex VS Code integration reports as the CLI, and the opencode desktop app
// as opencode.
func MonitorAgentId(agentID string) string {
	return canonicalAgentID(agentID)
}

func canonicalAgentID(agentID string) string {
	switch strings.ToLower(strings.TrimSpace(agentID)) {
	case "codex_vscode", "codex-vscode":
		return "codex-cli"
	case "opencode-desktop":
		return "opencode"
	case "cursor-agent":
		return "cursor"
	default:
		return strings.ToLower(strings.TrimSpace(agentID))
	}
}
