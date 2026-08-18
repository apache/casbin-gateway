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

package agentconfig

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const (
	claudeBaseURL   = "ANTHROPIC_BASE_URL"
	claudeAuthToken = "ANTHROPIC_AUTH_TOKEN"
	claudeAPIKey    = "ANTHROPIC_API_KEY"
)

type configBackup struct {
	Exists bool        `json:"exists"`
	Mode   os.FileMode `json:"mode"`
	Data   []byte      `json:"data"`
}

// ClaudeCodeAdapter manages ~/.claude/settings.json.
type ClaudeCodeAdapter struct {
	configPath string
	backupPath string
}

func NewClaudeCodeAdapter(home string) *ClaudeCodeAdapter {
	configPath := filepath.Join(home, ".claude", "settings.json")
	return &ClaudeCodeAdapter{
		configPath: configPath,
		backupPath: configPath + ".casbin-gateway.bak",
	}
}

func (a *ClaudeCodeAdapter) ConfigPath() string {
	return a.configPath
}

func (a *ClaudeCodeAdapter) Read() (map[string]any, error) {
	data, err := os.ReadFile(a.configPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}

	config := map[string]any{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func (a *ClaudeCodeAdapter) Takeover(endpoint, token string) error {
	if err := a.createBackup(); err != nil {
		return err
	}

	config, err := a.Read()
	if err != nil {
		return err
	}
	env, ok := config["env"].(map[string]any)
	if !ok {
		env = map[string]any{}
		config["env"] = env
	}
	env[claudeBaseURL] = endpoint
	env[claudeAuthToken] = token
	delete(env, claudeAPIKey)

	return writeJSON(a.configPath, config, 0o600)
}

func (a *ClaudeCodeAdapter) Restore() error {
	data, err := os.ReadFile(a.backupPath)
	if err != nil {
		return err
	}

	var backup configBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return err
	}
	if backup.Exists {
		if err := writeFile(a.configPath, backup.Data, backup.Mode); err != nil {
			return err
		}
	} else if err := os.Remove(a.configPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return os.Remove(a.backupPath)
}

func (a *ClaudeCodeAdapter) Status() (Status, error) {
	if _, err := os.Stat(a.backupPath); errors.Is(err, os.ErrNotExist) {
		return Status{}, nil
	} else if err != nil {
		return Status{}, err
	}

	config, err := a.Read()
	if err != nil {
		return Status{}, err
	}
	env, _ := config["env"].(map[string]any)
	endpoint, _ := env[claudeBaseURL].(string)
	return Status{TakenOver: endpoint != "", Endpoint: endpoint}, nil
}

func (a *ClaudeCodeAdapter) createBackup() error {
	if _, err := os.Stat(a.backupPath); err == nil {
		return nil
	}

	backup := configBackup{}
	data, err := os.ReadFile(a.configPath)
	if err == nil {
		info, statErr := os.Stat(a.configPath)
		if statErr != nil {
			return statErr
		}
		backup = configBackup{Exists: true, Mode: info.Mode().Perm(), Data: data}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	data, err = json.Marshal(backup)
	if err != nil {
		return err
	}
	return writeFile(a.backupPath, data, 0o600)
}

func writeJSON(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, append(data, '\n'), mode)
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}
