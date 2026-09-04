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
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/apache/casbin-gateway/agenthome"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/internal/hermes"
)

const (
	hermesPluginName      = gatewayEntryName
	hermesConfigSchema    = 1
	minHermesAgentVersion = "0.19.0"
)

//go:embed hermes_observer.py
var hermesObserver []byte

//go:embed hermes_plugin.yaml
var hermesPluginManifest []byte

// hermesPluginVersion comes from the manifest so the two never drift apart.
var hermesPluginVersion = hermesManifestVersion()

func hermesManifestVersion() string {
	var manifest struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(hermesPluginManifest, &manifest); err != nil {
		return ""
	}
	return manifest.Version
}

type hermesPatcher struct{}

type hermesLayout struct {
	home       string
	configPath string
	pluginDir  string
}

type hermesObserverConfig struct {
	SchemaVersion     int    `json:"schemaVersion"`
	Owner             string `json:"owner"`
	PluginVersion     string `json:"pluginVersion"`
	RecordsURL        string `json:"recordsUrl"`
	AgentPath         string `json:"agentPath"`
	User              string `json:"user"`
	IngestToken       string `json:"ingestToken"`
	IngestTokenHeader string `json:"ingestTokenHeader"`
}

func init() {
	register(hermesPatcher{})
}

func (hermesPatcher) AgentId() string { return hermes.AgentID }

func (hermesPatcher) Supported() bool { return true }

