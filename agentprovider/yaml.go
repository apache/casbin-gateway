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

	"github.com/apache/casbin-gateway/internal/yamledit"
	"gopkg.in/yaml.v3"
)

// readYAMLMapping is the root mapping of one agent's YAML configuration, ready
// to be edited in place. A file that has never been written parses to an empty
// document, which is filled rather than refused.
func readYAMLMapping(path string) (*yamledit.Document, *yaml.Node, error) {
	data, _, _, err := readFile(path)
	if err != nil {
		return nil, nil, err
	}
	document, err := yamledit.Parse(data)
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	mapping, err := document.Mapping()
	if err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return document, mapping, nil
}

// saveYAML replaces one file with the rendered document, or removes it once
// nothing is left in it. An emptied root renders as "{}" rather than as
// nothing, and a file holding that is not what the agent had before Gateway
// wrote one.
func saveYAML(changes *txn, path string, document *yamledit.Document, root *yaml.Node) error {
	if yamledit.IsEmpty(root) {
		return removeFile(path)
	}
	data, err := document.Bytes()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return removeFile(path)
	}
	return changes.write(path, data)
}

// sequenceAt is the list at key, nil when the entry is missing or holds
// something else.
func sequenceAt(mapping *yaml.Node, key string) *yaml.Node {
	node := yamledit.Get(mapping, key)
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	return node
}

// indexOfEntry is the position of the list item whose key holds want, -1 when
// the list has no such item. It is how a writer finds the entry it owns among
// the ones the file's owner wrote.
func indexOfEntry(sequence *yaml.Node, key string, want string) int {
	if sequence == nil {
		return -1
	}
	for index, item := range sequence.Content {
		if item.Kind == yaml.MappingNode && yamledit.String(item, key) == want {
			return index
		}
	}
	return -1
}
