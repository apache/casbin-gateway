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

// Package agentinstall installs, upgrades and removes agent CLIs and apps the
// way this host installed them: with a package manager, with the uninstaller
// the app registered, with the agent's own updater, or with the vendor's own
// install command. Everything it runs is built from the fingerprints compiled
// into Gateway and from what the host itself records, never from a request.
package agentinstall

import (
	"os/exec"
	"path/filepath"
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

// The ways Gateway can change an installation. The first three are also install
// methods a scan reports, which is how an action finds the manager that owns a
// tree; the rest are what is left for the installations no package manager put
// there.
const (
	ManagerNpm      = "npm"
	ManagerWinget   = "winget"
	ManagerHomebrew = "homebrew"
	// ManagerMsStore is winget's Microsoft Store source, for an app published
	// only there.
	ManagerMsStore = "msstore"
	// ManagerAppx removes a Store app by its package family.
	ManagerAppx = "appx"
	// ManagerUninstaller runs the uninstaller the app registered with Windows,
	// which is the one its own installer left behind.
	ManagerUninstaller = "uninstaller"
	// ManagerSelf runs the agent's own updater, for one that ships with it.
	ManagerSelf = "self"
	// ManagerScript runs the vendor's own install command, which installs and
	// upgrades in one.
	ManagerScript = "script"
	// ManagerFiles deletes what an install left on disk, which is all that is
	// left for one that registered nothing anywhere.
	ManagerFiles = "files"
)

// Plan is the command that would carry out one action here, and, when there is
// none, what is missing.
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
	// Interactive marks a command that puts a window on screen: an uninstaller
	// with no silent switch, or the consent prompt Windows raises for a
	// machine-wide change. It runs visibly, since hiding it would leave the job
	// waiting on a dialog nobody can answer.
	Interactive bool `json:"interactive,omitempty"`
	// Warning is what this command does that a package manager would not, for
	// the ones worth reading before clicking.
	Warning string `json:"warning,omitempty"`
	// InstallUrl is the vendor's own page, which is all that is left for an
	// agent nothing here can install.
	InstallUrl string `json:"installUrl,omitempty"`

	program string
	args    []string
}

// InstallPlan picks the way to install an agent this host does not have.
func InstallPlan(agentId string) Plan {
	return InstallVersionPlan(agentId, "")
}

// InstallVersionPlan installs a chosen release rather than the current one, for
// a machine that wants the version the rest of its fleet is on.
func InstallVersionPlan(agentId string, version string) Plan {
	if version != "" && !IsValidVersion(version) {
		return invalidVersionPlan(agentId, ActionInstall, version)
	}
	return resolve(agent.Installation{AgentId: agentId}, ActionInstall, version)
}

// UpgradePlan updates an installation in place, through whatever owns it.
func UpgradePlan(installation agent.Installation) Plan {
	return resolve(installation, ActionUpgrade, "")
}

// VersionPlan moves an installation onto one published release. Going back is
// what it is for: an agent whose new release broke something is pinned to the
// one before it.
func VersionPlan(installation agent.Installation, version string) Plan {
	action := ActionUpgrade
	if CompareVersions(version, installation.Version) < 0 {
		action = ActionDowngrade
	}
	if !IsValidVersion(version) {
		return invalidVersionPlan(installation.AgentId, action, version)
	}
	return resolve(installation, action, version)
}

// UninstallPlan removes an installation the way it was installed. Only the
// program goes: an agent's own state directory holds its sign-in and its
// history, and a reinstall finds them where it left them.
func UninstallPlan(installation agent.Installation) Plan {
	return resolve(installation, ActionUninstall, "")
}

func invalidVersionPlan(agentId string, action string, version string) Plan {
	return Plan{AgentId: agentId, Action: action, Version: version,
		InstallUrl: agent.PackagesOf(agentId).InstallUrl,
		Detail:     quote(version) + " is not a version number"}
}

// target is everything a driver needs: the installation to act on, what its
// agent is published as, and what is being asked of it.
type target struct {
	installation agent.Installation
	packages     agent.Packages
	action       string
	version      string
	// notes are why the drivers tried first could not do it, which is what the
	// answer says when none of them can.
	notes *[]string
}

func (t target) method() string { return t.installation.InstallMethod }

// pinned reports an action that names one release, which only a package manager
// can carry out: a vendor's script and an agent's own updater both install
// whatever they call current.
func (t target) pinned() bool { return t.version != "" }

func (t target) note(detail string) {
	if detail != "" {
		*t.notes = append(*t.notes, detail)
	}
}

// driver builds the command for one way of carrying an action out, and reports
// whether that way is open here.
type driver func(Plan, target) (Plan, bool)

