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

// Package agentpatch enables audit-only monitoring for locally discovered AI
// agent installations.
package agentpatch

import (
	"errors"
	"fmt"
)

// ErrNotSupported reports an agent Gateway can discover but cannot monitor yet.
var ErrNotSupported = errors.New("patching this agent is not supported yet")

// Target identifies one discovered agent installation.
type Target struct {
	AgentId string `json:"agentId"`
	Path    string `json:"path"`
	Owner   string `json:"owner"`
}

// Status is the monitoring state shown beside an agent installation.
type Status struct {
	Supported bool   `json:"supported"`
	Patched   bool   `json:"patched"`
	Detail    string `json:"detail,omitempty"`
	Notice    string `json:"notice,omitempty"`
	Followup  string `json:"followup,omitempty"`
}

type patcher interface {
	AgentId() string
	Supported() bool
	Status(Target) (Status, error)
	Patch(Target) error
	Unpatch(Target) error
}

type noticer interface {
	PatchNotice(bool) (notice string, followup string)
}

var patchers = map[string]patcher{}

func register(value patcher) {
	patchers[value.AgentId()] = value
}

// StatusOf reports one installation's monitoring state. Listing agents should
// remain useful even when a local configuration file cannot be read, so probe
// failures are returned as status detail rather than API failures.
func StatusOf(target Target) Status {
	patcher, ok := patchers[target.AgentId]
	if !ok {
		return Status{Detail: "no patcher registered for this agent"}
	}

	status, err := patcher.Status(target)
	if err != nil {
		status = Status{Detail: err.Error()}
	}
	status.Supported = patcher.Supported()
	if status.Supported {
		if source, ok := patcher.(noticer); ok {
			status.Notice, status.Followup = source.PatchNotice(status.Patched)
		} else if status.Patched {
			status.Notice = "Stops Gateway agent monitoring and restores files changed by the patch."
		} else {
			status.Notice = "Enables audit-only monitoring for this agent."
		}
	}
	return status
}

// Patch enables monitoring for target.
func Patch(target Target) error {
	patcher, err := patcherFor(target)
	if err != nil {
		return err
	}
	return patcher.Patch(target)
}

// Unpatch disables monitoring and restores files changed by Patch.
func Unpatch(target Target) error {
	patcher, err := patcherFor(target)
	if err != nil {
		return err
	}
	return patcher.Unpatch(target)
}

func patcherFor(target Target) (patcher, error) {
	if target.AgentId == "" {
		return nil, errors.New("agentId is required")
	}
	if target.Path == "" {
		return nil, errors.New("path is required")
	}
	patcher, ok := patchers[target.AgentId]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q", target.AgentId)
	}
	if !patcher.Supported() {
		return nil, fmt.Errorf("%s: %w", target.AgentId, ErrNotSupported)
	}
	return patcher, nil
}
