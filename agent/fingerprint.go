// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package agent

import "github.com/apache/casbin-gateway/localserver"

// Fingerprint describes known installation layouts for one agent.
type Fingerprint struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// InstallUrl is the vendor's own install page, which is all Gateway has to
	// offer for an agent this host has not installed.
	InstallUrl string `json:"installUrl,omitempty"`

	ExecName string `json:"execName,omitempty"`
	// LaunchArgs are the arguments a start passes, literally: an agent that
	// refuses to run bare needs the ones that pick its default mode.
	LaunchArgs []string `json:"launchArgs,omitempty"`
	// Desktop marks a windowed app, which is launched without a console.
	Desktop bool `json:"desktop,omitempty"`
	// Sandboxed marks an agent that runs its sessions in a sandbox of its own,
	// where 127.0.0.1 is the sandbox rather than this host.
	Sandboxed bool `json:"sandboxed,omitempty"`
	// CustomScan marks an agent the scan locates in code rather than from the
	// layout fields below: one installed by cloning its own repository, which
	// no package layout describes. The fingerprint still carries the identity
	// every lookup by agent id goes through.
	CustomScan bool `json:"customScan,omitempty"`

	// InstanceArg is the flag a private state directory is passed on the command
	// line with, for an app that takes one - Chromium's --user-data-dir.
	// InstanceEnv is the variable that names it for an agent that reads its
	// state directory from the environment. Either one lets Gateway run a second
	// copy of the agent signed in to a different account; neither means it
	// cannot.
	InstanceArg string `json:"instanceArg,omitempty"`
	InstanceEnv string `json:"instanceEnv,omitempty"`

	// LinkScheme is the URL scheme the agent registers for its own links, which
	// is how a browser hands a finished sign-in back to it.
	LinkScheme string `json:"linkScheme,omitempty"`

	StateDir            string   `json:"stateDir,omitempty"`
	NpmPackage          string   `json:"npmPackage,omitempty"`
	ExtraUnixNpmDirs    []string `json:"extraUnixNpmDirs,omitempty"`
	ExtraWindowsNpmDirs []string `json:"extraWindowsNpmDirs,omitempty"`
	WingetPackage       string   `json:"wingetPackage,omitempty"`
	// MsStorePackage is the Microsoft Store product id, which is how winget
	// installs an agent published nowhere else.
	MsStorePackage      string `json:"msStorePackage,omitempty"`
	MSIXFamily          string `json:"msixFamily,omitempty"`
	DesktopInstallerDir string `json:"desktopInstallerDir,omitempty"`
	// BundledDirs are directories another app keeps its own copy of this agent
	// in, under the account's application data - %APPDATA% on Windows,
	// ~/Library/Application Support on macOS, ~/.config elsewhere. A "*" segment
	// is the directory these apps keep one of per release.
	BundledDirs         []string `json:"bundledDirs,omitempty"`
	WindowsProgramDirs  []string `json:"windowsProgramDirs,omitempty"`
	WindowsUserDirs     []string `json:"windowsUserDirs,omitempty"`
	HomeDirs            []string `json:"homeDirs,omitempty"`
	HomebrewCasks       []string `json:"homebrewCasks,omitempty"`
	SystemPackage       string   `json:"systemPackage,omitempty"`
	BuildInfoModule     string   `json:"buildInfoModule,omitempty"`
	BuildInfoVersionVar string   `json:"buildInfoVersionVar,omitempty"`
	VersionFile         string   `json:"versionFile,omitempty"`
	// IgnoreExecutableVersion marks a launcher whose Windows version resource
	// belongs to whatever packaged it rather than to the agent, which is every
	// single-file Node build: it carries the packager's number, and reading it
	// would report a release the vendor never published.
	IgnoreExecutableVersion bool `json:"ignoreExecutableVersion,omitempty"`
	// StateVersionGlob matches JSONL files under the state directory whose
	// records carry the version of the agent that wrote them. It is the only
	// version an installation found by its configuration alone can report.
	StateVersionGlob string `json:"stateVersionGlob,omitempty"`
	// StateVersionField is the dotted path to that version within a record, for
	// an agent that does not keep it at the top level under "version".
	StateVersionField string `json:"stateVersionField,omitempty"`
	// StateIgnore names the entries an agent writes on its own, without being
	// installed here: what an app that embeds the agent leaves behind. A state
	// directory holding nothing else is not evidence of an installation.
	StateIgnore []string            `json:"stateIgnore,omitempty"`
	LocalServer *localserver.Server `json:"localServer,omitempty"`

	// UpdateArgs are what the agent's own launcher takes to update itself, for
	// one that ships an updater. It is the only way to move an installation no
	// package manager owns, which is what a vendor's install script leaves.
	UpdateArgs []string `json:"updateArgs,omitempty"`
	// RemoveArgs are the same for removing it.
	RemoveArgs []string `json:"removeArgs,omitempty"`
	// InstallScript is the vendor's own documented one-line installer, which
	// both installs and upgrades. It runs through the platform shell, so it is
	// shown in full before anyone clicks it.
	InstallScript *InstallScript `json:"installScript,omitempty"`

	// Headless is the agent's own non-interactive mode, which is what Gateway
	// drives it through. Nil for an agent that publishes none: it can still be
	// watched and configured, just not driven.
	Headless *Headless `json:"headless,omitempty"`
}

// InstallScript is the vendor's installer command per platform. An empty
// platform means the vendor publishes no script for it.
type InstallScript struct {
	Windows string `json:"windows,omitempty"`
	Darwin  string `json:"darwin,omitempty"`
	Linux   string `json:"linux,omitempty"`
}