// resolve walks the ways an action can be carried out, in the order that leaves
// the host consistent: whatever owns the installation first, then the vendor's
// own tools, then what is left on disk.
func resolve(installation agent.Installation, action string, version string) Plan {
	packages := agent.PackagesOf(installation.AgentId)
	plan := Plan{AgentId: installation.AgentId, Action: action, Version: version,
		InstallUrl: packages.InstallUrl}

	if action == ActionUninstall && installation.InstallMethod == agent.InstallMethodConfig {
		plan.Detail = "Gateway found this agent's configuration directory but not its program, so there is nothing here to remove"
		return plan
	}
	if action == ActionUninstall && installation.InstallMethod == agent.InstallMethodManual {
		plan.Detail = "Gateway was pointed at this program rather than installing it, so it is removed from the list rather than from disk"
		return plan
	}

	notes := []string{}
	spec := target{installation: installation, packages: packages,
		action: action, version: version, notes: &notes}

	for _, build := range driversFor(action) {
		if built, ok := build(plan, spec); ok {
			return built
		}
	}

	plan.Detail = unavailableDetail(spec, notes)
	return plan
}

func driversFor(action string) []driver {
	switch action {
	case ActionInstall:
		return []driver{installManagerDriver, scriptDriver, msStoreDriver}
	case ActionUninstall:
		return []driver{ownManagerDriver, uninstallerDriver, appxDriver,
			selfRemoveDriver, filesDriver}
	default:
		return []driver{ownManagerDriver, selfUpdateDriver, scriptDriver, wingetDriver, msStoreDriver}
	}
}

// installManagerDriver picks the manager that can install an agent this host
// does not have: the one it is published on, whose own program is on PATH.
func installManagerDriver(plan Plan, spec target) (Plan, bool) {
	packages := spec.packages
	for _, manager := range installOrder() {
		switch manager {
		case ManagerNpm:
			if packages.Npm == "" {
				continue
			}
			if program := lookup("npm"); program != "" {
				return fill(plan, ManagerNpm, program, "install", "-g", packages.Npm+"@"+requestedRelease(spec.version)), true
			}
			spec.note("npm publishes this agent but is not on Gateway's PATH")
		case ManagerHomebrew:
			// A cask names one version, so a pinned install has to go somewhere
			// that keeps the older ones.
			if packages.HomebrewCask == "" || spec.pinned() {
				continue
			}
			if program := lookup("brew"); program != "" {
				return fill(plan, ManagerHomebrew, program, "install", "--cask", packages.HomebrewCask), true
			}
			spec.note("Homebrew publishes this agent but is not on Gateway's PATH")
		case ManagerWinget:
			if packages.Winget == "" {
				continue
			}
			if program := lookup("winget"); program != "" {
				args := []string{"install", "--id", packages.Winget, "--exact"}
				if spec.pinned() {
					args = append(args, "--version", spec.version)
				}
				return fill(plan, ManagerWinget, program, append(args, wingetFlags...)...), true
			}
			spec.note("winget publishes this agent but is not on Gateway's PATH")
		}
	}
	return plan, false
}

// ownManagerDriver acts through the package manager that owns the tree, when
// one does: another manager would install a second copy beside it.
func ownManagerDriver(plan Plan, spec target) (Plan, bool) {
	packages := spec.packages

	switch spec.method() {
	case ManagerNpm:
		if packages.Npm == "" {
			return plan, false
		}
		program := lookup("npm")
		if program == "" {
			spec.note("npm installed this agent but is not on Gateway's PATH")
			return plan, false
		}
		if spec.action == ActionUninstall {
			return fill(plan, ManagerNpm, program, "uninstall", "-g", packages.Npm), true
		}
		return fill(plan, ManagerNpm, program, "install", "-g", packages.Npm+"@"+requestedRelease(spec.version)), true
	case ManagerHomebrew:
		if packages.HomebrewCask == "" {
			return plan, false
		}
		program := lookup("brew")
		if program == "" {
			spec.note("Homebrew installed this agent but is not on Gateway's PATH")
			return plan, false
		}
		switch {
		case spec.action == ActionUninstall:
			return fill(plan, ManagerHomebrew, program, "uninstall", "--cask", packages.HomebrewCask), true
		case !spec.pinned():
			return fill(plan, ManagerHomebrew, program, "upgrade", "--cask", packages.HomebrewCask), true
		}
		// A cask carries one version, the current one, so there is no older
		// release to ask it for.
		spec.note("Homebrew only installs the version its cask names")
		return plan, false
	case ManagerWinget:
		return wingetDriver(plan, spec)
	}
	return plan, false
}

