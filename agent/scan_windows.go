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

//go:build windows

package agent

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/internal/hermes"
	"golang.org/x/sys/windows/registry"
)

// homeDir is one user's profile directory and the account that owns it.
type homeDir struct {
	owner string
	path  string
}

// scan finds installations of every known agent in known Windows layouts. It
// neither executes discovered binaries nor walks arbitrary filesystem roots.
func scan(ctx context.Context) []Installation {
	homes := windowsHomes(ctx)
	paths := newPathIndex()
	nodeRoots := windowsNodeRoots()
	nodeOwner := currentOwner()

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
			installations = append(installations, scanWindowsNative(fingerprint, home)...)
			installations = append(installations, scanBundledDirs(fingerprint, home)...)
			installations = append(installations, scanWindowsWinget(ctx, fingerprint, home)...)
			installations = append(installations, scanWindowsNpm(ctx, fingerprint, home)...)
			installations = append(installations, scanWindowsUserPrograms(ctx, fingerprint, home)...)
			installations = append(installations, scanWindowsInstallDirs(ctx, fingerprint, home.path, fingerprint.HomeDirs, home.owner, "native")...)
		}
		installations = append(installations, scanWindowsDesktop(ctx, fingerprint, homes)...)
		installations = append(installations, scanMachineWinget(ctx, fingerprint)...)
		installations = append(installations, scanMachinePrograms(ctx, fingerprint)...)
		installations = append(installations, scanMachineNpm(ctx, fingerprint, nodeRoots, nodeOwner)...)
		installations = append(installations, scanPathDirs(fingerprint, paths, installations[mark:])...)
		stampAgentId(installations, mark, fingerprint.ID)
		installations = append(installations, scanStateDirs(fingerprint, homes, installations[mark:])...)
		fillMissingVersions(installations, mark, fingerprint)
	}
	for _, home := range homes {
		if ctx.Err() != nil {
			return installations
		}
		installations = append(installations, scanHermesWindows(home)...)
	}
	installations = append(installations, scanHermesOnPath()...)

	// Last, so that an agent found both on disk and by its port keeps the
	// richer install-layout row when the two resolve to the same executable.
	installations = append(installations, scanLocalServers(ctx)...)
	// After every layout, so a chosen program yields to the same one found where
	// its installer puts it.
	installations = append(installations, ManualInstallations()...)

	installations = expandSharedCodexWindowsInstallations(dedupeInstallations(installations), homes)
	// Whatever layout an installation came from, its launcher may carry a
	// version resource, so fall back to that rather than reporting no version -
	// except where the fingerprint says that resource is the packager's.
	for i := range installations {
		if ctx.Err() != nil {
			return installations
		}
		if installations[i].Version == "" && !ignoresExecutableVersion(installations[i].AgentId) {
			installations[i].Version = executableVersion(installations[i].Path)
		}
	}
	fillAccounts(installations, homes)
	return installations
}

func ignoresExecutableVersion(agentId string) bool {
	for i := range fingerprints {
		if fingerprints[i].ID == agentId {
			return fingerprints[i].IgnoreExecutableVersion
		}
	}
	return false
}

