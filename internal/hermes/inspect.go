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

// Package hermes validates Hermes Agent installations.
package hermes

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	AgentID     = "hermes-agent"
	ExecName    = "hermes"
	ProjectDir  = "hermes-agent"
	ProjectName = "hermes-agent"

	maxProjectFileSize = 1024 * 1024
	maxParentDepth     = 6
)

type Project struct {
	Root    string
	Version string
}

// InspectLauncher validates a launcher and locates its Hermes checkout.
func InspectLauncher(launcher string, knownRoots ...string) (Project, error) {
	info, err := os.Stat(launcher)
	if err != nil || !info.Mode().IsRegular() {
		return Project{}, fmt.Errorf("%s is not a regular Hermes launcher", launcher)
	}

	seen := map[string]bool{}
	for _, candidate := range knownRoots {
		if candidate == "" {
			continue
		}
		root := filepath.Clean(candidate)
		seen[root] = true
		if version, ok := projectVersion(root); ok {
			return Project{Root: root, Version: version}, nil
		}
	}

	candidates := []string{filepath.Dir(launcher)}
	if resolved, resolveErr := filepath.EvalSymlinks(launcher); resolveErr == nil {
		candidates = append(candidates, filepath.Dir(resolved))
	}

	for _, candidate := range candidates {
		for depth := 0; candidate != "" && depth <= maxParentDepth; depth++ {
			root := filepath.Clean(candidate)
			if seen[root] {
				break
			}
			seen[root] = true
			if version, ok := projectVersion(root); ok {
				return Project{Root: root, Version: version}, nil
			}
			parent := filepath.Dir(root)
			if parent == root {
				break
			}
			candidate = parent
		}
	}
	return Project{}, fmt.Errorf("%s is not a validated Hermes Agent launcher", launcher)
}

func projectVersion(root string) (string, bool) {
	path := filepath.Join(root, "pyproject.toml")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxProjectFileSize {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	inProject := false
	name, version := "", ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inProject = line == "[project]"
			continue
		}
		if !inProject {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = tomlString(value)
		case "version":
			version = tomlString(value)
		}
	}
	return version, name == ProjectName && version != ""
}

func tomlString(value string) string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '"' && value[0] != '\'' {
		return ""
	}
	quote := value[0]
	for index := 1; index < len(value); index++ {
		if quote == '"' && value[index] == '\\' {
			index++
			continue
		}
		if value[index] != quote {
			continue
		}
		rest := strings.TrimSpace(value[index+1:])
		if rest == "" || strings.HasPrefix(rest, "#") {
			return value[1:index]
		}
		return ""
	}
	return ""
}

func VersionAtLeast(actual, minimum string) bool {
	parse := func(value string) [3]int {
		var result [3]int
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		for index, part := range strings.SplitN(value, ".", len(result)) {
			digits := strings.TrimLeftFunc(part, func(char rune) bool {
				return char >= '0' && char <= '9'
			})
			digits = part[:len(part)-len(digits)]
			result[index], _ = strconv.Atoi(digits)
		}
		return result
	}
	got, want := parse(actual), parse(minimum)
	for index := range got {
		if got[index] != want[index] {
			return got[index] > want[index]
		}
	}
	return true
}
