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

// Package agentenv finds the environment variables that beat what Gateway
// writes into an agent's configuration file. An agent started from a shell that
// exports ANTHROPIC_BASE_URL reads that URL and not the one in settings.json, so
// a switch that wrote the file cleanly still has no effect, with nothing on
// screen saying why.
package agentenv

import (
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agenthome"
)

// Conflict is one variable set in the environment that overrides the
// configuration Gateway wrote for an agent.
type Conflict struct {
	Key string `json:"key"`
	// Value is masked when the variable carries a credential.
	Value string `json:"value"`
	// Source is the kind of place it is set in, which the UI has the wording
	// for, and Path names the file when it is set in one.
	Source string `json:"source"`
	Path   string `json:"path"`
	// Fix is the command that clears it, empty when the file above has to be
	// edited instead.
	Fix string `json:"fix"`
}

// variable is one environment variable an agent reads before its own
// configuration file.
type variable struct {
	key string
	// endpoint marks the variable that carries the base URL: a value equal to
	// the one Gateway wrote agrees with the configuration rather than fighting
	// it, and is not reported.
	endpoint bool
	secret   bool
}

// agentVars is what each agent reads from the environment. Only the variables
// that win over the configuration file belong here: reporting one an agent
// ignores would send people editing their shell for nothing.
var agentVars = map[string][]variable{
	"claude-code": {
		{key: "ANTHROPIC_BASE_URL", endpoint: true},
		{key: "ANTHROPIC_AUTH_TOKEN", secret: true},
		{key: "ANTHROPIC_API_KEY", secret: true},
		{key: "ANTHROPIC_MODEL"},
	},
	"codex": {
		{key: "OPENAI_BASE_URL", endpoint: true},
		{key: "OPENAI_API_KEY", secret: true},
	},
	"codex-cli": {
		{key: "OPENAI_BASE_URL", endpoint: true},
		{key: "OPENAI_API_KEY", secret: true},
	},
	// The Gemini CLI reads ~/.gemini/.env with dotenv rules, which never
	// replace a variable the shell already exports.
	"gemini-cli": {
		{key: "GOOGLE_GEMINI_BASE_URL", endpoint: true},
		{key: "GEMINI_API_KEY", secret: true},
		{key: "GEMINI_MODEL"},
	},
	// Qwen Code is a Gemini CLI fork and reads ~/.qwen/.env the same way, which
	// never replaces a variable the shell already exports.
	"qwen-code": {
		{key: "OPENAI_BASE_URL", endpoint: true},
		{key: "OPENAI_API_KEY", secret: true},
		{key: "OPENAI_MODEL"},
	},
	// dsh resolves the route's key by name, and the environment answers before
	// the credentials file does.
	"dsh": {
		{key: "CASBIN_GATEWAY_API_KEY", secret: true},
	},
}

// value is one variable found set, with where it was found.
type value struct {
	text   string
	source string
	path   string
	fix    string
}

// The kinds of place a variable is set in, as the UI names them.
const (
	SourceProcess = "process"
	SourceShell   = "shell"
	SourceProfile = "profile"
	SourceUser    = "user"
	SourceMachine = "machine"
	SourceSystem  = "system"
)

// source is one place the environment is set, with the variables it sets.
type source struct {
	kind string
	path string
	// fix is the command that clears a variable here, with %s for its name.
	fix  string
	vars map[string]string
}

// The scan reads a handful of files, and the agent list asks for every
// installation at once, so the result is held briefly rather than read once per
// agent per refresh.
const cacheTTL = 5 * time.Second

type cacheEntry struct {
	found map[string]value
	at    time.Time
}

var (
	cacheMutex sync.Mutex
	cache      = map[string]cacheEntry{}
)

// Check reports the variables that override what Gateway wrote into agentId's
// configuration for owner. baseUrl is the endpoint Gateway wrote, empty when it
// wrote none.
func Check(agentId string, owner string, baseUrl string) []Conflict {
	conflicts := []Conflict{}
	spec, ok := agentVars[agentId]
	if !ok {
		return conflicts
	}

	found := lookup(owner)
	for _, item := range spec {
		set, ok := found[item.key]
		if !ok || set.text == "" {
			continue
		}
		if item.endpoint && baseUrl != "" && strings.TrimRight(set.text, "/") == strings.TrimRight(baseUrl, "/") {
			continue
		}

		text := set.text
		if item.secret {
			text = maskSecret(text)
		}
		fix := set.fix
		if fix != "" {
			fix = strings.ReplaceAll(fix, "%s", item.key)
		}
		conflicts = append(conflicts, Conflict{
			Key:    item.key,
			Value:  text,
			Source: set.source,
			Path:   set.path,
			Fix:    fix,
		})
	}
	return conflicts
}

// lookup finds every variable an agent might read, in the environment owner's
// agents are started with. The first source that sets one wins, which is the
// order the shell itself resolves them in.
func lookup(owner string) map[string]value {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	if entry, ok := cache[owner]; ok && time.Since(entry.at) < cacheTTL {
		return entry.found
	}

	keys := knownKeys()
	found := map[string]value{}
	for _, src := range append(processSource(owner, keys), persistentSources(owner, keys)...) {
		for key, text := range src.vars {
			if _, taken := found[key]; taken {
				continue
			}
			found[key] = value{text: text, source: src.kind, path: src.path, fix: src.fix}
		}
	}

	cache[owner] = cacheEntry{found: found, at: time.Now()}
	return found
}

// processSource is the environment Gateway itself runs in, which is the one the
// agents of the same account inherit when they are started from the desktop.
// Another account's agents inherit nothing from it, so it is left out there.
func processSource(owner string, keys map[string]bool) []source {
	if !runsAs(owner) {
		return nil
	}

	vars := map[string]string{}
	for key := range keys {
		if text, ok := os.LookupEnv(key); ok && text != "" {
			vars[key] = text
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return []source{{kind: SourceProcess, fix: processFix, vars: vars}}
}

func runsAs(owner string) bool {
	if strings.TrimSpace(owner) == "" {
		return true
	}
	current, err := user.Current()
	if err != nil {
		return false
	}
	return agenthome.SameAccount(owner, current.Username)
}

// knownKeys is every variable any agent reads, so a scan keeps only those and
// not the rest of the environment.
func knownKeys() map[string]bool {
	keys := map[string]bool{}
	for _, spec := range agentVars {
		for _, item := range spec {
			keys[item.key] = true
		}
	}
	return keys
}

// maskSecret shows enough of a credential to recognize it, not enough to use it.
func maskSecret(text string) string {
	if len(text) <= 8 {
		return "***"
	}
	return text[:4] + "***" + text[len(text)-4:]
}
