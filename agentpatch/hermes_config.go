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

package agentpatch

// Hermes' config.yaml belongs to the user, not to Gateway: a normal install is
// well over a thousand lines of section banners, trailing notes and blank-line
// separated blocks. Round-tripping that through a YAML marshaller to flip one
// list entry rewrites every line of the file, so enabling the observer inserts
// a single list item into the text instead. Everything outside that one line
// stays byte for byte identical, which is also what lets Unpatch delete just
// that line rather than restore a whole-file snapshot taken at Patch time.
//
// Reading stays with the real YAML parser: hermesPluginMembership is what
// Status trusts, and writeHermesPluginMembership re-parses its own output
// before committing it, so a document shape the line editor gets wrong is
// caught here instead of by Hermes at startup.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// writeHermesPluginMembership adds the observer to plugins.enabled, or removes
// it, editing configPath in place. An existing config keeps its current mode.
func writeHermesPluginMembership(configPath string, enabled bool) error {
	data, err := os.ReadFile(configPath)
	missing := os.IsNotExist(err)
	if err != nil && !missing {
		return err
	}
	if missing && !enabled {
		return nil
	}

	updated, changed, err := setHermesPluginMembership(data, enabled)
	if err != nil {
		return fmt.Errorf("update %s: %w", configPath, err)
	}
	if !changed {
		return nil
	}
	if err := verifyHermesPluginMembership(updated, enabled); err != nil {
		return fmt.Errorf("update %s: %w", configPath, err)
	}

	if err := os.WriteFile(configPath, updated, 0o600); err != nil {
		return err
	}
	return nil
}

// verifyHermesPluginMembership re-reads an edited document with the YAML parser
// and checks the observer ended up where the edit intended. It is the guard
// that keeps a line edit from writing a document Hermes would read differently.
func verifyHermesPluginMembership(data []byte, enabled bool) error {
	listed, blocked, err := hermesPluginMembershipOf(data)
	if err != nil {
		return err
	}
	if listed != enabled || (enabled && blocked) {
		return errors.New("the edited plugins list does not read back as expected")
	}
	return nil
}

// hermesPluginEdit is one list membership the observer needs in config.yaml.
type hermesPluginEdit struct {
	key  string
	want bool
}

// setHermesPluginMembership returns config.yaml with the observer added to or
// removed from plugins.enabled. Enabling also drops it from plugins.disabled,
// which Hermes treats as a deny list that wins over plugins.enabled. Disabling
// leaves plugins.disabled alone: an entry there is the user's, never ours.
func setHermesPluginMembership(data []byte, enabled bool) ([]byte, bool, error) {
	edits := []hermesPluginEdit{{key: "enabled", want: enabled}}
	if enabled {
		edits = append(edits, hermesPluginEdit{key: "disabled", want: false})
	}

	current, changed := data, false
	for _, edit := range edits {
		next, stepChanged, err := setHermesPluginListMember(current, edit.key, edit.want)
		if err != nil {
			return nil, false, err
		}
		if stepChanged {
			current, changed = next, true
		}
	}
	return current, changed, nil
}

