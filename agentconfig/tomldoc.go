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
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
)

// loadTOML parses a TOML document into a map tree, returning an empty map for an
// absent or blank file. Unknown tables and keys are preserved on the round trip
// so an adapter only rewrites the slots it owns.
//
// Note: decoding to a map does not retain comments, so a rewrite drops any
// comments a user placed in the file. Preserving those would require a
// comment-aware AST, which is out of scope here.
func loadTOML(existing []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(existing)) == 0 {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal(existing, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if doc == nil {
		return map[string]any{}, nil
	}
	return doc, nil
}

// marshalTOML renders doc back to TOML. go-toml/v2 sorts map keys, so the output
// is deterministic.
func marshalTOML(doc map[string]any) ([]byte, error) {
	return toml.Marshal(doc)
}

// ensureTOMLTable returns the nested table at key, creating an empty one when it
// is absent or not a table.
func ensureTOMLTable(doc map[string]any, key string) map[string]any {
	if nested, ok := doc[key].(map[string]any); ok {
		return nested
	}
	nested := map[string]any{}
	doc[key] = nested
	return nested
}
