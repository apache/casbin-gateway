// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package agentpatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apache/casbin-gateway/conf"
)

const claudeGatewayToken = "casbin-gateway"

var claudeGatewayValues = map[string]string{
	"ANTHROPIC_AUTH_TOKEN":           claudeGatewayToken,
	"ANTHROPIC_MODEL":                "casbin-default",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "casbin-haiku",
	"ANTHROPIC_DEFAULT_SONNET_MODEL": "casbin-sonnet",
	"ANTHROPIC_DEFAULT_OPUS_MODEL":   "casbin-opus",
}

var claudeGatewayManagedKeys = []string{
	"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY",
	"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL",
}

type gatewayOriginalValue struct {
	Existed bool `json:"existed"`
	Value   any  `json:"value,omitempty"`
}

type gatewayState struct {
	Target   Target                          `json:"target"`
	Original map[string]gatewayOriginalValue `json:"original"`
}

type GatewayStatus struct {
	Configured bool   `json:"configured"`
	Restorable bool   `json:"restorable"`
	Endpoint   string `json:"endpoint"`
	Detail     string `json:"detail,omitempty"`
}

func claudeGatewayEndpoint() string {
	return fmt.Sprintf("http://127.0.0.1:%d", conf.GetHttpPort())
}

func gatewayStatePath(target Target) string {
	return filepath.Join(stateDir(), targetKey(target)+".gateway.json")
}

// ConfigureClaudeGateway connects a discovered Claude Code installation to
// Gateway while preserving the first pre-Gateway value of every managed key.
func ConfigureClaudeGateway(target Target) error {
	if target.AgentId != "claude-code" {
		return fmt.Errorf("Gateway configuration is unsupported for %s", target.AgentId)
	}
	stateMutex.Lock()
	defer stateMutex.Unlock()

	path, err := claudeCodeConfigPath(target)
	if err != nil {
		return err
	}
	config, mode, _, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	env, err := claudeEnv(config)
	if err != nil {
		return err
	}
	saved, err := loadGatewayState(target)
	if err != nil {
		return err
	}
	if saved == nil {
		original := make(map[string]gatewayOriginalValue, len(claudeGatewayManagedKeys))
		for _, key := range claudeGatewayManagedKeys {
			value, exists := env[key]
			original[key] = gatewayOriginalValue{Existed: exists, Value: value}
		}
		if err := saveGatewayState(target, &gatewayState{Target: target, Original: original}); err != nil {
			return err
		}
	}

	env["ANTHROPIC_BASE_URL"] = claudeGatewayEndpoint()
	for key, value := range claudeGatewayValues {
		env[key] = value
	}
	delete(env, "ANTHROPIC_API_KEY")
	return writeJSONConfig(path, config, mode)
}

func RestoreClaudeGateway(target Target) error {
	if target.AgentId != "claude-code" {
		return fmt.Errorf("Gateway configuration is unsupported for %s", target.AgentId)
	}
	stateMutex.Lock()
	defer stateMutex.Unlock()

	saved, err := loadGatewayState(target)
	if err != nil {
		return err
	}
	if saved == nil {
		return fmt.Errorf("no Gateway configuration backup exists")
	}
	path, err := claudeCodeConfigPath(target)
	if err != nil {
		return err
	}
	config, mode, _, err := readJSONConfig(path)
	if err != nil {
		return err
	}
	env, err := claudeEnv(config)
	if err != nil {
		return err
	}
	for _, key := range claudeGatewayManagedKeys {
		original := saved.Original[key]
		if original.Existed {
			env[key] = original.Value
		} else {
			delete(env, key)
		}
	}
	if len(env) == 0 {
		delete(config, "env")
	}
	if err := writeJSONConfig(path, config, mode); err != nil {
		return err
	}
	return os.Remove(gatewayStatePath(target))
}

func ClaudeGatewayStatusOf(target Target) GatewayStatus {
	status := GatewayStatus{Endpoint: claudeGatewayEndpoint()}
	if target.AgentId != "claude-code" {
		return status
	}
	stateMutex.Lock()
	defer stateMutex.Unlock()

	if saved, err := loadGatewayState(target); err == nil && saved != nil {
		status.Restorable = true
	} else if err != nil {
		status.Detail = err.Error()
		return status
	}
	path, err := claudeCodeConfigPath(target)
	if err != nil {
		status.Detail = err.Error()
		return status
	}
	config, _, exists, err := readJSONConfig(path)
	if err != nil {
		status.Detail = err.Error()
		return status
	}
	if !exists {
		status.Detail = "Claude Code is not connected to Gateway"
		return status
	}
	env, ok := objectAt(config["env"])
	if !ok {
		status.Detail = "Claude Code is not connected to Gateway"
		return status
	}
	configured := env["ANTHROPIC_BASE_URL"] == status.Endpoint
	for key, value := range claudeGatewayValues {
		configured = configured && env[key] == value
	}
	_, hasApiKey := env["ANTHROPIC_API_KEY"]
	configured = configured && !hasApiKey
	status.Configured = configured
	if configured {
		status.Detail = "Claude Code is connected to Gateway"
	} else {
		status.Detail = "Claude Code Gateway configuration needs refresh"
	}
	return status
}

func claudeEnv(config map[string]any) (map[string]any, error) {
	if value, exists := config["env"]; exists {
		env, ok := objectAt(value)
		if !ok {
			return nil, fmt.Errorf("env must be a JSON object")
		}
		return env, nil
	}
	env := map[string]any{}
	config["env"] = env
	return env, nil
}

func loadGatewayState(target Target) (*gatewayState, error) {
	data, err := os.ReadFile(gatewayStatePath(target))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var saved gatewayState
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, err
	}
	return &saved, nil
}

func saveGatewayState(target Target, saved *gatewayState) error {
	if err := os.MkdirAll(stateDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(gatewayStatePath(target), append(data, '\n'), 0o600)
}
