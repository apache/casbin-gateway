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

//go:build darwin

package agent

import (
	"context"
	"os"
	"os/user"
	"path/filepath"

	"github.com/apache/casbin-gateway/internal/hermes"
)

// scan finds installations of every known agent in known macOS layouts. It
// neither executes discovered binaries nor walks arbitrary filesystem roots.
func scan(ctx context.Context) []Installation {
	homes := darwinHomes()
	paths := newPathIndex()

	var installations []Installation
	for i := range fingerprints {
		fingerprint := &fingerprints[i]
		if ctx.Err() != nil {
			return installations
		}
		mark := len(installations)
		for _, home := range homes {
			if ctx.Err() != nil {
				return installations
			}
			installations = append(installations, scanNative(fingerprint, home)...)
			installations = append(installations, scanHomeDirs(fingerprint, home)...)
			installations = append(installations, scanNpmPatterns(ctx, fingerprint, userNpmPatterns(fingerprint, home.path), home.owner, fileOwner)...)
		}
		for _, prefix := range []string{"/opt/homebrew", "/usr/local"} {
			installations = append(installations, scanDarwinHomebrew(fingerprint, prefix)...)
			installations = append(installations, scanDarwinSystemNpm(ctx, fingerprint, prefix)...)
		}
		installations = append(installations, scanPathDirs(fingerprint, paths, installations[mark:])...)
		stampAgentId(installations, mark, fingerprint.ID)
		installations = append(installations, scanStateDirs(fingerprint, homes, installations[mark:])...)
		fillMissingVersions(installations, mark, fingerprint)
	}
	for _, home := range homes {
		if ctx.Err() != nil {
			return installations
		}
		installations = append(installations, scanHermesUnix(home, filepath.Join(home.path, ".local", "bin", hermes.ExecName))...)
	}
	installations = append(installations, scanHermesOnPath()...)
	installations = append(installations, scanCodexStandalone()...)
	installations = append(installations, scanCodexDarwinApps(homes)...)
	// Last, so that an agent found both on disk and by its port keeps the
	// richer install-layout row when the two resolve to the same executable.
	installations = append(installations, scanLocalServers(ctx)...)
	// After every layout, so a chosen program yields to the same one found where
	// its installer puts it.
	installations = append(installations, ManualInstallations()...)
	result := expandSharedCodexInstallations(dedupeInstallations(installations), homes)
	fillAccounts(result, homes)
	return result
}

func darwinHomes() []homeDir {
	seen := map[string]bool{}
	var homes []homeDir
	add := func(owner, path string) {
		path = filepath.Clean(path)
		if info, err := os.Stat(path); err != nil || !info.IsDir() || seen[path] {
			return
		}
		seen[path] = true
		if owner == "" {
			owner = filepath.Base(path)
		}
		homes = append(homes, homeDir{owner: owner, path: path})
	}

	if entries, err := os.ReadDir("/Users"); err == nil {
		for _, entry := range entries {
			if entry.Name() != "Shared" {
				add(entry.Name(), filepath.Join("/Users", entry.Name()))
			}
		}
	}
	add("root", "/var/root")
	if home, err := os.UserHomeDir(); err == nil {
		owner := ""
		if account, err := user.Current(); err == nil {
			owner = account.Username
		}
		add(owner, home)
	}
	return homes
}

// scanDarwinHomebrew reads the Caskroom layout directly; on macOS every cask
// version keeps its own directory, so no brew invocation is needed.
func scanDarwinHomebrew(fingerprint *Fingerprint, prefix string) []Installation {
	if fingerprint.ExecName == "" {
		return nil
	}

	var result []Installation
	for _, cask := range fingerprint.HomebrewCasks {
		versions, _ := filepath.Glob(filepath.Join(prefix, "Caskroom", cask, "*"))
		for _, versionDir := range versions {
			executable := filepath.Join(versionDir, fingerprint.ExecName)
			info, err := os.Stat(executable)
			if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
				continue
			}
			result = append(result, Installation{
				Name: fingerprint.DisplayName, Version: filepath.Base(versionDir), Path: executable,
				InstallMethod: "homebrew", Owner: fileOwner(executable),
			})
		}
	}
	return result
}

func scanDarwinSystemNpm(ctx context.Context, fingerprint *Fingerprint, prefix string) []Installation {
	pattern := filepath.Join(prefix, "lib", "node_modules", fingerprint.npmPackagePath(), "package.json")
	return scanNpmPatterns(ctx, fingerprint, []string{pattern}, "", fileOwner)
}
