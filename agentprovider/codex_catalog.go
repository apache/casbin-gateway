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
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agent"
)

const (
	// codexCatalogFile is the model list Gateway owns beside config.toml. Codex
	// names a model in its picker only when its catalog carries an entry for it,
	// so a bound model that is not one of OpenAI's reads as "Custom" until this
	// file names it.
	codexCatalogFile = "casbin-gateway-models.json"
	// codexCatalogKey is the root key pointing at that file. Codex refuses to
	// start when it names a file it cannot read, so the key and the file are
	// written and taken back together.
	codexCatalogKey = "model_catalog_json"
	// codexCatalogWait bounds one run of Codex's own catalog dump.
	codexCatalogWait = 60 * time.Second
	// codexCatalogTtl is how long one built catalog answers for the next call.
	// A single switch plans and then writes, and both run Codex.
	codexCatalogTtl    = time.Minute
	codexCatalogDetail = "Served through Casbin Gateway"
)

// codexCatalogCache is the last catalog built, kept only long enough for the
// plan and the write of one switch to share it.
var codexCatalogCache = struct {
	sync.Mutex
	key   string
	data  []byte
	built bool
	time  time.Time
}{}

// codexCatalog is the catalog file for the models endpoint serves, and whether
// one could be made at all.
//
// Every entry is a copy of the one Codex itself would start on, taken from the
// installed Codex rather than written here: an entry carries the prompt and the
// tool wiring sent with each turn, and both change with the release. What comes
// out is handed back to the same Codex to parse, and a release Gateway cannot
// write for leaves the picker as it was rather than a Codex that will not start.
func codexCatalog(target Target, endpoint Endpoint) ([]byte, bool) {
	models := codexCatalogModels(endpoint)
	if len(models) == 0 {
		return nil, false
	}
	program := agent.CodexProgram(agent.Installation{AgentId: target.AgentId, Path: target.Path, Owner: target.Owner})
	if program == "" {
		return nil, false
	}

	key := program + "\n" + strings.Join(models, "\n")
	codexCatalogCache.Lock()
	defer codexCatalogCache.Unlock()
	if codexCatalogCache.key == key && time.Since(codexCatalogCache.time) < codexCatalogTtl {
		return codexCatalogCache.data, codexCatalogCache.built
	}

	data, built := codexBuildCatalog(program, models)
	codexCatalogCache.key, codexCatalogCache.data = key, data
	codexCatalogCache.built, codexCatalogCache.time = built, time.Now()
	return data, built
}

func codexBuildCatalog(program string, models []string) ([]byte, bool) {
	known, ok := codexKnownModels(program, "")
	if !ok {
		return nil, false
	}
	donor := codexDonorModel(known)
	if donor == nil {
		return nil, false
	}

	entries := make([]any, 0, len(models))
	for index, model := range models {
		entries = append(entries, codexCatalogEntry(donor, model, index))
	}
	data, err := json.MarshalIndent(map[string]any{"models": entries}, "", "  ")
	if err != nil {
		return nil, false
	}
	data = append(data, '\n')
	if !codexReadsCatalog(program, data, models[0]) {
		return nil, false
	}
	return data, true
}

// codexCatalogModels is every model to name in the catalog, the bound one first.
func codexCatalogModels(endpoint Endpoint) []string {
	models := []string{}
	seen := map[string]bool{}
	for _, model := range append([]string{endpoint.Model}, endpoint.Models...) {
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		models = append(models, model)
	}
	return models
}

// codexKnownModels is the catalog Codex resolves for itself, read against a
// directory of its own so that whatever is in the real one, this file included,
// cannot answer for it. catalogPath, when given, is the catalog to read.
func codexKnownModels(program string, catalogPath string) ([]map[string]any, bool) {
	home, err := os.MkdirTemp("", "casbin-gateway-catalog-")
	if err != nil {
		return nil, false
	}
	defer os.RemoveAll(home)

	if catalogPath != "" {
		config := tomlSetRootKey("", codexCatalogKey, catalogPath)
		if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), defaultMode); err != nil {
			return nil, false
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), codexCatalogWait)
	defer cancel()
	command := exec.CommandContext(ctx, program, "debug", "models")
	command.Env = append(os.Environ(), "CODEX_HOME="+home)
	command.Dir = home
	hideWindow(command)

	output, err := command.Output()
	if err != nil {
		return nil, false
	}
	document := struct {
		Models []map[string]any `json:"models"`
	}{}
	if err := json.Unmarshal(output, &document); err != nil || len(document.Models) == 0 {
		return nil, false
	}
	return document.Models, true
}

// codexDonorModel is the entry the copies are made from: the one Codex offers
// first, which is the one it would have started on.
func codexDonorModel(models []map[string]any) map[string]any {
	var donor map[string]any
	priority := 0
	for _, model := range models {
		if stringAt(model, "visibility") != "list" {
			continue
		}
		value, ok := model["priority"].(float64)
		if !ok {
			continue
		}
		if donor == nil || int(value) < priority {
			donor, priority = model, int(value)
		}
	}
	if donor == nil && len(models) > 0 {
		donor = models[0]
	}
	return donor
}

func codexCatalogEntry(donor map[string]any, model string, priority int) map[string]any {
	entry := map[string]any{}
	for key, value := range donor {
		entry[key] = value
	}

	entry["slug"] = model
	entry["display_name"] = model
	entry["description"] = codexCatalogDetail
	entry["visibility"] = "list"
	entry["priority"] = priority
	// None of what the copied entry says about the OpenAI model it came from
	// holds here: no announcement, no upgrade, and no faster tier to buy.
	entry["availability_nux"] = nil
	entry["upgrade"] = nil
	replacePresent(entry, "additional_speed_tiers", []any{})
	replacePresent(entry, "service_tiers", []any{})
	// Codex leaves these out of what it prints while reading a catalog without
	// them as an error, so a copy says what Codex does with a model it has no
	// entry for at all.
	fillMissing(entry, "supports_reasoning_summaries", false)
	fillMissing(entry, "supports_parallel_tool_calls", false)
	return entry
}

// codexReadsCatalog reports whether the Codex being written for parses the
// catalog back and finds the model in it.
func codexReadsCatalog(program string, data []byte, model string) bool {
	directory, err := os.MkdirTemp("", "casbin-gateway-catalog-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(directory)

	path := filepath.Join(directory, codexCatalogFile)
	if err := os.WriteFile(path, data, defaultMode); err != nil {
		return false
	}
	models, ok := codexKnownModels(program, path)
	if !ok {
		return false
	}
	for _, known := range models {
		if stringAt(known, "slug") == model {
			return true
		}
	}
	return false
}

func replacePresent(entry map[string]any, key string, value any) {
	if _, ok := entry[key]; ok {
		entry[key] = value
	}
}

func fillMissing(entry map[string]any, key string, value any) {
	if _, ok := entry[key]; !ok {
		entry[key] = value
	}
}