// setHermesPluginListMember adds or removes the observer in plugins.<key>. The
// document is re-read for each edit so one edit's line numbers never depend on
// another's.
func setHermesPluginListMember(data []byte, key string, want bool) ([]byte, bool, error) {
	document := newYAMLLines(data)

	plugins := document.findMappingKey("plugins", 0, 0, len(document.lines))
	if plugins < 0 {
		if !want {
			return data, false, nil
		}
		document.appendBlock("plugins:", "  "+key+":", "    - "+hermesPluginName)
		return document.bytes(), true, nil
	}
	if _, value, _ := yamlMappingKey(document.lines[plugins], 0); value != "" {
		return nil, false, fmt.Errorf("plugins must be a block mapping, found %q", value)
	}

	pluginsEnd := document.blockEnd(0, plugins+1)
	childIndent := document.childIndent(plugins, 0, pluginsEnd)
	keyLine := document.findMappingKey(key, childIndent, plugins+1, pluginsEnd)
	if keyLine < 0 {
		if !want {
			return data, false, nil
		}
		indent := strings.Repeat(" ", childIndent)
		document.insert(document.lastContent(plugins+1, pluginsEnd)+1,
			indent+key+":", indent+"  - "+hermesPluginName)
		return document.bytes(), true, nil
	}
	if _, value, _ := yamlMappingKey(document.lines[keyLine], childIndent); value != "" {
		return setHermesPluginFlowMember(document, keyLine, key, want)
	}

	items := document.sequenceItems(keyLine, childIndent)
	if len(items) == 0 && document.lastContent(keyLine+1, document.blockEnd(childIndent, keyLine+1)) > keyLine {
		return nil, false, fmt.Errorf("plugins.%s must be a list", key)
	}
	existing := -1
	for _, item := range items {
		if value, _, _ := yamlSequenceItem(document.lines[item]); value == hermesPluginName {
			existing = item
			break
		}
	}

	if !want {
		if existing < 0 {
			return data, false, nil
		}
		document.remove(existing)
		return document.bytes(), true, nil
	}
	if existing >= 0 {
		return data, false, nil
	}
	itemIndent, insertAt := childIndent+2, keyLine+1
	if len(items) > 0 {
		last := items[len(items)-1]
		_, itemIndent, _ = yamlSequenceItem(document.lines[last])
		// An item may span more than its first line, so append after the last
		// line that still belongs to it rather than after its opening dash.
		insertAt = document.lastContent(last, document.blockEnd(itemIndent, last+1)) + 1
	}
	document.insert(insertAt, strings.Repeat(" ", itemIndent)+"- "+hermesPluginName)
	return document.bytes(), true, nil
}

// setHermesPluginFlowMember edits an inline "key: [a, b]" list without parsing
// or rewriting its existing values. Patch appends its own scalar at the end;
// Unpatch removes only that exact trailing scalar. If an operator moves or
// rewrites the entry, Unpatch reports the conflict instead of risking the rest
// of the flow sequence.
func setHermesPluginFlowMember(document *yamlLines, keyLine int, key string, want bool) ([]byte, bool, error) {
	line := document.lines[keyLine]
	open := strings.IndexByte(line, '[')
	end := strings.LastIndexByte(line, ']')
	if open < 0 || end < open {
		return nil, false, fmt.Errorf("plugins.%s must be a list", key)
	}

	enabled, disabled, err := hermesPluginMembershipOf(document.bytes())
	if err != nil {
		return nil, false, err
	}
	found := enabled
	if key == "disabled" {
		found = disabled
	}
	if found == want {
		return document.bytes(), false, nil
	}

	content := line[open+1 : end]
	core := strings.TrimRight(content, " \t")
	trailing := content[len(core):]
	if want {
		separator := ""
		if strings.TrimSpace(core) != "" {
			separator = ", "
		}
		document.lines[keyLine] = line[:open+1] + core + separator + hermesPluginName + trailing + line[end:]
		return document.bytes(), true, nil
	}

	if strings.TrimSpace(core) == hermesPluginName {
		document.lines[keyLine] = line[:open+1] + trailing + line[end:]
		return document.bytes(), true, nil
	}
	separator := strings.LastIndexByte(core, ',')
	if separator < 0 || strings.TrimSpace(core[separator+1:]) != hermesPluginName {
		return nil, false, fmt.Errorf("plugins.%s Gateway entry is not the final flow-list item", key)
	}
	document.lines[keyLine] = line[:open+1] + core[:separator] + trailing + line[end:]
	return document.bytes(), true, nil
}

// yamlLines is a line-oriented view of a YAML document. It understands just
// enough of the syntax to find a mapping key and the block sequence under it;
// every shape beyond that it declines to touch, leaving the caller to report a
// config it should not be editing blind.
type yamlLines struct {
	lines   []string
	newline string
}

