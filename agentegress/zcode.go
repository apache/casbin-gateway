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

package agentegress

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Snapshot is one workspace an agent packed up whole, read back from what the
// packing left on disk. It outlives the upload, which the watch only sees while
// it happens.
type Snapshot struct {
	Workspace      string    `json:"workspace"`
	EncryptedBytes int64     `json:"encryptedBytes,omitempty"`
	WorkspaceBytes int64     `json:"workspaceBytes,omitempty"`
	RecordedAt     time.Time `json:"recordedAt,omitempty"`
	// Accepted means the vendor confirmed it received the package.
	Accepted bool `json:"accepted"`
	// Pending means a package is queued, mid-upload, or waiting on a retry.
	Pending  bool `json:"pending"`
	Failures int  `json:"failures,omitempty"`
	// The rest is read from the plaintext manifest of the last package.
	Files    int   `json:"files,omitempty"`
	GitFiles int   `json:"gitFiles,omitempty"`
	GitBytes int64 `json:"gitBytes,omitempty"`
}

// ZCode's closed-source builds up to 3.12.x pack the worktree with its whole
// .git before each prompt and post it to Aliyun OSS; v2/checkpoints keeps a
// state.json and the manifests per workspace. ZCode's git checkpoints share the
// directory as <id>.json files, which carry none of these fields.
type zcodeState struct {
	WorkspacePath      string `json:"workspacePath"`
	FailureCount       *int   `json:"failureCount"`
	LastCompressedSize *struct {
		EncryptedSizeBytes int64  `json:"encryptedSizeBytes"`
		WorkspaceSizeBytes int64  `json:"workspaceSizeBytes"`
		ManifestHash       string `json:"manifestHash"`
		RecordedAt         int64  `json:"recordedAt"`
	} `json:"lastCompressedSize"`
	LastAcceptedManifestHash string          `json:"lastAcceptedManifestHash"`
	PendingUpload            json.RawMessage `json:"pendingUpload"`
	ActiveUpload             json.RawMessage `json:"activeUpload"`
}

type zcodeManifest struct {
	Files []struct {
		Path      string `json:"path"`
		SizeBytes int64  `json:"sizeBytes"`
	} `json:"files"`
}

const maxJsonBytes = 64 << 20

// SnapshotsOf reads the snapshots an agent left in the given home, nil for an
// agent known to leave none.
func SnapshotsOf(agentId, home string) []Snapshot {
	if agentId != "zcode" || home == "" {
		return nil
	}
	root := filepath.Join(home, ".zcode", "v2", "checkpoints")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	snapshots := []Snapshot{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if snapshot, ok := readZcodeWorkspace(filepath.Join(root, entry.Name())); ok {
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].RecordedAt.After(snapshots[j].RecordedAt) })
	return snapshots
}

func readZcodeWorkspace(dir string) (Snapshot, bool) {
	var state zcodeState
	if !readJson(filepath.Join(dir, "state.json"), &state) {
		return Snapshot{}, false
	}
	archives := hasArchive(dir)
	pending := present(state.PendingUpload) || present(state.ActiveUpload) || archives
	if state.FailureCount == nil && state.LastCompressedSize == nil && state.LastAcceptedManifestHash == "" && !pending {
		return Snapshot{}, false
	}

	snapshot := Snapshot{
		Workspace: state.WorkspacePath,
		Accepted:  state.LastAcceptedManifestHash != "",
		Pending:   pending,
	}
	if snapshot.Workspace == "" {
		snapshot.Workspace = dir
	}
	if state.FailureCount != nil {
		snapshot.Failures = *state.FailureCount
	}
	hash := state.LastAcceptedManifestHash
	if size := state.LastCompressedSize; size != nil {
		snapshot.EncryptedBytes = size.EncryptedSizeBytes
		snapshot.WorkspaceBytes = size.WorkspaceSizeBytes
		if size.RecordedAt > 0 {
			snapshot.RecordedAt = time.UnixMilli(size.RecordedAt)
		}
		if size.ManifestHash != "" {
			hash = size.ManifestHash
		}
	}

	var manifest zcodeManifest
	if hash != "" && readJson(filepath.Join(dir, "manifests", hash+".json"), &manifest) {
		snapshot.Files = len(manifest.Files)
		for _, file := range manifest.Files {
			if file.Path == ".git" || strings.HasPrefix(file.Path, ".git/") {
				snapshot.GitFiles++
				snapshot.GitBytes += file.SizeBytes
			}
		}
	}
	return snapshot, true
}

func hasArchive(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(entry.Name(), ".enc") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

func readJson(path string, into any) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxJsonBytes {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, into) == nil
}

func present(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && !bytes.Equal(raw, []byte("null"))
}
