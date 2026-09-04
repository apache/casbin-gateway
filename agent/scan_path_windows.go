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

//go:build windows

package agent

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// pathDirs are this process's PATH first, then the ones Windows records for the
// account and the machine: a directory added after the last sign-in is on the
// stored PATH while being on no environment Gateway inherited.
func pathDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(value string) {
		for _, dir := range filepath.SplitList(value) {
			dir = strings.Trim(strings.TrimSpace(dir), `"`)
			if dir == "" {
				continue
			}
			if expanded, err := registry.ExpandString(dir); err == nil {
				dir = expanded
			}
			dir = filepath.Clean(os.ExpandEnv(dir))
			if !filepath.IsAbs(dir) {
				continue
			}
			key := strings.ToLower(dir)
			if seen[key] {
				continue
			}
			seen[key] = true
			dirs = append(dirs, dir)
		}
	}

	add(os.Getenv("PATH"))
	add(storedPath(registry.CURRENT_USER, `Environment`))
	add(storedPath(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`))
	return dirs
}

func storedPath(root registry.Key, path string) string {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	value, _, err := key.GetStringValue("Path")
	if err != nil {
		return ""
	}
	return value
}

// ownedByOS covers the Windows directory, which belongs to the system.
func ownedByOS(dir string) bool {
	root := os.Getenv("SystemRoot")
	if root == "" {
		return false
	}
	root = filepath.Clean(root) + string(filepath.Separator)
	return strings.HasPrefix(strings.ToLower(dir)+string(filepath.Separator), strings.ToLower(root))
}

func isPathExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
