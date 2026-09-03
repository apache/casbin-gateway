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

package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/internal/hermes"
)

// InstallMethodConfig marks an agent found by the state directory it left in a
// home rather than by a program. Nothing can be launched from it, but its
// configuration is on disk and is what most of Gateway acts on.
const InstallMethodConfig = "config"

// maxPackageJSONSize limits reads from glob-matched manifests.
const maxPackageJSONSize = 1024 * 1024

// maxNpmrcSize limits reads from npmrc files.
const maxNpmrcSize = 64 * 1024

// npmrcEnvRef matches the ${VAR} references npm expands in npmrc values.
var npmrcEnvRef = regexp.MustCompile(`\$\{([^{}]+)\}`)

// scanNpmPatterns validates npm packages found by known glob patterns.
func scanNpmPatterns(ctx context.Context, fingerprint *Fingerprint, patterns []string, owner string, ownerFor func(string) string) []Installation {
	if fingerprint.NpmPackage == "" {
		return nil
	}

	var result []Installation
	for _, pattern := range patterns {
		if ctx.Err() != nil {
			return result
		}
		matches, _ := filepath.Glob(pattern)
		for _, packageJSON := range matches {
			if ctx.Err() != nil {
				return result
			}
			info, err := os.Stat(packageJSON)
			if err != nil || !info.Mode().IsRegular() || info.Size() > maxPackageJSONSize {
				continue
			}
			data, err := os.ReadFile(packageJSON)
			if err != nil {
				continue
			}
			var pkg struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			if json.Unmarshal(data, &pkg) != nil || pkg.Name != fingerprint.NpmPackage || pkg.Version == "" {
				continue
			}
			packageOwner := owner
			if packageOwner == "" && ownerFor != nil {
				packageOwner = ownerFor(packageJSON)
			}
			result = append(result, Installation{
				Name: fingerprint.DisplayName, Version: pkg.Version, Path: filepath.Dir(packageJSON),
				InstallMethod: "npm", Owner: packageOwner,
			})
		}
	}
	return result
}

func (f *Fingerprint) npmPackagePath() string {
	return filepath.FromSlash(f.NpmPackage)
}

// isCurrentHome reports whether path is the home of the user this process runs
// as, the only profile the environment describes.
func isCurrentHome(path string) bool {
	current, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	current, path = filepath.Clean(current), filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(current, path)
	}
	return current == path
}

// npmrcPrefix returns the "prefix" setting of an npmrc file, later keys winning
// as in npm's own ini parsing.
func npmrcPrefix(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxNpmrcSize {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	prefix := ""
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "prefix" {
			continue
		}
		prefix = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return npmrcEnvRef.ReplaceAllStringFunc(prefix, func(ref string) string {
		return os.Getenv(ref[2 : len(ref)-1])
	})
}

// npmPrefixPatterns cover a global root relocated with "npm config set prefix",
// which falls outside every fixed layout. Windows keeps the packages directly
// under the prefix, Unix under lib/.
func npmPrefixPatterns(fingerprint *Fingerprint, home string) []string {
	var prefixes []string
	if isCurrentHome(home) {
		// npm accepts either case for its environment config.
		prefixes = append(prefixes, os.Getenv("npm_config_prefix"), os.Getenv("NPM_CONFIG_PREFIX"))
	}
	prefixes = append(prefixes, npmrcPrefix(filepath.Join(home, ".npmrc")))

	pkg := fingerprint.npmPackagePath()
	seen := map[string]bool{}
	var patterns []string
	for _, prefix := range prefixes {
		prefix = filepath.Clean(strings.TrimSpace(prefix))
		if !filepath.IsAbs(prefix) {
			continue
		}
		key := prefix
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		patterns = append(patterns,
			filepath.Join(prefix, "node_modules", pkg, "package.json"),
			filepath.Join(prefix, "lib", "node_modules", pkg, "package.json"),
		)
	}
	return patterns
}

func stampAgentId(installations []Installation, mark int, agentId string) {
	for i := mark; i < len(installations); i++ {
		installations[i].AgentId = agentId
	}
}

// scanStateDirs reports an agent that left its state directory in a home while
// none of the install layouts matched it anywhere: installed by a package
// manager Gateway does not scan, on a PATH it cannot see, or run through a
// wrapper. Providers, skills and MCP servers are read and written through the
// home directory rather than the program, so leaving these out reports "not
// installed" for an agent someone uses every day.
//
// `found` is what this fingerprint matched already, and one hit anywhere is
// enough to stop: a state directory is also left in the home of every service
// account an agent sandboxes itself into, and those are not installations
// anyone wants listed next to their own.
func scanStateDirs(fingerprint *Fingerprint, homes []homeDir, found []Installation) []Installation {
	if fingerprint.StateDir == "" || len(found) > 0 {
		return nil
	}

	installations := []Installation{}
	for _, home := range homes {
		path := filepath.Join(home.path, "."+fingerprint.StateDir)
		if !hasEntries(path) {
			continue
		}
		installations = append(installations, Installation{
			AgentId:       fingerprint.ID,
			Name:          fingerprint.DisplayName,
			Path:          path,
			InstallMethod: InstallMethodConfig,
			Owner:         home.owner,
		})
	}
	return installations
}

// hasEntries reports a directory that holds something. An empty one is what an
// uninstall leaves behind, and is not evidence that the agent was ever set up.
func hasEntries(path string) bool {
	dir, err := os.Open(path)
	if err != nil {
		return false
	}
	defer dir.Close()

	names, err := dir.Readdirnames(1)
	return err == nil && len(names) > 0
}

// dedupeInstallations returns unique installations ordered by owner and path.
func dedupeInstallations(installations []Installation) []Installation {
	seen := map[string]bool{}
	result := make([]Installation, 0, len(installations))
	for _, installation := range installations {
		key := canonicalPath(installation.Path)
		if key == "" {
			continue
		}
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, installation)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Owner != result[j].Owner {
			return result[i].Owner < result[j].Owner
		}
		return result[i].Path < result[j].Path
	})
	return result
}

