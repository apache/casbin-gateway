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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
)

const maxCoworkMetadataBytes = 1024 * 1024

type coworkMetadata struct {
	SessionID string `json:"sessionId"`
	Title     string `json:"title"`
}

type coworkMetadataCacheEntry struct {
	metadata coworkMetadata
	modified time.Time
	size     int64
}

type coworkMetadataCache struct {
	entries map[string]coworkMetadataCacheEntry
}

func (cache *coworkMetadataCache) load(auditPath string) (coworkMetadata, bool, error) {
	metadataPath := filepath.Dir(filepath.Clean(auditPath)) + ".json"
	info, err := os.Stat(metadataPath)
	if os.IsNotExist(err) {
		delete(cache.entries, metadataPath)
		return coworkMetadata{}, false, nil
	}
	if err != nil {
		return coworkMetadata{}, false, fmt.Errorf("stat Cowork session metadata %s: %w", metadataPath, err)
	}
	cached, found := cache.entries[metadataPath]
	if found && cached.size == info.Size() && cached.modified.Equal(info.ModTime()) {
		return cached.metadata, false, nil
	}
	entry := coworkMetadataCacheEntry{modified: info.ModTime(), size: info.Size()}
	metadata, err := readCoworkMetadataFile(metadataPath, info.Size())
	if err != nil {
		cache.entries[metadataPath] = entry
		return coworkMetadata{}, false, err
	}
	entry.metadata = metadata
	cache.entries[metadataPath] = entry
	return metadata, true, nil
}

func readCoworkMetadataFile(path string, size int64) (coworkMetadata, error) {
	if size > maxCoworkMetadataBytes {
		return coworkMetadata{}, fmt.Errorf("Cowork session metadata exceeds %d bytes: %s", maxCoworkMetadataBytes, path)
	}
	file, err := os.Open(path)
	if err != nil {
		return coworkMetadata{}, fmt.Errorf("read Cowork session metadata %s: %w", path, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCoworkMetadataBytes+1))
	if err != nil {
		return coworkMetadata{}, fmt.Errorf("read Cowork session metadata %s: %w", path, err)
	}
	if len(data) > maxCoworkMetadataBytes {
		return coworkMetadata{}, fmt.Errorf("Cowork session metadata exceeds %d bytes: %s", maxCoworkMetadataBytes, path)
	}
	var metadata coworkMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return coworkMetadata{}, fmt.Errorf("decode Cowork session metadata %s: %w", path, err)
	}
	metadata.SessionID = strings.TrimSpace(metadata.SessionID)
	metadata.Title = auditutil.SanitizeString(strings.TrimSpace(metadata.Title))
	return metadata, nil
}

func (metadata coworkMetadata) sessionKey(auditPath string) string {
	value := metadata.SessionID
	if value == "" {
		value = filepath.Base(filepath.Dir(auditPath))
	}
	return strings.TrimPrefix(value, "local_")
}
