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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// maxVersionFileSize limits reads from version files.
const maxVersionFileSize = 4 * 1024

// maxStateVersionFiles is how many of the newest matches are opened, in case
// the newest one was cut short before it recorded anything.
const maxStateVersionFiles = 3

// maxStateVersionHead is how much of a record file's start is read. The version
// sits on the first records, and a later one can be megabytes of tool output.
const maxStateVersionHead = 64 * 1024

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

// stateDirVersion reads the version out of the newest records the agent wrote
// under its state directory. An installation found by its configuration alone
// has no program to read a version from, but its own records name one.
func stateDirVersion(stateDir, glob, field string) string {
	if stateDir == "" || glob == "" {
		return ""
	}
	if field == "" {
		field = "version"
	}
	matches, err := filepath.Glob(filepath.Join(stateDir, filepath.FromSlash(glob)))
	if err != nil {
		return ""
	}

	type candidate struct {
		path string
		info os.FileInfo
	}
	files := make([]candidate, 0, len(matches))
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		files = append(files, candidate{path: match, info: info})
	}
	sort.SliceStable(files, func(left, right int) bool {
		return files[left].info.ModTime().After(files[right].info.ModTime())
	})
	if len(files) > maxStateVersionFiles {
		files = files[:maxStateVersionFiles]
	}

	for _, file := range files {
		if version := recordVersion(file.path, field); version != "" {
			return version
		}
	}
	return ""
}

// recordVersion is the version field of the first JSON record that has one.
func recordVersion(path, field string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	head := make([]byte, maxStateVersionHead)
	read, err := io.ReadFull(file, head)
	if read == 0 || (err != nil && err != io.EOF && err != io.ErrUnexpectedEOF) {
		return ""
	}
	head = head[:read]
	// The last line is dropped unless the read stopped on a record boundary:
	// what follows a full buffer is half a record.
	if end := bytes.LastIndexByte(head, '\n'); end >= 0 {
		head = head[:end]
	} else if err == nil {
		return ""
	}

	segments := strings.Split(field, ".")
	for _, line := range bytes.Split(head, []byte("\n")) {
		var record map[string]json.RawMessage
		if json.Unmarshal(bytes.TrimSpace(line), &record) != nil {
			continue
		}
		if version := sanitizeVersion(recordField(record, segments)); version != "" {
			return version
		}
	}
	return ""
}

// recordField walks a dotted path into one record, and is empty unless the path
// ends on a string.
func recordField(record map[string]json.RawMessage, segments []string) string {
	for index, segment := range segments {
		raw, ok := record[segment]
		if !ok {
			return ""
		}
		if index == len(segments)-1 {
			var value string
			if json.Unmarshal(raw, &value) != nil {
				return ""
			}
			return value
		}
		record = nil
		if json.Unmarshal(raw, &record) != nil {
			return ""
		}
	}
	return ""
}

func fillMissingVersions(installations []Installation, mark int, fingerprint *Fingerprint) {
	if fingerprint.BuildInfoModule == "" && fingerprint.VersionFile == "" && fingerprint.StateVersionGlob == "" {
		return
	}
	for i := mark; i < len(installations); i++ {
		if installations[i].Version != "" {
			continue
		}
		if installations[i].InstallMethod == InstallMethodConfig {
			installations[i].Version = stateDirVersion(
				installations[i].Path, fingerprint.StateVersionGlob, fingerprint.StateVersionField)
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
