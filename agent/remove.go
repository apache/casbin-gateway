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

package agent

import (
	"path/filepath"
	"strings"
)

// sharedInstallDirs are the declared directories an agent shares with whatever
// else the host put there. A program in one of those is removed on its own; the
// directory around it belongs to nobody in particular.
var sharedInstallDirs = map[string]bool{
	".local/bin": true,
	"bin":        true,
	".local":     true,
	"usr/bin":    true,
	"usr/local":  true,
}

// RemovablePathOf is what an uninstall may delete for an installation no
// package manager and no registered uninstaller owns: the directory the
// fingerprint declares for this agent, or, where that directory is shared, the
// launcher alone. The second result marks which of the two it is. An agent's
// state directory is never part of the answer.
// A program someone pointed Gateway at is never deleted: forgetting the row is
// what removing it means.
func RemovablePathOf(installation Installation) (string, bool) {
	if installation.Path == "" || installation.InstallMethod == InstallMethodConfig ||
		installation.InstallMethod == InstallMethodManual {
		return "", false
	}

	fingerprint := fingerprintOf(installation.AgentId)
	if fingerprint == nil {
		return installation.Path, false
	}

	directory := filepath.Dir(installation.Path)
	for _, declared := range declaredInstallDirs(fingerprint) {
		if !endsWithDir(directory, declared) {
			continue
		}
		if sharedInstallDirs[strings.ToLower(declared)] {
			return installation.Path, false
		}
		return directory, true
	}
	return installation.Path, false
}

func fingerprintOf(id string) *Fingerprint {
	for i := range fingerprints {
		if fingerprints[i].ID == id {
			return &fingerprints[i]
		}
	}
	return nil
}

// declaredInstallDirs are every layout a fingerprint says this agent's program
// is installed into, relative to whichever root the scan looked under.
func declaredInstallDirs(fingerprint *Fingerprint) []string {
	dirs := make([]string, 0, 8)
	dirs = append(dirs, fingerprint.HomeDirs...)
	dirs = append(dirs, fingerprint.WindowsUserDirs...)
	dirs = append(dirs, fingerprint.WindowsProgramDirs...)
	if fingerprint.DesktopInstallerDir != "" {
		dirs = append(dirs, fingerprint.DesktopInstallerDir)
	}
	return dirs
}

// endsWithDir reports whether a directory ends in the declared relative path,
// matching whole segments so that "cursor" never matches "cursor-agent".
func endsWithDir(directory string, declared string) bool {
	want := splitSegments(filepath.FromSlash(declared))
	have := splitSegments(directory)
	if len(want) == 0 || len(want) > len(have) {
		return false
	}
	have = have[len(have)-len(want):]
	for i := range want {
		if !strings.EqualFold(want[i], have[i]) {
			return false
		}
	}
	return true
}

func splitSegments(path string) []string {
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))
	kept := parts[:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return kept
}
