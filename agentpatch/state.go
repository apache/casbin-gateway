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

package agentpatch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
)

type changeKind string

const (
	changeFile changeKind = "file"
	changeDir  changeKind = "dir"
)

type change struct {
	Kind        changeKind  `json:"kind"`
	Path        string      `json:"path"`
	Backup      string      `json:"backup,omitempty"`
	Mode        os.FileMode `json:"mode,omitempty"`
	PatchedHash string      `json:"patchedHash,omitempty"`
	PatchedMode os.FileMode `json:"patchedMode,omitempty"`
}

type manifest struct {
	AgentId   string    `json:"agentId"`
	Target    Target    `json:"target"`
	PatchedAt time.Time `json:"patchedAt"`
	Changes   []change  `json:"changes"`
}

var stateMutex sync.Mutex

// ChangeSet records the files and directories a patch owns. Files are restored
// only when they still contain the patch's content, so Unpatch never overwrites
// a configuration an operator has changed since Patch.
type ChangeSet struct {
	manifest  *manifest
	backupDir string
}

// MkdirAll creates dir and records only directories this patch created.
func (c *ChangeSet) MkdirAll(dir string) error {
	var created []string
	for current := filepath.Clean(dir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(current); err == nil {
			break
		}
		created = append(created, current)
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for index := len(created) - 1; index >= 0; index-- {
		c.manifest.Changes = append(c.manifest.Changes, change{Kind: changeDir, Path: created[index]})
	}
	return nil
}

// ReadFile reads a patch target, returning empty content for a file Patch will
// create.
func (c *ChangeSet) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

// WriteFile backs up the pre-patch file and records the replacement it owns.
func (c *ChangeSet) WriteFile(path string, data []byte, perm os.FileMode) error {
	item := change{Kind: changeFile, Path: path}
	if previous, err := os.ReadFile(path); err == nil {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		item.Mode = info.Mode().Perm()
		perm = item.Mode
		if err := os.MkdirAll(c.backupDir, 0o700); err != nil {
			return err
		}
		item.Backup = fmt.Sprintf("%d-%s", len(c.manifest.Changes), filepath.Base(path))
		if err := os.WriteFile(filepath.Join(c.backupDir, item.Backup), previous, 0o600); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	item.PatchedHash = contentHash(data)
	item.PatchedMode = info.Mode().Perm()
	c.manifest.Changes = append(c.manifest.Changes, item)
	return nil
}

// Apply runs a file-changing patch transaction and persists the information
// needed for Unpatch. A failed patch is rolled back before its error returns.
func Apply(target Target, apply func(*ChangeSet) error) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	if err := revertLocked(target); err != nil {
		return err
	}
	changes := &ChangeSet{
		manifest:  &manifest{AgentId: target.AgentId, Target: target, PatchedAt: time.Now()},
		backupDir: backupDir(target),
	}
	if err := apply(changes); err != nil {
		_ = rollback(changes.manifest, changes.backupDir)
		return err
	}
	if err := saveManifest(target, changes.manifest); err != nil {
		_ = rollback(changes.manifest, changes.backupDir)
		_ = os.Remove(manifestPath(target))
		_ = os.RemoveAll(changes.backupDir)
		return err
	}
	return nil
}

// Revert restores a patch's unchanged files. It is a no-op when the target is
// not currently tracked by Gateway.
func Revert(target Target) error {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	return revertLocked(target)
}

func revertLocked(target Target) error {
	saved, err := loadManifest(target)
	if err != nil || saved == nil {
		return err
	}
	if err := rollback(saved, backupDir(target)); err != nil {
		return err
	}
	if err := os.Remove(manifestPath(target)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.RemoveAll(backupDir(target))
}

// IsApplied reports whether Gateway has the manifest needed to restore target.
func IsApplied(target Target) bool {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	saved, err := loadManifest(target)
	return err == nil && saved != nil
}

func rollback(saved *manifest, backups string) error {
	if err := verifyPatchOwnership(saved); err != nil {
		return err
	}

	var first error
	for index := len(saved.Changes) - 1; index >= 0; index-- {
		item := saved.Changes[index]
		var err error
		switch item.Kind {
		case changeDir:
			_ = os.Remove(item.Path)
		case changeFile:
			if item.Backup == "" {
				err = os.Remove(item.Path)
				if os.IsNotExist(err) {
					err = nil
				}
			} else if content, readErr := os.ReadFile(filepath.Join(backups, item.Backup)); readErr != nil {
				err = readErr
			} else {
				err = os.WriteFile(item.Path, content, item.Mode)
				if err == nil {
					err = os.Chmod(item.Path, item.Mode)
				}
			}
		}
		if err != nil && first == nil {
			first = fmt.Errorf("restore %s: %w", item.Path, err)
		}
	}
	return first
}

func verifyPatchOwnership(saved *manifest) error {
	for _, item := range saved.Changes {
		if item.Kind != changeFile {
			continue
		}
		content, err := os.ReadFile(item.Path)
		if err != nil {
			return fmt.Errorf("cannot safely unpatch %s: %w", item.Path, err)
		}
		info, err := os.Stat(item.Path)
		if err != nil {
			return fmt.Errorf("cannot safely unpatch %s: %w", item.Path, err)
		}
		if item.PatchedHash == "" || item.PatchedHash != contentHash(content) || item.PatchedMode != info.Mode().Perm() {
			return fmt.Errorf("cannot safely unpatch %s because it changed after Patch", item.Path)
		}
	}
	return nil
}

func stateDir() string {
	return conf.GetAgentPatchStateDir()
}

func targetKey(target Target) string {
	sum := sha256.Sum256([]byte(target.AgentId + "|" + target.Owner + "|" + target.Path))
	return target.AgentId + "-" + hex.EncodeToString(sum[:])[:16]
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func manifestPath(target Target) string {
	return filepath.Join(stateDir(), targetKey(target)+".json")
}

func backupDir(target Target) string {
	return filepath.Join(stateDir(), targetKey(target))
}

func loadManifest(target Target) (*manifest, error) {
	data, err := os.ReadFile(manifestPath(target))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var saved manifest
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	return &saved, nil
}

func saveManifest(target Target, saved *manifest) error {
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(target), append(data, '\n'), 0o600)
}
