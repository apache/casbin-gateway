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
	ActionInstall   = "install"
	ActionUpgrade   = "upgrade"
	ActionDowngrade = "downgrade"
	ActionUninstall = "uninstall"
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
	Command string `json:"command,omitempty"`
	// Version is the release a pinned install asks for, empty for the plans
	// that take whatever the manager calls latest.
	Version   string `json:"version,omitempty"`
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
	return InstallVersionPlan(agentId, "")
}

// InstallVersionPlan installs a chosen release rather than the current one, for
// a machine that wants the version the rest of its fleet is on.
func InstallVersionPlan(agentId string, version string) Plan {
	if version != "" && !IsValidVersion(version) {
		return Plan{AgentId: agentId, Action: ActionInstall, Version: version,
			InstallUrl: agent.PackagesOf(agentId).InstallUrl,
			Detail:     "\"" + version + "\" is not a version number"}
	}
	return installPlan(agentId, version)
}

func installPlan(agentId string, version string) Plan {
	packages := agent.PackagesOf(agentId)
	plan := Plan{AgentId: agentId, Action: ActionInstall, Version: version, InstallUrl: packages.InstallUrl}

	for _, manager := range installOrder() {
		switch manager {
		case ManagerNpm:
			if packages.Npm == "" {
				continue
			}
			if program := lookup("npm"); program != "" {
				return fill(plan, ManagerNpm, program, "install", "-g", packages.Npm+"@"+requestedRelease(version))
			}
		case ManagerHomebrew:
			// A cask names one version, so a pinned install has to go somewhere
			// that keeps the older ones.
			if packages.HomebrewCask == "" || version != "" {
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
				args := []string{"install", "--id", packages.Winget, "--exact"}
				if version != "" {
					args = append(args, "--version", version)
				}
				return fill(plan, ManagerWinget, program, append(args, wingetFlags...)...)
			}
		}
	}

	plan.Detail = unavailableDetail(packages)
	return plan
}

// UpgradePlan upgrades an installation in place, through the manager that put
// it there: another one would install a second copy beside it.
func UpgradePlan(agentId string, installMethod string) Plan {
	return managerPlan(agentId, installMethod, ActionUpgrade, "")
}

// VersionPlan moves an installation onto one published release, through the
// manager that owns its tree. Going back is what it is for: an agent whose new
// release broke something is pinned to the one before it.
func VersionPlan(agentId string, installMethod string, version string, current string) Plan {
	action := ActionUpgrade
	if CompareVersions(version, current) < 0 {
		action = ActionDowngrade
	}

	plan := managerPlan(agentId, installMethod, action, version)
	if !IsValidVersion(version) {
		return Plan{AgentId: agentId, Action: action, Version: version,
			InstallUrl: plan.InstallUrl, Detail: "\"" + version + "\" is not a version number"}
	}
	return plan
}

// UninstallPlan removes an installation with the manager that installed it.
// Only the program goes: an agent's own state directory holds its sign-in and
// its history, and a reinstall finds them where it left them.
func UninstallPlan(agentId string, installMethod string) Plan {
	return managerPlan(agentId, installMethod, ActionUninstall, "")
}

// managerPlan builds the command for one action against an installation whose
// manager is known. Everything but the version comes from the fingerprint.
func managerPlan(agentId string, installMethod string, action string, version string) Plan {
	packages := agent.PackagesOf(agentId)
	plan := Plan{AgentId: agentId, Action: action, Version: version, InstallUrl: packages.InstallUrl}

	switch installMethod {
	case ManagerNpm:
		if packages.Npm == "" {
			break
		}
		program := lookup("npm")
		if program == "" {
			plan.Detail = "npm installed this agent but is not on Gateway's PATH"
			return plan
		}
		if action == ActionUninstall {
			return fill(plan, ManagerNpm, program, "uninstall", "-g", packages.Npm)
		}
		return fill(plan, ManagerNpm, program, "install", "-g", packages.Npm+"@"+requestedRelease(version))
	case ManagerHomebrew:
		if packages.HomebrewCask == "" {
			break
		}
		program := lookup("brew")
		if program == "" {
			plan.Detail = "Homebrew installed this agent but is not on Gateway's PATH"
			return plan
		}
		switch action {
		case ActionUninstall:
			return fill(plan, ManagerHomebrew, program, "uninstall", "--cask", packages.HomebrewCask)
		case ActionUpgrade:
			if version == "" {
				return fill(plan, ManagerHomebrew, program, "upgrade", "--cask", packages.HomebrewCask)
			}
		}
		// A cask carries one version, the current one, so there is no older
		// release to ask it for.
		plan.Detail = "Homebrew only installs the version its cask names; use the vendor's own downloads for an older one"
		return plan
	case ManagerWinget:
		if packages.Winget == "" {
			break
		}
		program := lookup("winget")
		if program == "" {
			plan.Detail = "winget installed this agent but is not on Gateway's PATH"
			return plan
		}
		exact := []string{"--id", packages.Winget, "--exact"}
		switch action {
		case ActionUninstall:
			return fill(plan, ManagerWinget, program, append(append([]string{"uninstall"}, exact...), wingetUninstallFlags...)...)
		case ActionUpgrade:
			if version == "" {
				return fill(plan, ManagerWinget, program, append(append([]string{"upgrade"}, exact...), wingetFlags...)...)
			}
		}
		// An older version needs the install verb: upgrade refuses to move back.
		return fill(plan, ManagerWinget, program,
			append(append([]string{"install"}, append(exact, "--version", version)...), wingetFlags...)...)
	}

	plan.Detail = "this agent was installed as \"" + installMethod + "\", which Gateway cannot " + action + "; use its own updater"
	return plan
}

// requestedRelease is the npm tag or version an install asks for.
func requestedRelease(version string) string {
	if version == "" {
		return "latest"
	}
	return version
}

// IsValidVersion accepts what a package manager would recognise as a release
// and nothing else: a version reaches here from a request, and it is the one
// part of a command that Gateway does not build from a fingerprint.
func IsValidVersion(version string) bool {
	if version == "" || len(version) > 64 || (version[0] < '0' || version[0] > '9') {
		return false
	}
	for i := 0; i < len(version); i++ {
		c := version[i]
		digit := c >= '0' && c <= '9'
		letter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		if !digit && !letter && c != '.' && c != '-' && c != '+' && c != '_' {
			return false
		}
	}
	return true
}

// wingetFlags keep an install unattended: winget otherwise stops on a licence
// prompt no one is there to answer.
var wingetFlags = []string{
	"--silent",
	"--accept-package-agreements",
	"--accept-source-agreements",
	"--disable-interactivity",
}

// wingetUninstallFlags are the ones an uninstall takes: it has no package
// agreement to accept, and rejects the flag that would accept one.
var wingetUninstallFlags = []string{
	"--silent",
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
