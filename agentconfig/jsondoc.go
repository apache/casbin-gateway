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

package agentconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// loadJSONObject parses a JSON object, returning an empty map for an absent or
// blank file so callers can create one. A non-object root is an error rather
// than silently discarded, since overwriting it would destroy user data.
func loadJSONObject(existing []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(existing)) == 0 {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(existing, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if doc == nil {
		return nil, fmt.Errorf("parse config: root must be a JSON object")
	}
	return doc, nil
}

// marshalJSONObject renders doc with two-space indentation and a trailing
// newline, matching the formatting agent tools write themselves. Map keys are
// sorted by encoding/json, so the output is deterministic.
func marshalJSONObject(doc map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// ensureObject returns the nested object at key, creating an empty one when the
// key is absent. A non-object value at key is replaced, because the adapter owns
// that slot (e.g. Claude Code's "env").
func ensureObject(doc map[string]any, key string) map[string]any {
	if nested, ok := doc[key].(map[string]any); ok {
		return nested
	}
	nested := map[string]any{}
	doc[key] = nested
	return nested
}

// setOrDelete sets key to value, or deletes it when value is empty, so an
// unset provider field clears the stale entry instead of leaving the previous
// provider's value behind.
func setOrDelete(obj map[string]any, key, value string) {
	if strings.TrimSpace(value) == "" {
		delete(obj, key)
		return
	}
	obj[key] = value
}
