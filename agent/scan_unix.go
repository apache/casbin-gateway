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

//go:build linux || darwin

package agent

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/apache/casbin-gateway/internal/hermes"
)

// homeDir is one user's home directory and the account that owns it.
type homeDir struct {
	owner string
	path  string
}

// scanNative reports the per-user native installer layout: a launcher at
// ~/.local/bin/<exec> pointing into ~/.local/share/<state>/versions/<version>.
func scanNative(fingerprint *Fingerprint, home homeDir) []Installation {
	if fingerprint.ExecName == "" || fingerprint.StateDir == "" {
		return nil
	}

	launcher := filepath.Join(home.path, ".local", "bin", fingerprint.ExecName)
	info, err := os.Stat(launcher)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil
	}

	version := ""
	if target, err := filepath.EvalSymlinks(launcher); err == nil {
		version = versionUnderDir(filepath.Join(home.path, ".local", "share", fingerprint.StateDir, "versions"), target)
	}
	return []Installation{{
		Name: fingerprint.DisplayName, Version: version, Path: launcher,
		InstallMethod: "native", Owner: home.owner,
	}}
}

// userNpmPatterns are the per-user Node package manager layouts that can hold a
// globally installed agent package, plus any bundled runtime the agent declares.
func userNpmPatterns(fingerprint *Fingerprint, home string) []string {
	pkg := fingerprint.npmPackagePath()
	patterns := []string{
		filepath.Join(home, ".npm-global", "lib", "node_modules", pkg, "package.json"),
		filepath.Join(home, ".nvm", "versions", "node", "*", "lib", "node_modules", pkg, "package.json"),
		// fnm honours XDG when it is set and falls back to ~/.fnm otherwise.
		filepath.Join(home, ".fnm", "node-versions", "*", "installation", "lib", "node_modules", pkg, "package.json"),
		filepath.Join(home, ".local", "share", "fnm", "node-versions", "*", "installation", "lib", "node_modules", pkg, "package.json"),
		filepath.Join(home, ".volta", "tools", "image", "packages", pkg, "lib", "node_modules", pkg, "package.json"),
		filepath.Join(home, ".asdf", "installs", "nodejs", "*", "lib", "node_modules", pkg, "package.json"),
		// pnpm groups its global packages under a store layout version.
		filepath.Join(home, ".local", "share", "pnpm", "global", "*", "node_modules", pkg, "package.json"),
	}
	for _, dir := range fingerprint.ExtraUnixNpmDirs {
		patterns = append(patterns, filepath.Join(home, filepath.FromSlash(dir), "node_modules", pkg, "package.json"))
	}
	return patterns
}

func fileOwner(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "root"
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "root"
	}
	id := strconv.FormatUint(uint64(stat.Uid), 10)
	account, err := user.LookupId(id)
	if err != nil {
		return id
	}
	return account.Username
}

func scanHermesUnix(home homeDir, launcher string, roots ...string) []Installation {
	info, err := os.Stat(launcher)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil
	}
	if len(roots) == 0 {
		roots = []string{filepath.Join(home.path, ".hermes", hermes.ProjectDir)}
		if currentHome, homeErr := os.UserHomeDir(); homeErr == nil &&
			sameHermesPath(currentHome, home.path) && os.Getenv("HERMES_HOME") != "" {
			roots = []string{filepath.Join(os.Getenv("HERMES_HOME"), hermes.ProjectDir)}
		}
	}
	return hermesInstallation(launcher, home.owner, roots...)
}