func newYAMLLines(data []byte) *yamlLines {
	newline := "\n"
	if bytes.Contains(data, []byte("\r\n")) {
		newline = "\r\n"
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	return &yamlLines{lines: strings.Split(text, "\n"), newline: newline}
}

func (y *yamlLines) bytes() []byte {
	return []byte(strings.Join(y.lines, y.newline))
}

func (y *yamlLines) insert(at int, lines ...string) {
	y.lines = append(y.lines[:at:at], append(lines, y.lines[at:]...)...)
}

func (y *yamlLines) remove(at int) {
	y.lines = append(y.lines[:at:at], y.lines[at+1:]...)
}

// appendBlock adds lines at the end of the document, keeping the file's final
// newline and separating the new block from whatever content came before it.
func (y *yamlLines) appendBlock(lines ...string) {
	at := len(y.lines)
	if at > 0 && y.lines[at-1] == "" {
		// The empty last element stands for the file's final newline.
		at--
	} else {
		y.lines = append(y.lines, "")
	}
	if at > 0 && strings.TrimSpace(y.lines[at-1]) != "" {
		lines = append([]string{""}, lines...)
	}
	y.insert(at, lines...)
}

// findMappingKey returns the line index of key at indent inside [start, end),
// or -1. The scan stops at the first content line shallower than indent, which
// is where the surrounding block ends.
func (y *yamlLines) findMappingKey(key string, indent, start, end int) int {
	for index := start; index < end && index < len(y.lines); index++ {
		current, ok := yamlContentIndent(y.lines[index])
		if !ok || current > indent {
			continue
		}
		if current < indent {
			return -1
		}
		if name, _, ok := yamlMappingKey(y.lines[index], indent); ok && name == key {
			return index
		}
	}
	return -1
}

// blockEnd returns the first line index at or after start that leaves the block
// owned by a key at indent.
func (y *yamlLines) blockEnd(indent, start int) int {
	for index := start; index < len(y.lines); index++ {
		if current, ok := yamlContentIndent(y.lines[index]); ok && current <= indent {
			return index
		}
	}
	return len(y.lines)
}

// childIndent is the indentation of the first content line inside the block
// under keyLine, or two past the key's own indent when the block is empty.
func (y *yamlLines) childIndent(keyLine, indent, end int) int {
	for index := keyLine + 1; index < end && index < len(y.lines); index++ {
		if current, ok := yamlContentIndent(y.lines[index]); ok {
			return current
		}
	}
	return indent + 2
}

// lastContent is the index of the last content line in [start, end), or
// start-1 when the range holds none, so callers can insert right after it.
func (y *yamlLines) lastContent(start, end int) int {
	last := start - 1
	for index := start; index < end && index < len(y.lines); index++ {
		if _, ok := yamlContentIndent(y.lines[index]); ok {
			last = index
		}
	}
	return last
}

// sequenceItems returns the line index of every item of the block sequence
// under the key at keyLine. Items may be indented deeper than their key or, as
// YAML also allows, sit at the key's own indentation.
func (y *yamlLines) sequenceItems(keyLine, keyIndent int) []int {
	var items []int
	for index := keyLine + 1; index < len(y.lines); index++ {
		current, ok := yamlContentIndent(y.lines[index])
		if !ok {
			continue
		}
		if _, itemIndent, isItem := yamlSequenceItem(y.lines[index]); isItem && itemIndent >= keyIndent {
			items = append(items, index)
			continue
		}
		// Anything deeper is a continuation of the item above it; anything at
		// or above the key's own indent ends the sequence.
		if current <= keyIndent {
			break
		}
	}
	return items
}

// yamlContentIndent returns the indentation of a line that carries content.
// Blank lines and whole-line comments belong to whichever block surrounds them,
// so they report no content and every scan above skips over them.
func yamlContentIndent(line string) (int, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return 0, false
	}
	return len(line) - len(trimmed), true
}

