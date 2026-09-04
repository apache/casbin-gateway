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

//go:build windows

package agentenv

import (
	"path/filepath"

	"github.com/apache/casbin-gateway/agenthome"
	"golang.org/x/sys/windows/registry"
)

// processFix clears a variable in the shell the agent is started from.
const processFix = `Remove-Item Env:\%s`

// The registry keys a new shell builds its environment from.
const (
	userEnvPath    = `Environment`
	machineEnvPath = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
)

// profileFiles are the PowerShell profiles, relative to the Documents folder,
// that run before the agent started from that shell does.
var profileFiles = []string{
	`PowerShell\Microsoft.PowerShell_profile.ps1`,
	`PowerShell\profile.ps1`,
	`WindowsPowerShell\Microsoft.PowerShell_profile.ps1`,
	`WindowsPowerShell\profile.ps1`,
}

// persistentSources are the places that set owner's environment again in every
// new shell: the profile that runs at startup, then the registry the session
// itself is built from.
func persistentSources(owner string, keys map[string]bool) []source {
	sources := []source{}
	if home, err := agenthome.Resolve(owner); err == nil {
		// Documents is redirected into OneDrive on many machines, and the
		// profile follows it there.
		for _, documents := range []string{filepath.Join(home, "Documents"), filepath.Join(home, "OneDrive", "Documents")} {
			for _, name := range profileFiles {
				if found := readSource(filepath.Join(documents, name), SourceProfile, keys, parsePowerShell); found != nil {
					sources = append(sources, *found)
				}
			}
		}
	}

	// Only the account Gateway runs as has its own hive loaded here; another
	// account's user variables are out of reach, and the machine ones below
	// still apply to it.
	if runsAs(owner) {
		if found := registrySource(registry.CURRENT_USER, userEnvPath, SourceUser, `[Environment]::SetEnvironmentVariable("%s", $null, "User")`, keys); found != nil {
			sources = append(sources, *found)
		}
	}
	if found := registrySource(registry.LOCAL_MACHINE, machineEnvPath, SourceMachine, `[Environment]::SetEnvironmentVariable("%s", $null, "Machine")`, keys); found != nil {
		sources = append(sources, *found)
	}
	return sources
}

func registrySource(root registry.Key, path string, kind string, fix string, keys map[string]bool) *source {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()

	vars := map[string]string{}
	for name := range keys {
		if text, _, err := key.GetStringValue(name); err == nil && text != "" {
			vars[name] = text
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return &source{kind: kind, fix: fix, vars: vars}
}
