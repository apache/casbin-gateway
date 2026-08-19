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
	"fmt"
	"os"
	"path/filepath"

	"github.com/apache/casbin-gateway/agentfile"
)

const (
	claudeBaseURL   = "ANTHROPIC_BASE_URL"
	claudeAuthToken = "ANTHROPIC_AUTH_TOKEN"
	claudeAPIKey    = "ANTHROPIC_API_KEY"
)

var claudeModelEnv = map[string]string{
	"model":       "ANTHROPIC_MODEL",
	"haikuModel":  "ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"sonnetModel": "ANTHROPIC_DEFAULT_SONNET_MODEL",
	"opusModel":   "ANTHROPIC_DEFAULT_OPUS_MODEL",
}

type valueBackup struct {
	Exists bool `json:"exists"`
	Value  any  `json:"value,omitempty"`
}

type configBackup struct {
	FileExists bool                   `json:"fileExists"`
	EnvExists  bool                   `json:"envExists"`
	Values     map[string]valueBackup `json:"values"`
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
	config, _, err := agentfile.ReadJSON(a.configPath)
	return config, err
}

func (a *ClaudeCodeAdapter) Takeover(settings Config) error {
	return agentfile.UpdateJSON(a.configPath, func(config map[string]any, fileExists bool) (agentfile.Action, error) {
		env, envExists, err := envObject(config)
		if err != nil {
			return agentfile.Keep, err
		}
		if err = a.createBackup(fileExists, envExists, env); err != nil {
			return agentfile.Keep, err
		}

		config["env"] = env
		env[claudeBaseURL] = settings.Endpoint
		env[claudeAuthToken] = settings.Token
		delete(env, claudeAPIKey)
		for field, envKey := range claudeModelEnv {
			setOrDelete(env, envKey, settings.Values[field])
		}
		return agentfile.Write, nil
	})
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
	return agentfile.UpdateJSON(a.configPath, func(config map[string]any, _ bool) (agentfile.Action, error) {
		env, _, envErr := envObject(config)
		if envErr != nil {
			return agentfile.Keep, envErr
		}
		for _, key := range managedEnvKeys() {
			value := backup.Values[key]
			if value.Exists {
				env[key] = value.Value
			} else {
				delete(env, key)
			}
		}
		if backup.EnvExists || len(env) > 0 {
			config["env"] = env
		} else {
			delete(config, "env")
		}
		if !backup.FileExists && len(config) == 0 {
			return agentfile.Remove, nil
		}
		return agentfile.Write, nil
	}, func() error { return os.Remove(a.backupPath) })
}

func (a *ClaudeCodeAdapter) Status() (Status, error) {
	if _, err := os.Stat(a.backupPath); errors.Is(err, os.ErrNotExist) {
		return Status{}, nil
	} else if err != nil {
		return Status{}, err
	}

	config, _, err := agentfile.ReadJSON(a.configPath)
	if err != nil {
		return Status{}, err
	}
	env, _, err := envObject(config)
	if err != nil {
		return Status{}, err
	}
	endpoint, _ := env[claudeBaseURL].(string)
	token, _ := env[claudeAuthToken].(string)
	values := map[string]string{}
	for field, envKey := range claudeModelEnv {
		if value := stringValue(env[envKey]); value != "" {
			values[field] = value
		}
	}
	return Status{
		TakenOver:  true,
		Configured: endpoint != "" && token != "",
		Endpoint:   endpoint,
		Values:     values,
	}, nil
}

func setOrDelete(env map[string]any, key, value string) {
	if value == "" {
		delete(env, key)
		return
	}
	env[key] = value
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func (a *ClaudeCodeAdapter) createBackup(fileExists, envExists bool, env map[string]any) error {
	if _, err := os.Stat(a.backupPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	backup := configBackup{
		FileExists: fileExists,
		EnvExists:  envExists,
		Values:     map[string]valueBackup{},
	}
	for _, key := range managedEnvKeys() {
		if value, exists := env[key]; exists {
			backup.Values[key] = valueBackup{Exists: true, Value: value}
		}
	}

	data, err := json.Marshal(backup)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(a.backupPath), 0o700); err != nil {
		return err
	}
	return os.WriteFile(a.backupPath, data, 0o600)
}

func managedEnvKeys() []string {
	keys := []string{claudeBaseURL, claudeAuthToken, claudeAPIKey}
	for _, key := range claudeModelEnv {
		keys = append(keys, key)
	}
	return keys
}

func envObject(config map[string]any) (map[string]any, bool, error) {
	value, exists := config["env"]
	if !exists {
		return map[string]any{}, false, nil
	}
	env, ok := value.(map[string]any)
	if !ok {
		return nil, true, fmt.Errorf("env must be a JSON object")
	}
	return env, true, nil
}
