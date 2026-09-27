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
	"fmt"
	"strconv"
	"strings"
)

// config.toml is hand-written, with comments and formatting in it, so it is
// edited as text: decoding and re-encoding it would hand it back stripped of
// both. Reading, where nothing is written back, decodes instead.

// tomlHeader parses a table header line such as "[model_providers.gateway]".
func tomlHeader(line string) ([]string, bool) {
	text := strings.TrimSpace(line)
	if !strings.HasPrefix(text, "[") || strings.HasPrefix(text, "[[") {
		return nil, false
	}

	// A quoted part may hold a "." or a "]" of its own - a model id such as
	// "gpt-5.5" does - so the header is scanned rather than split.
	parts := []string{}
	part := strings.Builder{}
	var quote byte
	for index := 1; index < len(text); index++ {
		character := text[index]
		switch {
		case quote != 0:
			if character == '\\' && quote == '"' && index+1 < len(text) {
				index++
				part.WriteByte(text[index])
			} else if character == quote {
				quote = 0
			} else {
				part.WriteByte(character)
			}
		case character == '"' || character == '\'':
			quote = character
		case character == '.' || character == ']':
			parts = append(parts, strings.TrimSpace(part.String()))
			part.Reset()
			if character == ']' {
				return parts, true
			}
		default:
			part.WriteByte(character)
		}
	}
	return nil, false
}

