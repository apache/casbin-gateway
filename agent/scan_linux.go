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

//go:build linux

package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/internal/hermes"
)

// scan finds installations of every known agent without executing any
// discovered binary or traversing arbitrary filesystem roots.
func scan(ctx context.Context) []Installation {
	homes := readHomes("/etc/passwd")
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
		installations = append(installations, scanSystemNpm(ctx, fingerprint)...)
		installations = append(installations, scanSystemPackages(ctx, fingerprint)...)
		installations = append(installations, scanHomebrew(ctx, fingerprint)...)
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
	installations = append(installations, scanHermesUnix(
		homeDir{owner: "root", path: "/root"},
		filepath.Join("/usr/local/bin", hermes.ExecName),
		filepath.Join("/usr/local/lib", hermes.ProjectDir),
		filepath.Join("/root/.hermes", hermes.ProjectDir),
	)...)
	installations = append(installations, scanHermesOnPath()...)
	installations = append(installations, scanCodexStandalone()...)
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

func readHomes(passwdPath string) []homeDir {
	data, err := os.ReadFile(passwdPath)
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var homes []homeDir
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 6 || !filepath.IsAbs(fields[5]) || seen[fields[5]] {
			continue
		}
		info, err := os.Stat(fields[5])
		if err != nil || !info.IsDir() {
			continue
		}
		seen[fields[5]] = true
		homes = append(homes, homeDir{owner: fields[0], path: fields[5]})
	}
	return homes
}

func scanSystemNpm(ctx context.Context, fingerprint *Fingerprint) []Installation {
	pkg := fingerprint.npmPackagePath()
	return scanNpmPatterns(ctx, fingerprint, []string{
		filepath.Join("/usr/local/lib/node_modules", pkg, "package.json"),
		filepath.Join("/usr/lib/node_modules", pkg, "package.json"),
	}, "", fileOwner)
}

// scanSystemPackages queries each distro package manager for the agent, which
// reports both its version and the files it owns.
func scanSystemPackages(ctx context.Context, fingerprint *Fingerprint) []Installation {
	pkg := fingerprint.SystemPackage
	if pkg == "" {
		return nil
	}

	var result []Installation
	if version, ok := commandOutput(ctx, "dpkg-query", "-W", "-f=${Version}", pkg); ok {
		files, _ := commandOutput(ctx, "dpkg", "-L", pkg)
		result = append(result, packageInstallation(fingerprint, "apt", version, files))
	}
	if version, ok := commandOutput(ctx, "rpm", "-q", "--qf", "%{VERSION}-%{RELEASE}", pkg); ok {
		files, _ := commandOutput(ctx, "rpm", "-ql", pkg)
		result = append(result, packageInstallation(fingerprint, "rpm", version, files))
	}
	if installed, ok := commandOutput(ctx, "apk", "info", "-e", pkg); ok && strings.TrimSpace(installed) != "" {
		version, _ := commandOutput(ctx, "apk", "info", "-v", pkg)
		files, _ := commandOutput(ctx, "apk", "info", "-L", pkg)
		version = strings.TrimPrefix(strings.TrimSpace(version), pkg+"-")
		result = append(result, packageInstallation(fingerprint, "apk", version, files))
	}
	return result
}

func packageInstallation(fingerprint *Fingerprint, method, version, files string) Installation {
	path := findExecutablePath(files, fingerprint.ExecName)
	if path == "" {
		path = filepath.Join("/usr/bin", fingerprint.ExecName)
	}
	return Installation{
		Name: fingerprint.DisplayName, Version: strings.TrimSpace(version), Path: path,
		InstallMethod: method, Owner: "root",
	}
}

func scanHomebrew(ctx context.Context, fingerprint *Fingerprint) []Installation {
	if len(fingerprint.HomebrewCasks) == 0 {
		return nil
	}

	var result []Installation
	for _, brew := range []string{"/home/linuxbrew/.linuxbrew/bin/brew", "/opt/homebrew/bin/brew", "/usr/local/bin/brew"} {
		if ctx.Err() != nil {
			return result
		}
		if info, err := os.Stat(brew); err != nil || !info.Mode().IsRegular() {
			continue
		}
		owner := fileOwner(brew)
		prefix := filepath.Dir(filepath.Dir(brew))
		result = append(result, scanHomebrewCaskroom(fingerprint, prefix, owner)...)
		for _, cask := range fingerprint.HomebrewCasks {
			out, ok := commandOutputPath(ctx, brew, "list", "--cask", "--versions", cask)
			if !ok || strings.TrimSpace(out) == "" {
				continue
			}
			fields := strings.Fields(out)
			version := ""
			if len(fields) > 1 {
				version = fields[len(fields)-1]
			}
			files, _ := commandOutputPath(ctx, brew, "list", "--cask", cask)
			path := findExecutablePath(files, fingerprint.ExecName)
			if path == "" {
				path = filepath.Join(filepath.Dir(brew), fingerprint.ExecName)
			}
			result = append(result, Installation{
				Name: fingerprint.DisplayName, Version: version, Path: path,
				InstallMethod: "homebrew", Owner: owner,
			})
		}
	}
	return result
}

// scanHomebrewCaskroom reads the version straight from the Caskroom layout, so
// a cask still resolves when the brew command itself is unavailable.
func scanHomebrewCaskroom(fingerprint *Fingerprint, prefix, owner string) []Installation {
	launcher := filepath.Join(prefix, "bin", fingerprint.ExecName)
	target, err := filepath.EvalSymlinks(launcher)
	if err != nil {
		return nil
	}
	for _, cask := range fingerprint.HomebrewCasks {
		version := versionUnderDir(filepath.Join(prefix, "Caskroom", cask), target)
		if version != "" {
			return []Installation{{
				Name: fingerprint.DisplayName, Version: version, Path: launcher,
				InstallMethod: "homebrew", Owner: owner,
			}}
		}
	}
	return nil
}

func commandOutput(ctx context.Context, name string, args ...string) (string, bool) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", false
	}
	return commandOutputPath(ctx, path, args...)
}

func commandOutputPath(ctx context.Context, path string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	return string(out), err == nil
}
