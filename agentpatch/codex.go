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

// Codex monitoring tails rollout files; it never changes Codex settings or
// installs a hook inside Codex.
type codexPatcher struct {
	id string
}

func init() {
	register(codexPatcher{id: "codex"})
	register(codexPatcher{id: "codex-cli"})
}

func (p codexPatcher) AgentId() string { return p.id }

func (codexPatcher) Supported() bool { return true }

func (p codexPatcher) Status(target Target) (Status, error) {
	patched, detail := agentmonitor.CodexMonitorStatus(target.AgentId, target.Path, target.Owner)
	return Status{Patched: patched, Detail: detail}, nil
}

func (p codexPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Stops Gateway rollout monitoring. Codex files and settings stay unchanged.", ""
	}
	return "Enables audit-only rollout monitoring. No Codex setting or hook is installed.", ""
}

func (p codexPatcher) Patch(target Target) error {
	home, err := agentmonitor.ResolveCodexHome(target.Path, target.Owner)
	if err != nil {
		return err
	}
	return agentmonitor.EnableCodexMonitor(target.AgentId, target.Path, target.Owner, home)
}

func (p codexPatcher) Unpatch(target Target) error {
	return agentmonitor.DisableCodexMonitor(target.AgentId, target.Path, target.Owner)
}
