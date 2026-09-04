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
	"runtime"
	"strconv"
	"strings"
)

// InstallMethodBundled marks a copy of an agent that another app downloaded for
// its own use, under that app's data directory. It runs like any other copy,
// but the app that put it there is what updates and removes it.
const InstallMethodBundled = "bundled"

// scanBundledDirs reports such a copy. Without it a desktop app that ships the
// CLI leaves nothing but the state directory the CLI writes, and an agent used
// every day is listed as one Gateway cannot start.
func scanBundledDirs(fingerprint *Fingerprint, home homeDir) []Installation {
	root := appDataRoot(home)
	if fingerprint.ExecName == "" || root == "" {
		return nil
	}

	name := fingerprint.ExecName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	var result []Installation
	for _, dir := range fingerprint.BundledDirs {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(dir), name))
		launcher, version := newestRelease(matches)
		if launcher == "" {
			continue
		}
		result = append(result, Installation{
			Name: fingerprint.DisplayName, Version: version, Path: launcher,
			InstallMethod: InstallMethodBundled, Owner: home.owner,
		})
	}
	return result
}

// newestRelease picks the launcher with the highest version among copies these
// apps keep one directory of per release, which is the one the app itself runs.
func newestRelease(paths []string) (string, string) {
	launcher, version := "", ""
	for _, path := range paths {
		if !isPathExecutable(path) {
			continue
		}
		release := releaseVersion(filepath.Base(filepath.Dir(path)))
		if launcher == "" || compareReleases(release, version) > 0 {
			launcher, version = path, release
		}
	}
	return launcher, version
}

// releaseVersion is the version a release directory is named after. A launcher
// sitting in a directory named anything else has no version to read here.
func releaseVersion(name string) string {
	version := sanitizeVersion(name)
	if version == "" || version[0] < '0' || version[0] > '9' {
		return ""
	}
	return version
}

// compareReleases orders two versions by their numbers, left to right.
func compareReleases(left string, right string) int {
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	for i := 0; i < len(leftParts) || i < len(rightParts); i++ {
		if diff := releaseSegment(leftParts, i) - releaseSegment(rightParts, i); diff != 0 {
			return diff
		}
	}
	return 0
}

func releaseSegment(parts []string, index int) int {
	if index >= len(parts) {
		return 0
	}
	number, _ := strconv.Atoi(parts[index])
	return number
}
