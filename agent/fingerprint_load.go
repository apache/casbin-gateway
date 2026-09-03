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

package agent

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	pathpkg "path"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/localserver"
)

// Fingerprints are embedded in the Gateway binary.
//
//go:embed fingerprints/*.json
var fingerprintFS embed.FS

const fingerprintDir = "fingerprints"

var fingerprints = mustLoadFingerprints(fingerprintFS)

// mustLoadFingerprints rejects invalid embedded build data at startup.
func mustLoadFingerprints(files fs.FS) []Fingerprint {
	loaded, err := loadFingerprints(files)
	if err != nil {
		panic("agent: cannot load agent fingerprints: " + err.Error())
	}
	return loaded
}

// loadFingerprints loads and validates fingerprint files in stable order.
func loadFingerprints(files fs.FS) ([]Fingerprint, error) {
	names, err := fs.Glob(files, fingerprintDir+"/*.json")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no fingerprint files under %s/", fingerprintDir)
	}

	loaded := make([]Fingerprint, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		fingerprint, err := decodeFingerprint(files, name)
		if err != nil {
			return nil, err
		}
		if seen[fingerprint.ID] {
			return nil, fmt.Errorf("%s: duplicate agent id %q", name, fingerprint.ID)
		}
		seen[fingerprint.ID] = true
		loaded = append(loaded, fingerprint)
	}
	return loaded, nil
}

// decodeFingerprint rejects unknown JSON fields.
func decodeFingerprint(files fs.FS, name string) (Fingerprint, error) {
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return Fingerprint{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var fingerprint Fingerprint
	if err := decoder.Decode(&fingerprint); err != nil {
		return Fingerprint{}, fmt.Errorf("%s: %w", name, err)
	}
	if decoder.More() {
		return Fingerprint{}, fmt.Errorf("%s: trailing content after the fingerprint object", name)
	}
	if err := validateFingerprint(fingerprint, name); err != nil {
		return Fingerprint{}, fmt.Errorf("%s: %w", name, err)
	}
	return fingerprint, nil
}

// validateFingerprint requires a usable detection source.
func validateFingerprint(f Fingerprint, name string) error {
	if id := strings.TrimSuffix(pathpkg.Base(name), ".json"); f.ID != id {
		return fmt.Errorf("id %q does not match the file name %q", f.ID, id)
	}
	if f.DisplayName == "" {
		return errors.New("displayName is required")
	}
	// The install page is rendered as a link, so nothing but the web schemes.
	if f.InstallUrl != "" && !strings.HasPrefix(f.InstallUrl, "https://") && !strings.HasPrefix(f.InstallUrl, "http://") {
		return fmt.Errorf("installUrl %q is not an http(s) URL", f.InstallUrl)
	}

	if f.StateVersionGlob != "" && f.StateDir == "" {
		return errors.New("stateVersionGlob has no stateDir to resolve against")
	}

	if err := validateLocalServer(f.LocalServer); err != nil {
		return err
	}
	if !hasScanSource(f) {
		return errors.New("no supported installation layout or local server")
	}
	return nil
}

func hasScanSource(f Fingerprint) bool {
	if f.LocalServer != nil || f.CustomScan {
		return true
	}
	if f.ExecName == "" {
		return false
	}
	return f.StateDir != "" || f.NpmPackage != "" || f.WingetPackage != "" ||
		f.MSIXFamily != "" || f.DesktopInstallerDir != "" ||
		len(f.WindowsProgramDirs) != 0 || len(f.WindowsUserDirs) != 0 || len(f.HomeDirs) != 0 ||
		len(f.HomebrewCasks) != 0 || f.SystemPackage != ""
}

// validateLocalServer rejects incomplete probes.
func validateLocalServer(server *localserver.Server) error {
	if server == nil {
		return nil
	}
	if len(server.Ports) == 0 {
		return errors.New("localServer.ports is required")
	}
	if server.ProbePath == "" || len(server.ProbeMarkers) == 0 {
		return errors.New("localServer needs both probePath and probeMarkers to confirm anything")
	}
	for i, marker := range server.ProbeMarkers {
		if strings.TrimSpace(marker) == "" {
			return fmt.Errorf("localServer.probeMarkers[%d] is empty, which confirms nothing", i)
		}
	}
	return nil
}
