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

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/apache/casbin-gateway/conf"
)

// InstallMethodManual marks an agent someone pointed Gateway at themselves,
// which is all that is left for one no layout and no PATH describes.
const InstallMethodManual = "manual"

type manualEntry struct {
	AgentId string `json:"agentId"`
	Path    string `json:"path"`
	Owner   string `json:"owner,omitempty"`
}

type manualStore struct {
	Installations []manualEntry `json:"installations"`
}

var manualLock sync.Mutex

// manualFile records the chosen programs, beside the other local agent state.
func manualFile() string {
	return filepath.Join(conf.GetAgentPatchStateDir(), "manual-agents.json")
}

// ManualInstallations are the chosen programs that are still there. One since
// deleted is left out, but its entry stays: restoring the file brings it back.
func ManualInstallations() []Installation {
	manualLock.Lock()
	defer manualLock.Unlock()

	var result []Installation
	for _, entry := range readManual().Installations {
		if !IsKnownAgentId(entry.AgentId) || !isPathExecutable(entry.Path) {
			continue
		}
		result = append(result, Installation{
			AgentId: entry.AgentId, Name: DisplayNameOf(entry.AgentId), Path: entry.Path,
			InstallMethod: InstallMethodManual, Owner: entry.Owner,
		})
	}
	return result
}

// AddManualInstallation records one program as an installation of an agent. Only
// what the host confirms is accepted: a known agent, and a file that can be run.
func AddManualInstallation(agentId string, path string) (Installation, error) {
	if !IsKnownAgentId(agentId) {
		return Installation{}, fmt.Errorf("unknown agent: %s", agentId)
	}
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if path == "" {
		return Installation{}, errors.New("the program path is empty")
	}
	if !filepath.IsAbs(path) {
		return Installation{}, fmt.Errorf("the program path must be absolute: %s", path)
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return Installation{}, fmt.Errorf("the program was not found: %s", path)
	}
	if info.IsDir() {
		return Installation{}, fmt.Errorf("this is a directory, not a program: %s", path)
	}
	if !isPathExecutable(path) {
		return Installation{}, fmt.Errorf("this file cannot be run: %s", path)
	}

	installation := Installation{
		AgentId: agentId, Name: DisplayNameOf(agentId), Path: path,
		InstallMethod: InstallMethodManual, Owner: currentOwner(),
	}

	manualLock.Lock()
	defer manualLock.Unlock()

	store := readManual()
	entries := make([]manualEntry, 0, len(store.Installations)+1)
	for _, entry := range store.Installations {
		if entry.AgentId != agentId || !samePath(entry.Path, path) {
			entries = append(entries, entry)
		}
	}
	// One agent may be installed more than once, so this joins the others.
	entries = append(entries, manualEntry{AgentId: agentId, Path: path, Owner: installation.Owner})
	if err := writeManual(manualStore{Installations: entries}); err != nil {
		return Installation{}, err
	}
	invalidateScanCache()
	return installation, nil
}

// RemoveManualInstallation forgets one chosen program. Nothing on disk is
// touched: Gateway was told where it is, not asked to install it.
func RemoveManualInstallation(agentId string, path string) error {
	manualLock.Lock()
	defer manualLock.Unlock()

	store := readManual()
	entries := make([]manualEntry, 0, len(store.Installations))
	removed := false
	for _, entry := range store.Installations {
		if entry.AgentId == agentId && (path == "" || samePath(entry.Path, path)) {
			removed = true
			continue
		}
		entries = append(entries, entry)
	}
	if !removed {
		return fmt.Errorf("this agent was not added by hand: %s", agentId)
	}
	if err := writeManual(manualStore{Installations: entries}); err != nil {
		return err
	}
	invalidateScanCache()
	return nil
}

func readManual() manualStore {
	store := manualStore{}
	data, err := os.ReadFile(manualFile())
	if err != nil {
		return store
	}
	if json.Unmarshal(data, &store) != nil {
		return manualStore{}
	}
	return store
}

func writeManual(store manualStore) error {
	path := manualFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func samePath(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
