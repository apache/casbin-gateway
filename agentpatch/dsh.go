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
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

const (
	// dshPluginFile is the telemetry backend Gateway owns. It sits beside the
	// profiles' shared node_modules so that its import of the harness's own
	// telemetry package resolves, and carries the extension that says it is an
	// ES module: the directory has no package.json to say so, and Node warns
	// into the operator's own dsh output when it has to guess.
	dshPluginFile = gatewayEntryName + ".mjs"
)

//go:embed dsh_plugin.mjs
var dshPluginSource string

type dshPatcher struct{}

func init() {
	register(dshPatcher{})
}

func (dshPatcher) AgentId() string { return "dsh" }

func (dshPatcher) Supported() bool { return true }

func (p dshPatcher) Patch(target Target) error {
	layout, err := p.layoutOf(target)
	if err != nil {
		return err
	}
	plugin, err := renderDshPlugin(target)
	if err != nil {
		return err
	}

	// The plugin file is the patch's own, so it is written through the change
	// set and put back by it. The patch list is shared with the MCP servers an
	// operator adds, so the entry in it is written last and removed by name:
	// a failure here rolls the plugin file back before this returns.
	return Apply(target, func(changes *ChangeSet) error {
		if err := changes.MkdirAll(filepath.Dir(layout.pluginPath)); err != nil {
			return err
		}
		if err := changes.WriteFile(layout.pluginPath, []byte(plugin), 0o644); err != nil {
			return err
		}
		return setDshEntry(layout, true)
	})
}

func (p dshPatcher) Unpatch(target Target) error {
	layout, err := p.layoutOf(target)
	if err != nil {
		return err
	}
	if err := setDshEntry(layout, false); err != nil {
		return err
	}
	if err := Revert(target); err != nil {
		return err
	}
	return RevokeIngestToken(monitorTarget(target))
}

func (p dshPatcher) Status(target Target) (Status, error) {
	layout, err := p.layoutOf(target)
	if err != nil {
		return Status{}, err
	}

	data, err := os.ReadFile(layout.pluginPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Status{Detail: "dsh plugin is not installed"}, nil
		}
		return Status{}, err
	}
	entry, err := dshEntryName(layout)
	if err != nil {
		return Status{}, err
	}
	if entry == "" {
		return Status{Detail: "dsh plugin is installed but not listed in cordis.patch.yml"}, nil
	}

	current, err := recordsURL()
	if err != nil {
		return Status{}, err
	}
	// A plugin left by an older Gateway, or by one listening on another port,
	// reports nowhere. Re-patching is what rewrites it.
	if !strings.Contains(string(data), jsonString(current)) || entry != layout.moduleName {
		return Status{Detail: "dsh plugin needs refresh"}, nil
	}
	if !IsApplied(target) {
		return Status{Patched: true, Detail: "dsh plugin was installed outside Gateway"}, nil
	}
	return Status{Patched: true, Detail: "dsh plugin active after restart"}, nil
}

func (dshPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes Gateway's audit-only dsh plugin.", "Restart dsh to stop loading it."
	}
	return "Installs an audit-only dsh plugin. It listens on the harness's session bus and changes nothing a session does.",
		"Restart dsh to load it."
}

// dshLayout is where one installation keeps the two files a patch touches: the
// plugin module and the home-level patch layer that loads it.
type dshLayout struct {
	pluginPath string
	patchPath  string
	// moduleName is the specifier the loader imports the plugin by. The home
	// layer is composed into every profile, whose base URLs differ, so the
	// entry names an absolute file URL rather than a relative path.
	moduleName string
}

func (p dshPatcher) layoutOf(target Target) (dshLayout, error) {
	home, err := dshHome(target.Owner)
	if err != nil {
		return dshLayout{}, err
	}
	pluginPath := filepath.Join(home, "profiles", dshPluginFile)
	return dshLayout{
		pluginPath: pluginPath,
		patchPath:  filepath.Join(home, "cordis.patch.yml"),
		moduleName: fileURL(pluginPath),
	}, nil
}