func (p hermesPatcher) Patch(target Target) error {
	layout, err := p.validateTarget(target)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(layout.pluginDir); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists and is not a directory", layout.pluginDir)
		}
		// What the directory holds decides this, not whether this Gateway's own
		// state remembers writing it. A reinstall or a move to another data
		// directory leaves a previous Gateway's observer behind, and refusing
		// that one left Hermes unmonitorable with nothing to do about it.
		if hermesPluginForeign(layout.pluginDir) {
			return fmt.Errorf("%s holds an observer that is not Gateway's; remove that directory to let Gateway install its own", layout.pluginDir)
		}
	} else if !os.IsNotExist(statErr) {
		return statErr
	}

	token, err := IssueIngestToken(target)
	if err != nil {
		return err
	}
	observerConfig, err := renderHermesObserverConfig(target, token)
	if err != nil {
		return errors.Join(err, RevokeIngestToken(target))
	}
	pluginFiles := []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{"plugin.yaml", hermesPluginManifest, 0o644},
		{"__init__.py", hermesObserver, 0o644},
		{"gateway.json", observerConfig, 0o600},
	}

	if err := Apply(target, func(changes *ChangeSet) error {
		if err := changes.MkdirAll(layout.pluginDir); err != nil {
			return err
		}
		for _, file := range pluginFiles {
			path := filepath.Join(layout.pluginDir, file.name)
			if err := changes.WriteFile(path, file.data, file.mode); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return errors.Join(err, RevokeIngestToken(target))
	}

	// config.yaml stays out of the backup journal: Unpatch deletes the one
	// entry Gateway added rather than restore a snapshot of the user's file.
	if err := writeHermesPluginMembership(layout.configPath, true); err != nil {
		return errors.Join(err, Revert(target), RevokeIngestToken(target))
	}
	return nil
}

func (p hermesPatcher) Unpatch(target Target) error {
	// Revert works from the saved paths, so a home directory that no longer
	// resolves still gets its plugin removed.
	var membershipErr error
	if layout, err := p.layoutOf(target); err == nil {
		membershipErr = writeHermesPluginMembership(layout.configPath, false)
	}
	return errors.Join(membershipErr, Revert(target), RevokeIngestToken(target))
}

// PatchNotice is the copy the agents table shows before and after the button,
// so the Web UI does not need a branch of its own for how Hermes loads plugins.
func (hermesPatcher) PatchNotice(patched bool) (string, string) {
	if patched {
		return "Removes the behaviour observer from the default Hermes profile and drops its entry from plugins.enabled. Nothing else in config.yaml changes.",
			"Restart running Hermes processes to unload it."
	}
	return "Installs a behaviour observer with redacted tool inputs in the default Hermes profile and adds it to plugins.enabled. Named profiles are left alone.",
		"Restart running Hermes processes to load it."
}

func (p hermesPatcher) Status(target Target) (Status, error) {
	layout, err := p.layoutOf(target)
	if err != nil {
		return Status{}, err
	}
	config, owned, err := readHermesObserverConfig(layout.pluginDir)
	if err != nil {
		return Status{}, err
	}
	if !owned {
		if _, statErr := os.Stat(layout.pluginDir); statErr == nil {
			return Status{Detail: "plugin directory exists but is not owned by Gateway"}, nil
		}
		return Status{Detail: "not patched"}, nil
	}

	issuedAgent, tokenValid := ValidateIngestToken(config.IngestToken)
	if !tokenValid || !strings.EqualFold(issuedAgent, target.AgentId) {
		return Status{Detail: "observer files are stale or incomplete; Patch again to refresh them"}, nil
	}
	expectedConfig, err := renderHermesObserverConfig(target, config.IngestToken)
	if err != nil {
		return Status{}, err
	}
	expectedFiles := map[string][]byte{
		"plugin.yaml": hermesPluginManifest, "__init__.py": hermesObserver,
		"gateway.json": expectedConfig,
	}
	for name, expected := range expectedFiles {
		actual, readErr := os.ReadFile(filepath.Join(layout.pluginDir, name))
		if readErr != nil || !bytes.Equal(actual, expected) {
			return Status{Detail: "observer files are stale or incomplete; Patch again to refresh them"}, nil
		}
	}
	enabled, disabled, err := hermesPluginMembership(layout.configPath)
	if err != nil {
		return Status{}, err
	}
	if !enabled || disabled {
		return Status{Detail: "observer is installed but not enabled in config.yaml; Patch again to repair it"}, nil
	}
	if !IsApplied(target) {
		return Status{
			Patched: true,
			Detail:  "observer is enabled outside this Gateway patch state; an automatic Unpatch is unsafe",
		}, nil
	}
	return Status{
		Patched: true,
		Detail: fmt.Sprintf(
			"behaviour observer installed in %s; start a new Hermes process or restart a running one to load it",
			layout.home,
		),
	}, nil
}

func (p hermesPatcher) validateTarget(target Target) (hermesLayout, error) {
	layout, err := p.layoutOf(target)
	if err != nil {
		return hermesLayout{}, err
	}
	roots := []string{filepath.Join(layout.home, hermes.ProjectDir)}
	if runtime.GOOS == "linux" && filepath.Clean(target.Path) == "/usr/local/bin/hermes" {
		roots = append(roots, "/usr/local/lib/hermes-agent", "/root/.hermes/hermes-agent")
	}
	project, err := hermes.InspectLauncher(target.Path, roots...)
	if err != nil {
		return hermesLayout{}, err
	}
	// A hook Hermes does not know simply never fires, so the version gate is
	// the only compatibility check the observer needs.
	if !hermes.VersionAtLeast(project.Version, minHermesAgentVersion) {
		return hermesLayout{}, fmt.Errorf(
			"Hermes Agent %s is incompatible; upgrade to %s or later",
			project.Version, minHermesAgentVersion,
		)
	}
	return layout, nil
}

func (hermesPatcher) layoutOf(target Target) (hermesLayout, error) {
	home, err := agenthome.Resolve(target.Owner)
	if err != nil {
		return hermesLayout{}, err
	}
	hermesHome := hermes.Home(home)
	return hermesLayout{
		home:       hermesHome,
		configPath: filepath.Join(hermesHome, "config.yaml"),
		pluginDir:  filepath.Join(hermesHome, "plugins", hermesPluginName),
	}, nil
}

func renderHermesObserverConfig(target Target, token string) ([]byte, error) {
	url, err := recordsURL()
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(hermesObserverConfig{
		SchemaVersion:     hermesConfigSchema,
		Owner:             "casbin-gateway",
		PluginVersion:     hermesPluginVersion,
		RecordsURL:        url,
		AgentPath:         target.Path,
		User:              target.Owner,
		IngestToken:       token,
		IngestTokenHeader: agentmonitor.IngestTokenHeader,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// hermesPluginForeign reports a plugin directory holding somebody else's
// gateway.json. A missing, unreadable or stale-schema file is not evidence of
// that, so a refresh can still repair it.
func hermesPluginForeign(pluginDir string) bool {
	data, err := os.ReadFile(filepath.Join(pluginDir, "gateway.json"))
	if err != nil {
		return false
	}
	var config hermesObserverConfig
	if json.Unmarshal(data, &config) != nil {
		return false
	}
	return config.Owner != "casbin-gateway"
}

func readHermesObserverConfig(pluginDir string) (hermesObserverConfig, bool, error) {
	data, err := os.ReadFile(filepath.Join(pluginDir, "gateway.json"))
	if os.IsNotExist(err) {
		return hermesObserverConfig{}, false, nil
	}
	if err != nil {
		return hermesObserverConfig{}, false, err
	}
	var config hermesObserverConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return hermesObserverConfig{}, false, nil
	}
	return config, config.Owner == "casbin-gateway" &&
		config.SchemaVersion == hermesConfigSchema, nil
}