// yamlMappingKey reports the key and the inline value of a "<key>: <value>"
// line sitting at exactly indent. Only plain unquoted keys are recognized,
// which covers every key Gateway looks for.
func yamlMappingKey(line string, indent int) (string, string, bool) {
	current, ok := yamlContentIndent(line)
	if !ok || current != indent {
		return "", "", false
	}
	key, value, ok := strings.Cut(line[indent:], ":")
	if !ok || !isPlainYAMLKey(key) {
		return "", "", false
	}
	return key, yamlInlineValue(value), true
}

func isPlainYAMLKey(key string) bool {
	if key == "" {
		return false
	}
	for _, char := range key {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9', char == '_', char == '-', char == '.':
		default:
			return false
		}
	}
	return true
}

// yamlInlineValue strips the trailing comment from the value half of a mapping
// line. A flow sequence ends at its closing bracket; anything else ends where a
// comment starts.
func yamlInlineValue(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "#") {
		return ""
	}
	if strings.HasPrefix(value, "[") {
		if end := strings.LastIndex(value, "]"); end >= 0 {
			return value[:end+1]
		}
		return value
	}
	if comment := strings.Index(value, " #"); comment >= 0 {
		return strings.TrimSpace(value[:comment])
	}
	return value
}

// yamlSequenceItem reports the scalar of a "- value" line and its indent.
// Quotes are stripped so a quoted plugin name matches the plain one Gateway
// writes.
func yamlSequenceItem(line string) (string, int, bool) {
	indent, ok := yamlContentIndent(line)
	if !ok {
		return "", 0, false
	}
	rest := line[indent:]
	if rest != "-" && !strings.HasPrefix(rest, "- ") {
		return "", 0, false
	}
	return strings.Trim(yamlInlineValue(rest[1:]), `"'`), indent, true
}

// hermesPluginMembership reports whether config.yaml lists the observer in
// plugins.enabled and in plugins.disabled. A missing config lists neither.
func hermesPluginMembership(configPath string) (bool, bool, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	return hermesPluginMembershipOf(data)
}

func hermesPluginMembershipOf(data []byte) (bool, bool, error) {
	root, err := parseYAMLMapping(data)
	if err != nil {
		return false, false, err
	}
	plugins, ok := mappingValue(root, "plugins")
	if !ok {
		return false, false, nil
	}
	if plugins.Kind != yaml.MappingNode {
		return false, false, errors.New("plugins must be a mapping")
	}
	enabled, err := pluginSequence(plugins, "enabled")
	if err != nil {
		return false, false, err
	}
	disabled, err := pluginSequence(plugins, "disabled")
	if err != nil {
		return false, false, err
	}
	return sequenceContains(enabled, hermesPluginName),
		sequenceContains(disabled, hermesPluginName), nil
}

// pluginSequence returns the plugins.<key> list, or nil when the key is absent
// or empty. Removing the last entry leaves the key behind with nothing under
// it, which Hermes reads the same way it reads a missing key, so an empty list
// written that way is not an error here either.
func pluginSequence(plugins *yaml.Node, key string) (*yaml.Node, error) {
	value, ok := mappingValue(plugins, key)
	if !ok || value.Tag == "!!null" {
		return nil, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("plugins.%s must be a sequence", key)
	}
	return value, nil
}

func parseYAMLMapping(data []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}, nil
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("config root must be a mapping")
	}
	return document.Content[0], nil
}

func mappingValue(mapping *yaml.Node, key string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], true
		}
	}
	return nil, false
}

func sequenceContains(sequence *yaml.Node, value string) bool {
	if sequence == nil || sequence.Kind != yaml.SequenceNode {
		return false
	}
	for _, item := range sequence.Content {
		if item.Kind == yaml.ScalarNode && item.Value == value {
			return true
		}
	}
	return false
}