// wingetDriver acts through winget on an installation winget did not make.
// Windows records every installer's own uninstall entry and winget matches its
// packages against them, so it upgrades and removes an app the vendor's own
// installer put there.
func wingetDriver(plan Plan, spec target) (Plan, bool) {
	if runtime.GOOS != "windows" || spec.packages.Winget == "" {
		return plan, false
	}
	program := lookup("winget")
	if program == "" {
		spec.note("winget publishes this agent but is not on Gateway's PATH")
		return plan, false
	}

	exact := []string{"--id", spec.packages.Winget, "--exact"}
	switch {
	case spec.action == ActionUninstall:
		return fill(plan, ManagerWinget, program,
			append(append([]string{"uninstall"}, exact...), wingetUninstallFlags...)...), true
	case !spec.pinned():
		return fill(plan, ManagerWinget, program,
			append(append([]string{"upgrade"}, exact...), wingetFlags...)...), true
	}
	// An older release needs the install verb: upgrade refuses to move back.
	return fill(plan, ManagerWinget, program,
		append(append([]string{"install"}, append(exact, "--version", spec.version)...), wingetFlags...)...), true
}

// msStoreDriver drives an app published only in the Microsoft Store, which
// winget installs and updates from a source of its own.
func msStoreDriver(plan Plan, spec target) (Plan, bool) {
	if runtime.GOOS != "windows" || spec.packages.MsStore == "" || spec.pinned() {
		return plan, false
	}
	program := lookup("winget")
	if program == "" {
		spec.note("this agent comes from the Microsoft Store, and winget is not on Gateway's PATH")
		return plan, false
	}

	verb := "install"
	if spec.action != ActionInstall {
		verb = "upgrade"
	}
	args := []string{verb, "--id", spec.packages.MsStore, "--exact", "--source", "msstore"}
	return fill(plan, ManagerMsStore, program, append(args, wingetFlags...)...), true
}

// selfUpdateDriver runs the updater the agent ships with, which is how an
// installation a vendor's script put there moves to a new release.
func selfUpdateDriver(plan Plan, spec target) (Plan, bool) {
	if len(spec.packages.UpdateArgs) == 0 || spec.pinned() {
		return plan, false
	}
	program := launcherOf(spec.installation)
	if program == "" {
		return plan, false
	}
	return fill(plan, ManagerSelf, program, spec.packages.UpdateArgs...), true
}

// selfRemoveDriver is the same for an agent that removes itself.
func selfRemoveDriver(plan Plan, spec target) (Plan, bool) {
	if len(spec.packages.RemoveArgs) == 0 {
		return plan, false
	}
	program := launcherOf(spec.installation)
	if program == "" {
		return plan, false
	}
	return fill(plan, ManagerSelf, program, spec.packages.RemoveArgs...), true
}

// scriptDriver runs the vendor's own install command, the one their setup page
// tells a person to paste. It installs and upgrades alike, which is what an
// agent published on no package manager here is left with.
func scriptDriver(plan Plan, spec target) (Plan, bool) {
	script := spec.packages.Script
	if script == "" || spec.pinned() {
		return plan, false
	}

	shell, args := shellCommand(script)
	if shell == "" {
		return plan, false
	}
	built := fill(plan, ManagerScript, shell, args...)
	// The wrapper is Gateway's; the command is the vendor's, and that is the
	// one worth reading before it runs.
	built.Command = script
	built.Warning = "this runs the vendor's own install command, which downloads and runs their installer"
	return built, true
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
	plan.Command = strings.TrimSpace(programName(program) + " " + strings.Join(args, " "))
	plan.Available = true
	return plan
}

// programName is what to call the program in the command a page shows: its own
// name, since the manager that chose it is named beside it anyway. Windows
// resolves npm to npm.cmd and a launcher to launcher.exe, and neither extension
// is part of what anyone would type.
func programName(program string) string {
	name := filepath.Base(program)
	for _, extension := range []string{".exe", ".cmd", ".bat", ".com", ".ps1"} {
		if strings.EqualFold(filepath.Ext(name), extension) {
			return name[:len(name)-len(extension)]
		}
	}
	return name
}

// unavailableDetail says why nothing can be done, preferring what a driver
// found over a guess from the fingerprint.
func unavailableDetail(spec target, notes []string) string {
	if len(notes) > 0 {
		return strings.Join(notes, "; ")
	}
	if spec.action == ActionInstall {
		return missingInstallDetail(spec.packages)
	}
	if spec.packages.Desktop {
		return "this app was installed as " + quote(spec.method()) +
			" and registered nothing Gateway can drive; use its own updater, or Windows Settings to remove it"
	}
	return "this agent was installed as " + quote(spec.method()) +
		", which registered no updater or uninstaller Gateway can find"
}

// missingInstallDetail says which of the two is missing: the package, or the
// manager that would install it.
func missingInstallDetail(packages agent.Packages) string {
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

func quote(value string) string {
	return "\"" + value + "\""
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
