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
	"context"
	"os"
	"path/filepath"
	"strings"
)

// maxNvmSettingsSize limits reads from an nvm settings file.
const maxNvmSettingsSize = 64 * 1024

// scanMachineNpm reports a package installed into a Node that lives outside
// every profile, which the per-home npm layouts never reach.
func scanMachineNpm(ctx context.Context, fingerprint *Fingerprint, roots []string, owner string) []Installation {
	pkg := fingerprint.npmPackagePath()
	patterns := make([]string, 0, len(roots))
	for _, root := range roots {
		patterns = append(patterns, filepath.Join(root, "node_modules", pkg, "package.json"))
	}
	return scanNpmPatterns(ctx, fingerprint, patterns, owner, nil)
}

// windowsNodeRoots are the global package directories of every Node installed
// outside a user profile. nvm-windows is why they exist: it keeps one directory
// per version it manages, current installers put those under ProgramData, and a
// global npm install lands in whichever version is in use rather than in any
// AppData prefix.
func windowsNodeRoots() []string {
	nvmDirs := []string{os.Getenv("NVM_HOME")}
	for _, base := range []string{
		os.Getenv("ProgramData"), os.Getenv("ALLUSERSPROFILE"),
		os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"),
		os.Getenv("LOCALAPPDATA"), os.Getenv("APPDATA"),
	} {
		if base != "" {
			nvmDirs = append(nvmDirs, filepath.Join(base, "nvm"))
		}
	}

	roots := []string{os.Getenv("NVM_SYMLINK")}
	for _, dir := range pathDirs() {
		// A version manager on PATH is on it as its own root, and a Node there
		// is one no layout above describes: unpacked by hand, or put there by a
		// manager Gateway does not know.
		if isFile(filepath.Join(dir, "nvm.exe")) {
			nvmDirs = append(nvmDirs, dir)
		}
		if isFile(filepath.Join(dir, "node.exe")) {
			roots = append(roots, dir)
		}
	}
	for _, dir := range nvmDirs {
		roots = append(roots, nvmNodeDirs(dir)...)
	}
	return existingDirs(roots)
}

// nvmNodeDirs are the Node directories one nvm-windows install holds: one per
// version, plus the link the version in use is exposed through. A root the
// install was moved to is recorded in the settings file beside it.
func nvmNodeDirs(dir string) []string {
	if dir == "" {
		return nil
	}
	root, symlink := nvmSettings(filepath.Join(dir, "settings.txt"))
	if root == "" {
		root = dir
	}
	versions, _ := filepath.Glob(filepath.Join(root, "*"))
	return append(versions, symlink)
}

// nvmSettings reads the root and the symlink an nvm-windows install records.
func nvmSettings(path string) (root, symlink string) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxNvmSettingsSize {
		return "", ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "root":
			root = strings.TrimSpace(value)
		case "path":
			symlink = strings.TrimSpace(value)
		}
	}
	return root, symlink
}

// existingDirs keeps the absolute directories of a candidate list, once each.
func existingDirs(paths []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, path := range paths {
		path = strings.Trim(strings.TrimSpace(path), `"`)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if !filepath.IsAbs(path) {
			continue
		}
		key := strings.ToLower(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			result = append(result, path)
		}
	}
	return result
}
