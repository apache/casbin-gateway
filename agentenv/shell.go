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

package agentenv

import (
	"os"
	"strings"
)

// parser reads the assignments one startup file makes, keeping the variables in
// keys. A later line wins, the way the shell running the file would resolve it.
type parser func(line string) (string, string, bool)

func readSource(path string, kind string, keys map[string]bool, parse parser) *source {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	vars := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, text, ok := parse(line)
		if !ok || !keys[key] || text == "" {
			continue
		}
		vars[key] = text
	}
	if len(vars) == 0 {
		return nil
	}
	return &source{kind: kind, path: path, vars: vars}
}

// parsePosix reads the shells' own spellings of an assignment: "export KEY=x",
// a bare "KEY=x", fish's "set -gx KEY x" and csh's "setenv KEY x".
func parsePosix(line string) (string, string, bool) {
	switch {
	case strings.HasPrefix(line, "set "):
		fields := strings.Fields(line)
		// Only an exported set reaches the agent, and the flags are written in
		// any order: -gx, -xg, -x.
		if len(fields) < 4 || !strings.HasPrefix(fields[1], "-") || !strings.Contains(fields[1], "x") {
			return "", "", false
		}
		return fields[2], unquote(strings.Join(fields[3:], " ")), true
	case strings.HasPrefix(line, "setenv "):
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return "", "", false
		}
		return fields[1], unquote(strings.Join(fields[2:], " ")), true
	}

	line = strings.TrimPrefix(line, "export ")
	key, text, found := strings.Cut(line, "=")
	if !found || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	return key, unquote(text), true
}

// parsePowerShell reads the one spelling a profile uses to point a client
// somewhere: $env:KEY = "x".
func parsePowerShell(line string) (string, string, bool) {
	if !strings.HasPrefix(strings.ToLower(line), "$env:") {
		return "", "", false
	}

	key, text, found := strings.Cut(line[len("$env:"):], "=")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	return key, unquote(strings.TrimSpace(text)), true
}

// unquote drops the quotes around a value, and the trailing comment after an
// unquoted one.
func unquote(text string) string {
	text = strings.TrimSpace(text)
	for _, quote := range []string{`"`, `'`} {
		if len(text) >= 2 && strings.HasPrefix(text, quote) && strings.HasSuffix(text, quote) {
			return text[1 : len(text)-1]
		}
	}
	if index := strings.Index(text, " #"); index >= 0 {
		text = strings.TrimSpace(text[:index])
	}
	return text
}
