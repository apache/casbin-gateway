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

package connector

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// The catalog is embedded in the Gateway binary.
//
//go:embed connectors/*.json
var catalogFS embed.FS

const catalogDir = "connectors"

var catalog = mustLoadCatalog(catalogFS)

// mustLoadCatalog rejects invalid embedded build data at startup, the way the
// agent fingerprints do: a malformed entry is a build mistake, not something to
// discover when somebody opens the page.
func mustLoadCatalog(files fs.FS) []Connector {
	loaded, err := loadCatalog(files)
	if err != nil {
		panic("connector: cannot load connector catalog: " + err.Error())
	}
	return loaded
}

func loadCatalog(files fs.FS) ([]Connector, error) {
	names, err := fs.Glob(files, catalogDir+"/*.json")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)

	loaded := make([]Connector, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		found, err := decode(files, name)
		if err != nil {
			return nil, err
		}
		if seen[found.Id] {
			return nil, fmt.Errorf("%s: duplicate connector id %q", name, found.Id)
		}
		seen[found.Id] = true
		loaded = append(loaded, found)
	}

	sort.Slice(loaded, func(i int, j int) bool { return loaded[i].Id < loaded[j].Id })
	return loaded, nil
}

func decode(files fs.FS, name string) (Connector, error) {
	raw, err := fs.ReadFile(files, name)
	if err != nil {
		return Connector{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	found := Connector{}
	if err := decoder.Decode(&found); err != nil {
		return Connector{}, fmt.Errorf("%s: %w", name, err)
	}
	if err := validate(found); err != nil {
		return Connector{}, fmt.Errorf("%s: %w", name, err)
	}
	return found, nil
}

func validate(c Connector) error {
	switch {
	case strings.TrimSpace(c.Id) == "":
		return fmt.Errorf("a connector needs an id")
	case strings.TrimSpace(c.DisplayName) == "":
		return fmt.Errorf("%s: needs a displayName", c.Id)
	case !categories[c.Category]:
		return fmt.Errorf("%s: unknown category %q", c.Id, c.Category)
	case strings.TrimSpace(c.Server.Name) == "":
		return fmt.Errorf("%s: needs a server name", c.Id)
	}

	switch c.Auth.Kind {
	case AuthNone:
	case AuthToken:
		if len(c.Auth.Fields) == 0 {
			return fmt.Errorf("%s: a token connector needs at least one field", c.Id)
		}
	case AuthOauth2:
		if c.Auth.AuthorizeUrl == "" || c.Auth.TokenUrl == "" {
			return fmt.Errorf("%s: an oauth2 connector needs authorizeUrl and tokenUrl", c.Id)
		}
		switch c.Auth.TokenAuth {
		case "", TokenAuthBody, TokenAuthBasic:
		default:
			return fmt.Errorf("%s: unknown tokenAuth %q", c.Id, c.Auth.TokenAuth)
		}
		// The client application is the operator's, so the form has to ask for
		// it; a connector that forgets would send an authorization request with
		// no client at all.
		for _, key := range []string{KeyClientId, KeyClientSecret} {
			if !hasField(c, key) {
				return fmt.Errorf("%s: an oauth2 connector needs a %q field", c.Id, key)
			}
		}
	default:
		return fmt.Errorf("%s: unknown auth kind %q", c.Id, c.Auth.Kind)
	}

	reserved := map[string]bool{}
	for _, key := range ReservedKeys {
		reserved[key] = true
	}
	for _, field := range c.Auth.Fields {
		if strings.TrimSpace(field.Key) == "" {
			return fmt.Errorf("%s: a field needs a key", c.Id)
		}
		if reserved[field.Key] {
			return fmt.Errorf("%s: %q is filled in by Gateway and cannot be a field", c.Id, field.Key)
		}
	}

	switch c.Server.Transport {
	case "stdio":
		if c.Server.Command == "" {
			return fmt.Errorf("%s: a stdio server needs a command", c.Id)
		}
	case "http":
		if c.Server.Url == "" {
			return fmt.Errorf("%s: an http server needs a url", c.Id)
		}
	default:
		return fmt.Errorf("%s: unknown transport %q", c.Id, c.Server.Transport)
	}

	// Every placeholder a template uses has to be a field somebody fills in,
	// or the entry written into an agent carries a literal "${...}".
	declared := map[string]bool{}
	for _, field := range c.Auth.Fields {
		declared[field.Key] = true
	}
	// What an authorization produces is not a field anybody fills in, but a
	// template may refer to it.
	if c.Auth.Kind == AuthOauth2 {
		for _, key := range ReservedKeys {
			declared[key] = true
		}
	}
	for _, reference := range placeholders(c.Server) {
		if !declared[reference] {
			return fmt.Errorf("%s: server uses ${%s}, which no field supplies", c.Id, reference)
		}
	}
	return nil
}

func hasField(c Connector, key string) bool {
	for _, field := range c.Auth.Fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

// List is the whole catalog, by id.
func List() []Connector {
	found := make([]Connector, len(catalog))
	copy(found, catalog)
	return found
}

// Get finds one connector by id.
func Get(id string) (Connector, bool) {
	for _, found := range catalog {
		if found.Id == id {
			return found, true
		}
	}
	return Connector{}, false
}

// Categories is every section the catalog uses, in display order.
func Categories() []string {
	return []string{CategoryOffice, CategoryDocs, CategoryDev, CategoryPro, CategoryMail, CategoryStorage, CategoryLife}
}
