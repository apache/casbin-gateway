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

//go:build !windows

package agentenv

import (
	"path/filepath"

	"github.com/apache/casbin-gateway/agenthome"
)

// processFix clears a variable in the shell the agent is started from.
const processFix = "unset %s"

// startupFiles are what a shell reads before it starts an agent, in the order a
// login shell reads them.
var startupFiles = []string{
	".zshenv",
	".zprofile",
	".zshrc",
	".bash_profile",
	".bash_login",
	".bashrc",
	".profile",
	".config/fish/config.fish",
}

// persistentSources are the files that set owner's environment again in every
// new shell, so that a variable cleared in one session comes back in the next.
func persistentSources(owner string, keys map[string]bool) []source {
	home, err := agenthome.Resolve(owner)
	if err != nil {
		return nil
	}

	sources := []source{}
	for _, name := range startupFiles {
		if found := readSource(filepath.Join(home, filepath.FromSlash(name)), SourceShell, keys, parsePosix); found != nil {
			sources = append(sources, *found)
		}
	}
	if found := readSource("/etc/environment", SourceSystem, keys, parsePosix); found != nil {
		sources = append(sources, *found)
	}
	return sources
}
