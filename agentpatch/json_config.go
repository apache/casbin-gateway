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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readJSONConfig(path string) (map[string]any, os.FileMode, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]any{}, 0o600, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, false, err
	}
	config := map[string]any{}
	if strings.TrimSpace(string(data)) == "" {
		return config, info.Mode().Perm(), true, nil
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, 0, false, fmt.Errorf("parse %s: %w", path, err)
	}
	if config == nil {
		return nil, 0, false, fmt.Errorf("parse %s: root must be a JSON object", path)
	}
	return config, info.Mode().Perm(), true, nil
}

func writeJSONConfig(path string, config map[string]any, mode os.FileMode) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), mode)
}

func objectAt(value any) (map[string]any, bool) {
	result, ok := value.(map[string]any)
	return result, ok
}

func stringArray(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return nil
		}
		result = append(result, text)
	}
	return result
}
