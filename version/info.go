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

package version

import "runtime"

// Info is everything the web UI needs to show what this Gateway is running and
// what it could run instead.
type Info struct {
	Current Build    `json:"current"`
	Latest  *Release `json:"latest"`
	// UpdateAvailable means the published build is one this Gateway is not on.
	UpdateAvailable bool `json:"updateAvailable"`
	// CanUpdate means the update can be done from here; when it is false,
	// Blocked says why and the page falls back to the install command.
	CanUpdate bool   `json:"canUpdate"`
	Blocked   string `json:"blocked"`
	// CheckError is a failed lookup rather than a failed update: the version in
	// hand is still worth showing.
	CheckError string `json:"checkError"`
	// CheckNetwork means the lookup failed because GitHub could not be reached.
	CheckNetwork bool   `json:"checkNetwork"`
	Update       Status `json:"update"`
	ReleaseUrl   string `json:"releaseUrl"`
	// InstallCommand is the manual way in, shown when CanUpdate is false.
	InstallCommand string `json:"installCommand"`
}

// Describe answers the version page, asking GitHub again only when force says
// to. A lookup that fails is reported alongside the current version instead of
// replacing it.
func Describe(force bool) Info {
	build := Current()
	info := Info{
		Current:        build,
		Update:         UpdateStatus(),
		Blocked:        BlockedReason(),
		ReleaseUrl:     "https://github.com/" + repository + "/releases/tag/" + releaseTag,
		InstallCommand: InstallCommand(),
	}

	release, err := LatestRelease(force)
	if err != nil {
		info.CheckError = err.Error()
		info.CheckNetwork = IsNetworkError(err)
		return info
	}

	info.Latest = release
	info.UpdateAvailable = IsNewer(release, build)
	if release != nil && release.AssetUrl == "" && info.Blocked == "" {
		// The platform is one that is built for, but this particular release is
		// missing its archive.
		info.Blocked = BlockedPlatform
	}
	info.CanUpdate = info.UpdateAvailable && info.Blocked == ""

	return info
}

// InstallCommand is the one-liner that installs the published build by hand,
// for a Gateway that cannot replace its own executable.
func InstallCommand() string {
	if runtime.GOOS == "windows" {
		return "irm https://raw.githubusercontent.com/" + repository + "/master/scripts/install.ps1 | iex"
	}

	return "curl -fsSL https://raw.githubusercontent.com/" + repository + "/master/scripts/install.sh | bash"
}
