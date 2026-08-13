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
	"runtime"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/internal/hermes"
)

// maxPackageJSONSize limits reads from glob-matched manifests.
const maxPackageJSONSize = 1024 * 1024

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

func stampAgentId(installations []Installation, mark int, agentId string) {
	for i := mark; i < len(installations); i++ {
		installations[i].AgentId = agentId
	}
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
		AgentId: hermes.AgentID, Name: hermes.DisplayName, Version: project.Version,
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
