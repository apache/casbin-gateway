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

package agentprovider

import (
	"strings"
)

// envFile is a .env file kept as the lines it is made of, so the keys Gateway
// does not own stay where they were, comments and all.
type envFile struct {
	lines []string
}

func readEnvFile(path string) (*envFile, error) {
	data, _, _, err := readFile(path)
	if err != nil {
		return nil, err
	}

	file := &envFile{}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if strings.TrimSpace(text) != "" {
		file.lines = strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	return file, nil
}

// keyOf is the variable one line assigns, empty for a comment or a blank line.
func keyOf(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}

	name, _, found := strings.Cut(trimmed, "=")
	if !found {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(name), "export "))
}

func (file *envFile) get(key string) (string, bool) {
	for _, line := range file.lines {
		if keyOf(line) != key {
			continue
		}
		_, value, _ := strings.Cut(line, "=")
		return unquoteEnv(strings.TrimSpace(value)), true
	}
	return "", false
}

// set writes a variable, in place when the file already assigns it so the file
// keeps its order.
func (file *envFile) set(key string, value string) {
	line := key + "=" + quoteEnv(value)
	for i, existing := range file.lines {
		if keyOf(existing) == key {
			file.lines[i] = line
			return
		}
	}
	file.lines = append(file.lines, line)
}

func (file *envFile) remove(key string) {
	kept := make([]string, 0, len(file.lines))
	for _, line := range file.lines {
		if keyOf(line) != key {
			kept = append(kept, line)
		}
	}
	file.lines = kept
}

func (file *envFile) bytes() []byte {
	if len(file.lines) == 0 {
		return nil
	}
	return []byte(strings.Join(file.lines, "\n") + "\n")
}

// quoteEnv quotes a value the shell rules would otherwise cut short. A key or a
// URL needs none of it, which is what these files usually hold.
func quoteEnv(value string) string {
	if value == "" {
		return ""
	}
	if !strings.ContainsAny(value, " \t\"'#$`\\") {
		return value
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func unquoteEnv(value string) string {
	if len(value) < 2 {
		return value
	}

	switch {
	case strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		inner := value[1 : len(value)-1]
		return strings.NewReplacer(`\"`, `"`, `\\`, `\`).Replace(inner)
	case strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'"):
		return value[1 : len(value)-1]
	}
	return value
}
