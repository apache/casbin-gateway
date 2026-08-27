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
	"path/filepath"
	"runtime"
	"strings"
)

// Launch is how one installation is started.
type Launch struct {
	// Executable is the file to run, empty when none was resolved.
	Executable string
	// Args are passed to the executable, literally.
	Args []string
	// Desktop marks a windowed app, which needs no console of its own.
	Desktop bool
}

// LaunchOf resolves what to run for one installation.
func LaunchOf(installation Installation) Launch {
	launch := Launch{}
	execName := ""
	for i := range fingerprints {
		if fingerprints[i].ID == installation.AgentId {
			execName = fingerprints[i].ExecName
			launch.Args = fingerprints[i].LaunchArgs
			launch.Desktop = fingerprints[i].Desktop
			break
		}
	}
	launch.Executable = executableOf(installation.Path, execName)
	return launch
}

// executableOf resolves the launcher of an installation. A package manager
// records the package directory rather than a program, so the shim it installed
// beside the tree is what runs the agent.
func executableOf(path, execName string) string {
	if isFile(path) {
		return path
	}
	if execName == "" {
		return ""
	}

	if root := nodeModulesRoot(path); root != "" {
		for _, candidate := range npmShims(root, execName) {
			if isFile(candidate) {
				return candidate
			}
		}
		for _, dir := range managerBinDirs(root) {
			for _, candidate := range shimNames(dir, execName) {
				if isFile(candidate) {
					return candidate
				}
			}
		}
	}
	// Last, the program the shim would have run: an upgraded or half-removed
	// package manager leaves the tree without its shim.
	return packagedExecutable(path, execName)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// nodeModulesRoot is the directory holding the node_modules tree path sits in.
func nodeModulesRoot(path string) string {
	for dir := filepath.Clean(path); ; {
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		if strings.EqualFold(filepath.Base(dir), "node_modules") {
			return parent
		}
		dir = parent
	}
}

// npmShims are the launcher layouts npm writes: beside the tree on Windows,
// under the prefix's bin/ elsewhere.
func npmShims(root, execName string) []string {
	local := filepath.Join(root, "node_modules", ".bin", execName)
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(root, execName+".cmd"),
			filepath.Join(root, execName+".exe"),
			local + ".cmd",
		}
	}

	prefix := root
	// A Unix global root is <prefix>/lib/node_modules.
	if filepath.Base(root) == "lib" {
		prefix = filepath.Dir(root)
	}
	return []string{filepath.Join(prefix, "bin", execName), local}
}

// managerBinDirs are the shim directories of the package managers that keep one
// for every global package instead of writing beside the tree: pnpm's home and
// Volta's bin, both recognisable from the package path itself.
func managerBinDirs(root string) []string {
	var dirs []string
	for dir := filepath.Clean(root); ; {
		parent := filepath.Dir(dir)
		if parent == dir {
			return dirs
		}
		switch strings.ToLower(filepath.Base(dir)) {
		case "pnpm":
			dirs = append(dirs, dir)
		case "volta":
			dirs = append(dirs, filepath.Join(dir, "bin"))
		}
		dir = parent
	}
}

// shimNames are the launcher file names one directory may hold for an agent.
func shimNames(dir, execName string) []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join(dir, execName+".cmd"),
			filepath.Join(dir, execName+".exe"),
			filepath.Join(dir, execName+".bat"),
		}
	}
	return []string{filepath.Join(dir, execName)}
}

// packagedExecutable finds the program inside a package tree: npm packages of an
// agent written in a compiled language carry one build per platform, either
// beside the entry script or under a vendor directory.
func packagedExecutable(path, execName string) string {
	var patterns []string
	for _, dir := range []string{path, filepath.Join(path, "bin"),
		filepath.Join(path, "vendor", "*"), filepath.Join(path, "vendor", "*", "*")} {
		patterns = append(patterns, filepath.Join(dir, execName+"*"))
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, candidate := range matches {
			if isRunnable(candidate, execName) {
				return candidate
			}
		}
	}
	return ""
}

// isRunnable reports whether a file inside a package tree is the agent itself
// rather than one of the scripts or data files sitting beside it.
func isRunnable(path, execName string) bool {
	name := filepath.Base(path)
	extension := filepath.Ext(name)
	stem := strings.ToLower(strings.TrimSuffix(name, extension))
	if stem != strings.ToLower(execName) && !strings.HasPrefix(stem, strings.ToLower(execName)+"-") {
		return false
	}

	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	extension = strings.ToLower(extension)
	if runtime.GOOS == "windows" {
		return extension == ".exe" || extension == ".cmd" || extension == ".bat"
	}
	switch extension {
	case ".js", ".mjs", ".cjs", ".ts", ".json", ".md":
		return false
	}
	return info.Mode()&0o111 != 0
}
