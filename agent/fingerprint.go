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

	ExecName string `json:"execName,omitempty"`
	// LaunchArgs are the arguments a start passes, literally: an agent that
	// refuses to run bare needs the ones that pick its default mode.
	LaunchArgs []string `json:"launchArgs,omitempty"`
	// Desktop marks a windowed app, which is launched without a console.
	Desktop bool `json:"desktop,omitempty"`

	StateDir            string              `json:"stateDir,omitempty"`
	NpmPackage          string              `json:"npmPackage,omitempty"`
	ExtraUnixNpmDirs    []string            `json:"extraUnixNpmDirs,omitempty"`
	ExtraWindowsNpmDirs []string            `json:"extraWindowsNpmDirs,omitempty"`
	WingetPackage       string              `json:"wingetPackage,omitempty"`
	MSIXFamily          string              `json:"msixFamily,omitempty"`
	DesktopInstallerDir string              `json:"desktopInstallerDir,omitempty"`
	WindowsProgramDirs  []string            `json:"windowsProgramDirs,omitempty"`
	WindowsUserDirs     []string            `json:"windowsUserDirs,omitempty"`
	HomeDirs            []string            `json:"homeDirs,omitempty"`
	HomebrewCasks       []string            `json:"homebrewCasks,omitempty"`
	SystemPackage       string              `json:"systemPackage,omitempty"`
	BuildInfoModule     string              `json:"buildInfoModule,omitempty"`
	BuildInfoVersionVar string              `json:"buildInfoVersionVar,omitempty"`
	VersionFile         string              `json:"versionFile,omitempty"`
	LocalServer         *localserver.Server `json:"localServer,omitempty"`
}