func windowsHomes(ctx context.Context) []homeDir {
	seen := map[string]bool{}
	var homes []homeDir
	add := func(owner, path string) {
		path = filepath.Clean(path)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return
		}
		key := strings.ToLower(path)
		if seen[key] {
			return
		}
		seen[key] = true
		if owner == "" {
			owner = filepath.Base(path)
		}
		homes = append(homes, homeDir{owner: owner, path: path})
	}

	const profileList = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProfileList`
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, profileList, registry.ENUMERATE_SUB_KEYS)
	if err == nil {
		defer key.Close()
		if sids, err := key.ReadSubKeyNames(-1); err == nil {
			for _, sid := range sids {
				if ctx.Err() != nil {
					return homes
				}
				profile, err := registry.OpenKey(key, sid, registry.QUERY_VALUE)
				if err != nil {
					continue
				}
				path, _, readErr := profile.GetStringValue("ProfileImagePath")
				profile.Close()
				if readErr != nil {
					continue
				}
				if expanded, err := registry.ExpandString(path); err == nil {
					path = expanded
				}
				owner := ""
				if account, err := user.LookupId(sid); err == nil {
					owner = account.Username
				}
				add(owner, path)
			}
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		owner := ""
		if account, err := user.Current(); err == nil {
			owner = account.Username
		}
		add(owner, home)
	}
	return homes
}

// appDataDir respects relocated AppData paths for the current user.
func appDataDir(home homeDir, kind, variable string) string {
	if isCurrentHome(home.path) {
		if configured := os.Getenv(variable); configured != "" {
			return configured
		}
	}
	return filepath.Join(home.path, "AppData", kind)
}

func localAppData(home homeDir) string   { return appDataDir(home, "Local", "LOCALAPPDATA") }
func roamingAppData(home homeDir) string { return appDataDir(home, "Roaming", "APPDATA") }

// appDataRoot is where an app keeps the data of one account, which is where a
// copy of an agent it ships goes.
func appDataRoot(home homeDir) string { return roamingAppData(home) }

func scanHermesWindows(home homeDir) []Installation {
	root := filepath.Join(hermes.Home(home.path), hermes.ProjectDir)
	launcher := filepath.Join(root, "venv", "Scripts", hermes.ExecName+".exe")
	return hermesInstallation(launcher, home.owner, root)
}

// scanWindowsNative reports the per-user native installer layout: a launcher at
// %USERPROFILE%\.local\bin\<exec>.exe backed by a versioned payload directory.
func scanWindowsNative(fingerprint *Fingerprint, home homeDir) []Installation {
	if fingerprint.ExecName == "" || fingerprint.StateDir == "" {
		return nil
	}

	launcher := filepath.Join(home.path, ".local", "bin", fingerprint.ExecName+".exe")
	launcherInfo, err := os.Stat(launcher)
	if err != nil || !launcherInfo.Mode().IsRegular() {
		return nil
	}

	version := ""
	versionsDir := filepath.Join(home.path, ".local", "share", fingerprint.StateDir, "versions")
	if target, err := filepath.EvalSymlinks(launcher); err == nil {
		version = versionUnderDir(versionsDir, target)
	}
	// Windows installs may hard-link rather than symlink the launcher, in which
	// case the version is only recoverable by identity against the payloads.
	if version == "" {
		if entries, err := os.ReadDir(versionsDir); err == nil {
			for _, entry := range entries {
				candidate, err := entry.Info()
				if err == nil && candidate.Mode().IsRegular() && os.SameFile(launcherInfo, candidate) {
					version = entry.Name()
					break
				}
			}
		}
	}
	return []Installation{{
		Name: fingerprint.DisplayName, Version: version, Path: launcher,
		InstallMethod: "native", Owner: home.owner,
	}}
}

func scanWindowsWinget(ctx context.Context, fingerprint *Fingerprint, home homeDir) []Installation {
	root := filepath.Join(localAppData(home), "Microsoft", "WinGet", "Packages")
	return scanWingetPackages(ctx, fingerprint, root, home.owner)
}

func scanMachineWinget(ctx context.Context, fingerprint *Fingerprint) []Installation {
	var result []Installation
	seen := map[string]bool{}
	for _, variable := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if ctx.Err() != nil {
			return result
		}
		base := os.Getenv(variable)
		if base == "" {
			continue
		}
		root := filepath.Join(base, "WinGet", "Packages")
		key := strings.ToLower(filepath.Clean(root))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, scanWingetPackages(ctx, fingerprint, root, "SYSTEM")...)
	}
	return result
}

// scanWindowsUserPrograms checks known per-user installer directories.
func scanWindowsUserPrograms(ctx context.Context, fingerprint *Fingerprint, home homeDir) []Installation {
	local := localAppData(home)
	result := scanWindowsInstallDirs(ctx, fingerprint, filepath.Join(local, "Programs"), fingerprint.WindowsProgramDirs, home.owner, "installer")
	return append(result, scanWindowsInstallDirs(ctx, fingerprint, local, fingerprint.WindowsUserDirs, home.owner, "native")...)
}

func scanMachinePrograms(ctx context.Context, fingerprint *Fingerprint) []Installation {
	var result []Installation
	seen := map[string]bool{}
	for _, variable := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		if ctx.Err() != nil {
			return result
		}
		root := os.Getenv(variable)
		if root == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(root))
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, scanWindowsInstallDirs(ctx, fingerprint, root, fingerprint.WindowsProgramDirs, "SYSTEM", "installer")...)
	}
	return result
}

// scanWindowsInstallDirs reports an installer layout: one directory per agent
// holding the whole application, with the launcher at its root.
func scanWindowsInstallDirs(ctx context.Context, fingerprint *Fingerprint, root string, dirs []string, owner, method string) []Installation {
	if fingerprint.ExecName == "" || root == "" {
		return nil
	}

	var result []Installation
	for _, dir := range dirs {
		if ctx.Err() != nil {
			return result
		}
		installDir := filepath.Join(root, filepath.FromSlash(dir))
		executable := filepath.Join(installDir, fingerprint.ExecName+".exe")
		if info, err := os.Stat(executable); err != nil || !info.Mode().IsRegular() {
			continue
		}
		result = append(result, Installation{
			Name: fingerprint.DisplayName, Path: executable,
			InstallMethod: method, Owner: owner,
		})
	}
	return result
}

func scanWingetPackages(ctx context.Context, fingerprint *Fingerprint, root, owner string) []Installation {
	if fingerprint.WingetPackage == "" || fingerprint.ExecName == "" {
		return nil
	}

	// winget appends an install-source hash to the package identifier.
	packages, _ := filepath.Glob(filepath.Join(root, fingerprint.WingetPackage+"_*"))
	var result []Installation
	for _, packageDir := range packages {
		if ctx.Err() != nil {
			return result
		}
		executable := filepath.Join(packageDir, fingerprint.ExecName+".exe")
		if info, err := os.Stat(executable); err == nil && info.Mode().IsRegular() {
			result = append(result, Installation{
				Name: fingerprint.DisplayName, Path: executable,
				InstallMethod: "winget", Owner: owner,
			})
		}
	}
	return result
}

func scanWindowsNpm(ctx context.Context, fingerprint *Fingerprint, home homeDir) []Installation {
	roaming := roamingAppData(home)
	local := localAppData(home)
	pkg := fingerprint.npmPackagePath()
	patterns := []string{
		filepath.Join(roaming, "npm", "node_modules", pkg, "package.json"),
		filepath.Join(roaming, "nvm", "*", "node_modules", pkg, "package.json"),
		filepath.Join(roaming, "fnm", "node-versions", "*", "installation", "node_modules", pkg, "package.json"),
		// A Volta package image mirrors the npm prefix layout, which on Windows
		// has no "lib" level; match both so a relocated image still resolves.
		filepath.Join(local, "Volta", "tools", "image", "packages", pkg, "node_modules", pkg, "package.json"),
		filepath.Join(local, "Volta", "tools", "image", "packages", pkg, "lib", "node_modules", pkg, "package.json"),
		// pnpm groups its global packages under a store layout version.
		filepath.Join(local, "pnpm", "global", "*", "node_modules", pkg, "package.json"),
		// An agent whose documented command is "npx <package>" is never
		// installed globally; the copy npx keeps in npm's cache is the install.
		filepath.Join(local, "npm-cache", "_npx", "*", "node_modules", pkg, "package.json"),
	}
	for _, dir := range fingerprint.ExtraWindowsNpmDirs {
		patterns = append(patterns, filepath.Join(local, filepath.FromSlash(dir), "node_modules", pkg, "package.json"))
	}
	patterns = append(patterns, npmPrefixPatterns(fingerprint, home.path)...)
	return scanNpmPatterns(ctx, fingerprint, patterns, home.owner, nil)
}
