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
	"debug/buildinfo"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxVersionFileSize limits reads from version files.
const maxVersionFileSize = 4 * 1024

var pseudoVersion = regexp.MustCompile(`\d{14}-[0-9a-f]{12}`)

var describeSuffix = regexp.MustCompile(`-\d+-g[0-9a-f]+$`)

// executableBuildVersion reads release metadata without running the binary.
func executableBuildVersion(path, module, versionVar string) string {
	if path == "" || module == "" {
		return ""
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil || info == nil || info.Main.Path != module {
		return ""
	}
	if versionVar != "" {
		for _, setting := range info.Settings {
			if setting.Key != "-ldflags" {
				continue
			}
			if version := ldflagsVersion(setting.Value, versionVar); version != "" {
				return version
			}
		}
	}
	return cleanReleaseVersion(info.Main.Version)
}

func ldflagsVersion(ldflags, versionVar string) string {
	prefix := versionVar + "="
	for _, field := range strings.Fields(ldflags) {
		field = strings.TrimPrefix(field, "-X=")
		if value, ok := strings.CutPrefix(field, prefix); ok {
			return sanitizeVersion(value)
		}
	}
	return ""
}

func cleanReleaseVersion(version string) string {
	version = sanitizeVersion(version)
	if version == "" || strings.Contains(version, "+") || pseudoVersion.MatchString(version) {
		return ""
	}
	return version
}

func sanitizeVersion(version string) string {
	switch version = strings.TrimSpace(version); version {
	case "", "dev", "unknown", "(devel)":
		return ""
	}
	if len(version) > 1 && version[0] == 'v' && version[1] >= '0' && version[1] <= '9' {
		version = version[1:]
	}
	version = strings.TrimSuffix(version, "-dirty")
	version = describeSuffix.ReplaceAllString(version, "")
	return version
}

// executableVersionFile reads a small version file next to the binary.
func executableVersionFile(binaryPath, fileName string) string {
	if binaryPath == "" || fileName == "" {
		return ""
	}
	resolved := binaryPath
	if target, err := filepath.EvalSymlinks(binaryPath); err == nil {
		resolved = target
	}
	path := filepath.Join(filepath.Dir(resolved), fileName)

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxVersionFileSize {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if newline := bytes.IndexByte(data, '\n'); newline >= 0 {
		data = data[:newline]
	}
	return sanitizeVersion(string(data))
}

func fillMissingVersions(installations []Installation, mark int, fingerprint *Fingerprint) {
	if fingerprint.BuildInfoModule == "" && fingerprint.VersionFile == "" {
		return
	}
	for i := mark; i < len(installations); i++ {
		if installations[i].Version != "" {
			continue
		}
		version := executableBuildVersion(
			installations[i].Path, fingerprint.BuildInfoModule, fingerprint.BuildInfoVersionVar)
		if version == "" {
			version = executableVersionFile(installations[i].Path, fingerprint.VersionFile)
		}
		installations[i].Version = version
	}
}
