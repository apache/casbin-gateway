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
	MSIXFamily          string   `json:"msixFamily,omitempty"`
	DesktopInstallerDir string   `json:"desktopInstallerDir,omitempty"`
	WindowsProgramDirs  []string `json:"windowsProgramDirs,omitempty"`
	WindowsUserDirs     []string `json:"windowsUserDirs,omitempty"`
	HomeDirs            []string `json:"homeDirs,omitempty"`
	HomebrewCasks       []string `json:"homebrewCasks,omitempty"`
	SystemPackage       string   `json:"systemPackage,omitempty"`
	BuildInfoModule     string   `json:"buildInfoModule,omitempty"`
	BuildInfoVersionVar string   `json:"buildInfoVersionVar,omitempty"`
	VersionFile         string   `json:"versionFile,omitempty"`
	// StateVersionGlob matches JSONL files under the state directory whose
	// records carry the version of the agent that wrote them. It is the only
	// version an installation found by its configuration alone can report.
	StateVersionGlob string              `json:"stateVersionGlob,omitempty"`
	LocalServer      *localserver.Server `json:"localServer,omitempty"`
}
