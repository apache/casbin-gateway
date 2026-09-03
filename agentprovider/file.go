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

package agentprovider

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// defaultMode is what a config file Gateway creates is born with. These files
// carry API keys, so a new one is not world-readable.
const defaultMode = os.FileMode(0o600)

func readFile(path string) ([]byte, os.FileMode, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, defaultMode, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, false, err
	}
	return data, info.Mode().Perm(), true, nil
}

func readJSON(path string) (map[string]any, os.FileMode, error) {
	data, mode, _, err := readFile(path)
	if err != nil {
		return nil, 0, err
	}

	config := map[string]any{}
	if strings.TrimSpace(string(data)) == "" {
		return config, mode, nil
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, 0, fmt.Errorf("parse %s: %w", path, err)
	}
	if config == nil {
		return nil, 0, fmt.Errorf("parse %s: the root must be a JSON object", path)
	}
	return config, mode, nil
}

func encodeJSON(config map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func objectAt(config map[string]any, key string) map[string]any {
	if value, ok := config[key].(map[string]any); ok {
		return value
	}
	return nil
}

func stringAt(config map[string]any, key string) string {
	value, _ := config[key].(string)
	return value
}

// staged is one file waiting to be renamed over its target.
type staged struct {
	path     string
	temp     string
	mode     os.FileMode
	previous []byte
	existed  bool
}

// txn replaces a set of configuration files in one step. An agent may read its
// own config while Gateway rewrites it, so nothing is truncated in place: every
// file is staged next to its target and renamed over it, and a rename that
// fails halfway puts the files already renamed back.
type txn struct {
	files []*staged
}

func (t *txn) write(path string, data []byte) error {
	previous, mode, existed, err := readFile(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".gateway-*")
	if err != nil {
		return err
	}
	item := &staged{path: path, temp: file.Name(), mode: mode, previous: previous, existed: existed}
	t.files = append(t.files, item)

	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Chmod(item.temp, mode)
}

func (t *txn) commit() error {
	for index, item := range t.files {
		if err := os.Rename(item.temp, item.path); err != nil {
			t.undo(t.files[:index])
			t.abort()
			return err
		}
	}
	t.files = nil
	return nil
}

func (t *txn) abort() {
	for _, item := range t.files {
		_ = os.Remove(item.temp)
	}
	t.files = nil
}

func (t *txn) undo(done []*staged) {
	for index := len(done) - 1; index >= 0; index-- {
		item := done[index]
		if !item.existed {
			_ = os.Remove(item.path)
			continue
		}
		_ = os.WriteFile(item.path, item.previous, item.mode)
	}
}

// nestedObject is the object at path, nil when a step is missing or holds
// something else.
func nestedObject(config map[string]any, path ...string) map[string]any {
	for _, key := range path {
		config = objectAt(config, key)
		if config == nil {
			return nil
		}
	}
	return config
}

// ensureNested is nestedObject with the missing steps created. A step holding
// something other than an object is replaced: the caller is about to own it,
// and has saved what was there.
func ensureNested(config map[string]any, path ...string) map[string]any {
	for _, key := range path {
		next := objectAt(config, key)
		if next == nil {
			next = map[string]any{}
			config[key] = next
		}
		config = next
	}
	return config
}

// pruneEmpty drops the object at path once it is empty, and every parent that
// empties with it, so removing an entry leaves the file as it was found.
func pruneEmpty(config map[string]any, path ...string) {
	for i := len(path); i > 0; i-- {
		parent := nestedObject(config, path[:i-1]...)
		if parent == nil {
			return
		}
		value := objectAt(parent, path[i-1])
		if value == nil || len(value) > 0 {
			return
		}
		delete(parent, path[i-1])
	}
}
