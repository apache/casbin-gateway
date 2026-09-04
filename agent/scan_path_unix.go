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

//go:build !windows

package agent

import (
	"os"
	"path/filepath"
)

// pathDirs are the absolute directories PATH names, in its own order.
func pathDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		dir = filepath.Clean(dir)
		if !filepath.IsAbs(dir) || seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	return dirs
}

// osOwnedDirs are what the system package manager owns. "/usr/local/bin" is not
// one of them: that is where a person's own install goes.
var osOwnedDirs = map[string]bool{
	"/bin": true, "/sbin": true, "/usr/bin": true, "/usr/sbin": true,
}

func ownedByOS(dir string) bool {
	return osOwnedDirs[filepath.ToSlash(dir)]
}

func isPathExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}
