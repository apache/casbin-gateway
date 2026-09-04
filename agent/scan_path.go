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
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallMethodPath marks an agent found by name on PATH, unpacked by hand into
// a directory no fingerprint can describe.
const InstallMethodPath = "path"

// pathIndex maps a lower-case launcher name to the first copy on PATH.
type pathIndex map[string]string

// newPathIndex reads PATH once for a whole scan.
func newPathIndex() pathIndex {
	wanted := map[string]bool{}
	for i := range fingerprints {
		for _, name := range pathExecNames(&fingerprints[i]) {
			wanted[name] = true
		}
	}
	if len(wanted) == 0 {
		return pathIndex{}
	}

	index := pathIndex{}
	for _, dir := range pathDirs() {
		// Nobody unpacks an agent into the system's own program directories, and
		// an unrelated program of the same short name does live there.
		if ownedByOS(dir) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := strings.ToLower(entry.Name())
			if entry.IsDir() || !wanted[name] || index[name] != "" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if isPathExecutable(path) {
				index[name] = path
			}
		}
	}
	return index
}

// pathExecNames are the names one launcher goes by, program before shim. A
// windowed app is started from a menu, and an agent with a scan of its own is
// left to it: that scan reads the program before believing its name.
func pathExecNames(fingerprint *Fingerprint) []string {
	if fingerprint.ExecName == "" || fingerprint.Desktop || fingerprint.CustomScan {
		return nil
	}
	name := strings.ToLower(fingerprint.ExecName)
	if runtime.GOOS != "windows" {
		return []string{name}
	}
	return []string{name + ".exe", name + ".cmd", name + ".bat", name + ".com"}
}

// scanPathDirs reports an agent found on PATH, only where every other layout
// came up empty: PATH cannot say which manager owns a tree. Nothing is executed,
// so a program is identified by its name alone.
func scanPathDirs(fingerprint *Fingerprint, index pathIndex, found []Installation) []Installation {
	if len(found) > 0 {
		return nil
	}
	for _, name := range pathExecNames(fingerprint) {
		path := index[name]
		if path == "" {
			continue
		}
		return []Installation{{
			AgentId: fingerprint.ID, Name: fingerprint.DisplayName, Path: path,
			InstallMethod: InstallMethodPath, Owner: currentOwner(),
		}}
	}
	return nil
}

// currentOwner is the account Gateway runs as, whose PATH it reads.
func currentOwner() string {
	account, err := user.Current()
	if err != nil {
		return ""
	}
	return account.Username
}