// tomlCutTable removes one table and its sub-tables, leaving every other line,
// comment and blank line exactly as it was.
func tomlCutTable(text string, path ...string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	dropping := false

	for _, line := range lines {
		if header, ok := tomlHeader(line); ok {
			dropping = tomlHeaderMatches(header, path)
		}
		if !dropping {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func tomlHeaderMatches(header []string, path []string) bool {
	if len(header) < len(path) {
		return false
	}
	for index, part := range path {
		if header[index] != part {
			return false
		}
	}
	return true
}

// tomlSetRootKey writes a key of the root table, which is everything before the
// first table header. An existing assignment is replaced in place.
func tomlSetRootKey(text string, key string, value string) string {
	assignment := fmt.Sprintf("%s = %s", key, strconv.Quote(value))
	lines := strings.Split(text, "\n")
	end := len(lines)

	for index, line := range lines {
		if _, ok := tomlHeader(line); ok {
			end = index
			break
		}
		if tomlAssigns(line, key) {
			lines[index] = assignment
			return strings.Join(lines, "\n")
		}
	}

	// Keep the assignment inside the root table: after the first header it
	// would belong to that table instead.
	inserted := []string{assignment}
	if end < len(lines) {
		inserted = append(inserted, "")
	}
	rest := append(inserted, lines[end:]...)
	return strings.Join(append(lines[:end:end], rest...), "\n")
}

func tomlDeleteRootKey(text string, key string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	dropBlank := false

	for index, line := range lines {
		if _, ok := tomlHeader(line); ok {
			kept = append(kept, lines[index:]...)
			break
		}
		if tomlAssigns(line, key) {
			// The assignment was written with a blank line of its own, so
			// leaving both behind would drift the layout of the rest of the file.
			dropBlank = len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == ""
			continue
		}
		if dropBlank {
			dropBlank = false
			if strings.TrimSpace(line) == "" {
				continue
			}
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// tomlSetTableKey writes a key of one top-level table, appending the table when
// the file has none. An existing assignment is replaced in place.
func tomlSetTableKey(text string, table string, key string, value string) string {
	assignment := fmt.Sprintf("%s = %s", key, strconv.Quote(value))
	lines := strings.Split(text, "\n")
	start, end := tomlTableSpan(lines, table)
	if start < 0 {
		return tomlAppend(text, fmt.Sprintf("[%s]\n%s\n", tomlKey(table), assignment))
	}

	last := start
	for index := start + 1; index < end; index++ {
		if tomlAssigns(lines[index], key) {
			lines[index] = assignment
			return strings.Join(lines, "\n")
		}
		if strings.TrimSpace(lines[index]) != "" {
			last = index
		}
	}
	rest := append([]string{assignment}, lines[last+1:]...)
	return strings.Join(append(lines[:last+1:last+1], rest...), "\n")
}

// tomlDeleteTableKey removes a key of one top-level table, and the table with
// it when nothing else is left in it.
func tomlDeleteTableKey(text string, table string, key string) string {
	lines := strings.Split(text, "\n")
	start, end := tomlTableSpan(lines, table)
	if start < 0 {
		return text
	}

	kept := []string{}
	empty := true
	for index := start + 1; index < end; index++ {
		if tomlAssigns(lines[index], key) {
			continue
		}
		if strings.TrimSpace(lines[index]) != "" {
			empty = false
		}
		kept = append(kept, lines[index])
	}
	if empty {
		return strings.Join(append(lines[:start:start], lines[end:]...), "\n")
	}
	rest := append(kept, lines[end:]...)
	return strings.Join(append(lines[:start+1:start+1], rest...), "\n")
}

// tomlTableSpan is the header line of one top-level table and the line its
// body ends before, -1 when the file has no such table.
func tomlTableSpan(lines []string, table string) (int, int) {
	start := -1
	for index, line := range lines {
		header, ok := tomlHeader(line)
		if !ok && !strings.HasPrefix(strings.TrimSpace(line), "[[") {
			continue
		}
		if start >= 0 {
			return start, index
		}
		if ok && len(header) == 1 && header[0] == table {
			start = index
		}
	}
	return start, len(lines)
}

func tomlAssigns(line string, key string) bool {
	text := strings.TrimSpace(line)
	if !strings.HasPrefix(text, key) {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(text[len(key):]), "=")
}

// tomlCutTablesUnder removes every table under parent whose own key starts with
// prefix, and their sub-tables. It is how a set of entries written under one
// name is taken back out without knowing what was in it.
func tomlCutTablesUnder(text string, parent string, prefix string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	dropping := false

	for _, line := range lines {
		if header, ok := tomlHeader(line); ok {
			dropping = len(header) > 1 && header[0] == parent && strings.HasPrefix(header[1], prefix)
		}
		if !dropping {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// tomlTable renders one table with string values, sorted by the given order so
// the file reads the same after every switch.
func tomlTable(path []string, keys []string, values map[string]string) string {
	quoted := map[string]string{}
	for key, value := range values {
		if value != "" {
			quoted[key] = strconv.Quote(value)
		}
	}
	return tomlRawTable(path, keys, quoted)
}

// tomlRawTable is tomlTable for values that are already TOML literals, which is
// what a table mixing strings with numbers needs.
func tomlRawTable(path []string, keys []string, values map[string]string) string {
	names := make([]string, 0, len(path))
	for _, part := range path {
		names = append(names, tomlKey(part))
	}

	builder := &strings.Builder{}
	fmt.Fprintf(builder, "[%s]\n", strings.Join(names, "."))
	for _, key := range keys {
		if values[key] != "" {
			fmt.Fprintf(builder, "%s = %s\n", key, values[key])
		}
	}
	return builder.String()
}

// tomlKey is one part of a table path, quoted when it holds anything a bare key
// may not: a "." would otherwise nest the table a level deeper.
func tomlKey(part string) string {
	for _, letter := range part {
		if letter >= 'a' && letter <= 'z' || letter >= 'A' && letter <= 'Z' ||
			letter >= '0' && letter <= '9' || letter == '-' || letter == '_' {
			continue
		}
		return strconv.Quote(part)
	}
	return part
}

// tomlAppend puts a rendered block at the end of a document, separated from
// whatever precedes it by one blank line.
func tomlAppend(text string, block string) string {
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	return text + block
}

// tomlTidy collapses the blank lines a removed assignment leaves behind.
func tomlTidy(text string) string {
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimLeft(text, "\n")
}