// dropUnlaunchableTrees removes an installation nothing can start when the same
// agent is installed elsewhere with a launcher. A package manager that upgrades
// or half-removes itself leaves the package directory behind without the shim
// that ran it, and that leftover is not an installation the user can use.
func dropUnlaunchableTrees(installations []Installation) []Installation {
	launchable := map[string]bool{}
	resolved := make([]bool, len(installations))
	for i, installation := range installations {
		resolved[i] = LaunchOf(installation).Executable != ""
		if resolved[i] {
			launchable[installation.AgentId] = true
		}
	}

	result := make([]Installation, 0, len(installations))
	for i, installation := range installations {
		if !resolved[i] && launchable[installation.AgentId] {
			continue
		}
		result = append(result, installation)
	}
	return result
}

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

// versionUnderDir extracts a version directory below root.
func versionUnderDir(root, target string) string {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	return strings.Split(relative, string(filepath.Separator))[0]
}

func findExecutablePath(files, execName string) string {
	for _, line := range strings.Split(files, "\n") {
		line = strings.TrimSpace(line)
		if filepath.Base(line) == execName {
			return line
		}
	}
	return ""
}

func hermesInstallation(launcher, owner string, officialRoots ...string) []Installation {
	project, err := hermes.InspectLauncher(launcher, officialRoots...)
	if err != nil {
		return nil
	}
	method := "source"
	for _, root := range officialRoots {
		if sameHermesPath(root, project.Root) {
			method = "native"
			break
		}
	}
	return []Installation{{
		AgentId: hermes.AgentID, Name: DisplayNameOf(hermes.AgentID), Version: project.Version,
		Path: launcher, InstallMethod: method, Owner: owner,
	}}
}

func scanHermesOnPath() []Installation {
	launcher, err := exec.LookPath(hermes.ExecName)
	if err != nil && runtime.GOOS == "windows" {
		launcher, err = exec.LookPath(hermes.ExecName + ".exe")
	}
	if err != nil {
		return nil
	}
	owner := ""
	if account, userErr := user.Current(); userErr == nil {
		owner = account.Username
	}
	return hermesInstallation(launcher, owner)
}

func sameHermesPath(left, right string) bool {
	leftResolved, leftErr := filepath.EvalSymlinks(left)
	rightResolved, rightErr := filepath.EvalSymlinks(right)
	if leftErr == nil {
		left = leftResolved
	}
	if rightErr == nil {
		right = rightResolved
	}
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
