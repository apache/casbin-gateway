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
	_ "embed"
	"os"
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agentmonitor"
)

// opencodePluginFile is the plugin Gateway owns. opencode loads every module in
// the directory at session start, so installing one is writing this file.
const opencodePluginFile = gatewayEntryName + ".js"

// opencodeMonitorId is the agent every opencode record is reported under. The
// CLI and the desktop app read one ~/.config/opencode, so they share this file
// and the monitoring it feeds; the ingest credential is issued under this id so
// that either front end can report through it.
const opencodeMonitorId = "opencode"

//go:embed opencode_plugin.js
var opencodePluginSource string

type opencodePatcher struct {
	id string
}

func init() {
	register(opencodePatcher{id: "opencode"})
	register(opencodePatcher{id: "opencode-desktop"})
}

func (p opencodePatcher) AgentId() string { return p.id }

func (opencodePatcher) Supported() bool { return true }

func (p opencodePatcher) Patch(target Target) error {
	path, err := p.pluginPath(target)
	if err != nil {
		return err
	}
	plugin, err := renderOpencodePlugin(target)
	if err != nil {
		return err
	}
	return Apply(target, func(changes *ChangeSet) error {
		if err := changes.MkdirAll(filepath.Dir(path)); err != nil {
			return err
		}
		return changes.WriteFile(path, []byte(plugin), 0o644)
	})
}

func (p opencodePatcher) Unpatch(target Target) error {
	if err := Revert(target); err != nil {
		return err
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p opencodePatcher) Status(target Target) (Status, error) {
	path, err := p.pluginPath(target)
	if err != nil {
		return Status{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Detail: "opencode plugin is not installed"}, nil
		}
		return Status{}, err
	}
	current, err := recordsURL()
	if err != nil {
		return Status{}, err
	}
	// A plugin left by an older Gateway, or by one listening on another port,
	// reports nowhere. Re-patching is what rewrites it.
	if !strings.Contains(string(data), jsonString(current)) {
		return Status{Detail: "opencode plugin needs refresh"}, nil
	}
	if !IsApplied(target) {
		return Status{Patched: true, Detail: "opencode plugin was installed outside Gateway"}, nil
	}
	return Status{Patched: true, Detail: "opencode plugin active after restart"}, nil
}

func (opencodePatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's audit-only opencode plugin. The CLI and the desktop app share it, so both stop reporting.",
			"Restart any opencode session that is already running."
	}
	return "Installs an audit-only opencode plugin. It observes sessions, tool calls and permission prompts, and changes none of them.",
		"Restart any opencode session that is already running."
}

// pluginPath is the module opencode loads at session start. It sits beside the
// configuration file the MCP and skill listings already read.
func (opencodePatcher) pluginPath(target Target) (string, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "opencode", "plugin", opencodePluginFile), nil
}

func renderOpencodePlugin(target Target) (string, error) {
	url, err := recordsURL()
	if err != nil {
		return "", err
	}
	token, err := IssueIngestToken(monitorTarget(target))
	if err != nil {
		return "", err
	}

	plugin := opencodePluginSource
	for placeholder, value := range map[string]string{
		"__CASBIN_GATEWAY_AGENT__":               jsonString(opencodeMonitorId),
		"__CASBIN_GATEWAY_RECORDS_URL__":         jsonString(url),
		"__CASBIN_GATEWAY_AGENT_PATH__":          jsonString(target.Path),
		"__CASBIN_GATEWAY_OWNER__":               jsonString(target.Owner),
		"__CASBIN_GATEWAY_INGEST_TOKEN__":        jsonString(token),
		"__CASBIN_GATEWAY_INGEST_TOKEN_HEADER__": jsonString(agentmonitor.IngestTokenHeader),
	} {
		plugin = strings.ReplaceAll(plugin, placeholder, value)
	}
	return plugin, nil
}
