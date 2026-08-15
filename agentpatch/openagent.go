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

import "github.com/apache/casbin-gateway/agentmonitor"

// OpenAgent monitoring tails its audit JSONL files and does not edit OpenAgent
// configuration or data.
type openAgentPatcher struct{}

func init() {
	register(openAgentPatcher{})
}

func (openAgentPatcher) AgentId() string { return "openagent" }

func (openAgentPatcher) Supported() bool { return true }

func (openAgentPatcher) Status(target Target) (Status, error) {
	patched, detail := agentmonitor.OpenAgentMonitorStatus(target.Path, target.Owner)
	return Status{Patched: patched, Detail: detail}, nil
}

func (openAgentPatcher) Patch(target Target) error {
	dir, err := agentmonitor.ResolveOpenAgentAuditDir(target.Path, target.Owner)
	if err != nil {
		return err
	}
	return agentmonitor.EnableOpenAgentMonitor(target.Path, target.Owner, dir)
}

func (openAgentPatcher) Unpatch(target Target) error {
	return agentmonitor.DisableOpenAgentMonitor(target.Path, target.Owner)
}
