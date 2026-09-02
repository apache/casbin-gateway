// Copyright 2025 The casbin Authors. All Rights Reserved.
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

// Package agent detects AI agents in known installation locations.
package agent

// Installation describes an AI agent installation found on the host.
type Installation struct {
	AgentId       string `json:"agentId"`
	Name          string `json:"name"`
	Version       string `json:"version,omitempty"`
	Path          string `json:"path"`
	InstallMethod string `json:"installMethod"`
	Owner         string `json:"owner"`
}

// Known is one agent Gateway can detect, whether or not it is installed here.
type Known struct {
	AgentId    string `json:"agentId"`
	Name       string `json:"name"`
	InstallUrl string `json:"installUrl,omitempty"`
	// Desktop marks a windowed app, which is downloaded rather than installed
	// from a package manager.
	Desktop bool `json:"desktop,omitempty"`
}

// KnownAgents lists every agent a fingerprint declares, so the pages can show
// the ones this host has not installed next to the ones it has.
func KnownAgents() []Known {
	result := make([]Known, 0, len(fingerprints))
	for i := range fingerprints {
		result = append(result, Known{
			AgentId:    fingerprints[i].ID,
			Name:       fingerprints[i].DisplayName,
			InstallUrl: fingerprints[i].InstallUrl,
			Desktop:    fingerprints[i].Desktop,
		})
	}
	return result
}

// DisplayNameOf is the human-readable name of a known agent id, empty for an id
// no fingerprint declares.
func DisplayNameOf(id string) string {
	for i := range fingerprints {
		if fingerprints[i].ID == id {
			return fingerprints[i].DisplayName
		}
	}
	return ""
}

// RunsSandboxed reports whether an agent runs its sessions in a sandbox of its
// own. Claude Desktop does: its Cowork and Code sessions run in a virtual
// machine, where 127.0.0.1 is that machine rather than this host, so a loopback
// endpoint never reaches Gateway.
func RunsSandboxed(id string) bool {
	for i := range fingerprints {
		if fingerprints[i].ID == id {
			return fingerprints[i].Sandboxed
		}
	}
	return false
}

// IsKnownAgentId reads the fingerprints rather than a host scan, so an agent
// stays configurable while it is not installed.
func IsKnownAgentId(id string) bool {
	for i := range fingerprints {
		if fingerprints[i].ID == id {
			return true
		}
	}
	return false
}
