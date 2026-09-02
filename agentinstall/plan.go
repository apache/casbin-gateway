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

// Package agentinstall installs and upgrades agent CLIs with the package
// managers the host already has. Everything it runs is built from the
// fingerprints compiled into Gateway, never from a request.
package agentinstall

import (
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agent"
)

// The actions a plan describes.
const (
	ActionInstall = "install"
	ActionUpgrade = "upgrade"
)

// The package managers Gateway can drive. They are also the install methods a
// scan reports, which is how an upgrade finds the one that owns a tree.
const (
	ManagerNpm      = "npm"
	ManagerWinget   = "winget"
	ManagerHomebrew = "homebrew"
)

// Plan is the command that would install or upgrade one agent here, and, when
// there is none, what is missing.
type Plan struct {
	AgentId string `json:"agentId"`
	Action  string `json:"action"`
	Manager string `json:"manager,omitempty"`
	// Command is the command line as it will run, shown before it does.
	Command   string `json:"command,omitempty"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
	// InstallUrl is the vendor's own page, which is all that is left for an
	// agent no package manager here can install.
	InstallUrl string `json:"installUrl,omitempty"`

	program string
	args    []string
}

// InstallPlan picks the manager that can install an agent this host does not
// have: the one it is published on, whose own program is on PATH.
func InstallPlan(agentId string) Plan {
	packages := agent.PackagesOf(agentId)
	plan := Plan{AgentId: agentId, Action: ActionInstall, InstallUrl: packages.InstallUrl}

	for _, manager := range installOrder() {
		switch manager {
		case ManagerNpm:
			if packages.Npm == "" {
				continue
			}
			if program := lookup("npm"); program != "" {
				return fill(plan, ManagerNpm, program, "install", "-g", packages.Npm+"@latest")
			}
		case ManagerHomebrew:
			if packages.HomebrewCask == "" {
				continue
			}
			if program := lookup("brew"); program != "" {
				return fill(plan, ManagerHomebrew, program, "install", "--cask", packages.HomebrewCask)
			}
		case ManagerWinget:
			if packages.Winget == "" {
				continue
			}
			if program := lookup("winget"); program != "" {
				return fill(plan, ManagerWinget, program, append([]string{"install", "--id", packages.Winget, "--exact"}, wingetFlags...)...)
			}
		}
	}

	plan.Detail = unavailableDetail(packages)
	return plan
}

// UpgradePlan upgrades an installation in place, through the manager that put
// it there: another one would install a second copy beside it.
func UpgradePlan(agentId string, installMethod string) Plan {
	packages := agent.PackagesOf(agentId)
	plan := Plan{AgentId: agentId, Action: ActionUpgrade, InstallUrl: packages.InstallUrl}

	switch installMethod {
	case ManagerNpm:
		if packages.Npm == "" {
			break
		}
		if program := lookup("npm"); program != "" {
			return fill(plan, ManagerNpm, program, "install", "-g", packages.Npm+"@latest")
		}
		plan.Detail = "npm installed this agent but is not on Gateway's PATH"
		return plan
	case ManagerHomebrew:
		if packages.HomebrewCask == "" {
			break
		}
		if program := lookup("brew"); program != "" {
			return fill(plan, ManagerHomebrew, program, "upgrade", "--cask", packages.HomebrewCask)
		}
		plan.Detail = "Homebrew installed this agent but is not on Gateway's PATH"
		return plan
	case ManagerWinget:
		if packages.Winget == "" {
			break
		}
		if program := lookup("winget"); program != "" {
			return fill(plan, ManagerWinget, program, append([]string{"upgrade", "--id", packages.Winget, "--exact"}, wingetFlags...)...)
		}
		plan.Detail = "winget installed this agent but is not on Gateway's PATH"
		return plan
	}

	plan.Detail = "this agent was installed as \"" + installMethod + "\", which Gateway cannot upgrade; use its own updater"
	return plan
}

// wingetFlags keep an install unattended: winget otherwise stops on a licence
// prompt no one is there to answer.
var wingetFlags = []string{
	"--silent",
	"--accept-package-agreements",
	"--accept-source-agreements",
	"--disable-interactivity",
}

// installOrder prefers the manager that installs into the user's own account,
// since Gateway does not run as an administrator: npm with a user prefix on
// Windows and Linux, Homebrew on macOS, where it owns its own tree.
func installOrder() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{ManagerHomebrew, ManagerNpm}
	case "windows":
		return []string{ManagerNpm, ManagerWinget}
	default:
		return []string{ManagerNpm, ManagerHomebrew}
	}
}

func fill(plan Plan, manager string, program string, args ...string) Plan {
	plan.Manager = manager
	plan.program = program
	plan.args = args
	plan.Command = manager + " " + strings.Join(args, " ")
	plan.Available = true
	return plan
}

// unavailableDetail says which of the two is missing: the package, or the
// manager that would install it.
func unavailableDetail(packages agent.Packages) string {
	var managers []string
	if packages.Npm != "" {
		managers = append(managers, "npm")
	}
	if packages.HomebrewCask != "" && runtime.GOOS != "windows" {
		managers = append(managers, "Homebrew")
	}
	if packages.Winget != "" && runtime.GOOS == "windows" {
		managers = append(managers, "winget")
	}
	if len(managers) == 0 {
		if packages.Desktop {
			return "this is a desktop app, downloaded from its vendor rather than installed by a package manager"
		}
		return "no package manager on this platform publishes this agent"
	}
	return "install " + strings.Join(managers, " or ") + " first, or install the agent from its vendor's page"
}

// lookupTTL bounds how long a PATH search is trusted, so a manager installed
// after Gateway started is picked up without a restart.
const lookupTTL = 30 * time.Second

var lookups = struct {
	sync.Mutex
	found map[string]lookupResult
}{found: map[string]lookupResult{}}

type lookupResult struct {
	program string
	at      time.Time
}

// lookup resolves a manager to an absolute program, which is what the job runs:
// a bare name would be looked up again in whatever directory it is started in.
// Every agent listing asks for the same few managers, and a PATH search walks
// every directory on it, so the answers are held briefly.
func lookup(name string) string {
	lookups.Lock()
	defer lookups.Unlock()

	if cached, ok := lookups.found[name]; ok && time.Since(cached.at) < lookupTTL {
		return cached.program
	}
	program, err := exec.LookPath(name)
	if err != nil {
		program = ""
	}
	lookups.found[name] = lookupResult{program: program, at: time.Now()}
	return program
}
