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

// Package ccswitch reads what CC Switch keeps on this machine, so somebody
// moving to Gateway brings their providers, MCP servers, instructions and skill
// sources over instead of typing them again. Nothing here writes: CC Switch
// stays installed and unchanged.
package ccswitch

import (
	"os"
	"path/filepath"

	"github.com/apache/casbin-gateway/agenthome"
)

// dirName is the folder CC Switch keeps everything in, under the home directory
// on every platform it runs on.
const dirName = ".cc-switch"

const (
	databaseName = "cc-switch.db"
	// legacyName is the single file CC Switch kept before it moved to SQLite.
	legacyName = "config.json"
)

// Provider is one entry of CC Switch's provider list. Settings is the block it
// writes into the agent's own configuration file, in that agent's format, which
// is where the endpoint and the key are.
type Provider struct {
	Id      string
	App     string
	Name    string
	Website string
	Notes   string
	// Current marks the one CC Switch has applied for its app right now.
	Current  bool
	Settings string
}

// McpServer is one server of CC Switch's shared list, with the apps it was
// switched on for.
type McpServer struct {
	Id     string
	Name   string
	Config string
	Apps   []string
}

// Prompt is one set of instructions CC Switch keeps for an app. There is a
// library of them per app there and a single file per agent here, so only the
// enabled one has a place to go.
type Prompt struct {
	Id      string
	App     string
	Name    string
	Content string
	Enabled bool
}

// SkillRepo is a GitHub repository CC Switch installs skills from.
type SkillRepo struct {
	Owner  string
	Name   string
	Branch string
}

// Store is one CC Switch installation, read.
type Store struct {
	// Path is the folder it was read from, reported whether or not it is there.
	Path  string
	Found bool
	// Legacy marks a store still kept in config.json rather than the database.
	Legacy     bool
	Providers  []*Provider
	Mcps       []*McpServer
	Prompts    []*Prompt
	SkillRepos []*SkillRepo
}

// Dir is where CC Switch keeps its data for one account.
func Dir(owner string) (string, error) {
	home, err := agenthome.Resolve(owner)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dirName), nil
}

// Read collects everything one account's CC Switch holds. A missing
// installation is not an error: the answer is a store that was not found, which
// is what the page says.
func Read(owner string) (*Store, error) {
	dir, err := Dir(owner)
	if err != nil {
		return nil, err
	}

	store := &Store{Path: dir}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return store, nil
	}
	store.Found = true

	if err := readDatabase(filepath.Join(dir, databaseName), store); err != nil {
		return nil, err
	}
	// The database is the whole store once CC Switch has migrated to it, so the
	// file it kept before is only read when that migration never happened.
	if len(store.Providers) == 0 {
		if err := readLegacy(filepath.Join(dir, legacyName), store); err != nil {
			return nil, err
		}
	}
	return store, nil
}