// dshHome is the harness home the owner's dsh reads. DSH_HOME is meaningful
// only for the account Gateway runs as.
func dshHome(owner string) (string, error) {
	if current, err := user.Current(); err == nil && agenthome.SameAccount(owner, current.Username) {
		if configured := strings.TrimSpace(os.Getenv("DSH_HOME")); configured != "" {
			if !filepath.IsAbs(configured) {
				return "", errors.New("DSH_HOME must be an absolute path")
			}
			return filepath.Clean(configured), nil
		}
	}
	home, err := agenthome.Resolve(owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dsh"), nil
}

// fileURL is the absolute module specifier the loader imports. A Windows path
// starts with its drive letter rather than a slash, and a file URL without one
// would read the drive as the host.
func fileURL(path string) string {
	slashed := filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return (&url.URL{Scheme: "file", Path: slashed}).String()
}

func renderDshPlugin(target Target) (string, error) {
	recordsUrl, err := recordsURL()
	if err != nil {
		return "", err
	}
	token, err := IssueIngestToken(monitorTarget(target))
	if err != nil {
		return "", err
	}

	plugin := dshPluginSource
	for placeholder, value := range map[string]string{
		"__CASBIN_GATEWAY_AGENT__":               jsonString("dsh"),
		"__CASBIN_GATEWAY_RECORDS_URL__":         jsonString(recordsUrl),
		"__CASBIN_GATEWAY_AGENT_PATH__":          jsonString(target.Path),
		"__CASBIN_GATEWAY_OWNER__":               jsonString(target.Owner),
		"__CASBIN_GATEWAY_INGEST_TOKEN__":        jsonString(token),
		"__CASBIN_GATEWAY_INGEST_TOKEN_HEADER__": jsonString(agentmonitor.IngestTokenHeader),
	} {
		plugin = strings.ReplaceAll(plugin, placeholder, value)
	}
	return plugin, nil
}

// setDshEntry adds or removes the loader entry that mounts the plugin. The file
// is a list of patches, each of which may insert entries into the composed
// tree; Gateway owns one row in it and leaves every other patch alone.
func setDshEntry(layout dshLayout, want bool) error {
	document, patches, err := loadDshPatches(layout.patchPath)
	if err != nil {
		return err
	}

	kept := patches.Content[:0]
	found := false
	for _, patch := range patches.Content {
		inserts := yamledit.Get(patch, "insert")
		if inserts != nil && inserts.Kind == yaml.SequenceNode {
			rows := inserts.Content[:0]
			for _, row := range inserts.Content {
				if yamledit.String(row, "id") != gatewayEntryName {
					rows = append(rows, row)
					continue
				}
				found = true
				if want {
					// The row is rewritten rather than kept, so a moved plugin
					// or a renamed module is corrected by patching again.
					if err := yamledit.Set(row, layout.moduleName, "name"); err != nil {
						return err
					}
					rows = append(rows, row)
				}
			}
			inserts.Content = rows
			// A patch whose only purpose was the entry it no longer inserts is
			// not a patch worth keeping.
			if len(rows) == 0 && len(patch.Content) == 2 {
				continue
			}
		}
		kept = append(kept, patch)
	}
	patches.Content = kept

	if want && !found {
		row, err := yamledit.Node(map[string]any{"id": gatewayEntryName, "name": layout.moduleName})
		if err != nil {
			return err
		}
		patch, err := yamledit.Node(map[string]any{"insert": []any{}})
		if err != nil {
			return err
		}
		inserts := yamledit.Get(patch, "insert")
		inserts.Style = 0
		inserts.Content = append(inserts.Content, row)
		patches.Content = append(patches.Content, patch)
	}
	if !want && !found {
		return nil
	}
	return saveDshPatches(layout.patchPath, document)
}

// dshEntryName is the module the patch list loads Gateway's plugin from, empty
// when the list has no entry of Gateway's.
func dshEntryName(layout dshLayout) (string, error) {
	_, patches, err := loadDshPatches(layout.patchPath)
	if err != nil {
		return "", err
	}
	for _, patch := range patches.Content {
		inserts := yamledit.Get(patch, "insert")
		if inserts == nil || inserts.Kind != yaml.SequenceNode {
			continue
		}
		for _, row := range inserts.Content {
			if yamledit.String(row, "id") == gatewayEntryName {
				return yamledit.String(row, "name"), nil
			}
		}
	}
	return "", nil
}

func loadDshPatches(path string) (*yamledit.Document, *yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	document, err := yamledit.Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	patches, err := document.Sequence()
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, patches, nil
}

func saveDshPatches(path string, document *yamledit.Document) error {
	data, err := document.Bytes()
	if err != nil {
		return err
	}
	// A list Gateway created and has just emptied is removed rather than left
	// behind. One the operator wrote keeps its comments, and so keeps its file.
	if strings.TrimSpace(string(data)) == "[]" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
