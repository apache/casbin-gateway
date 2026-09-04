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

// Package autostart turns the login entry that starts Casbin Gateway on and
// off.
//
// The entry starts the desktop launcher rather than this server, because the
// launcher is what starts the server and the tray icon together. It is the same
// entry the launcher writes for its own "Start at Login" checkbox and for the
// "autostart on" the installers call, so the Settings page and the tray icon
// show one setting rather than two.
package autostart

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// appId matches the desktop launcher's, because both write the same entry.
const appId = "org.apache.casbin-gateway"

// macOsBundleLauncher is the copy inside the application bundle, which is the
// one the Dock starts and therefore the one to start at login.
const macOsBundleLauncher = "Applications/Casbin Gateway.app/Contents/MacOS/casbin-gateway-desktop"

// ErrNoLauncher is returned where there is no desktop launcher to start: a
// container or a headless host, where a login entry has nothing to open.
var ErrNoLauncher = errors.New("this installation has no desktop launcher to start at login")

// Supported reports whether a login entry can be written on this host.
func Supported() bool {
	return LauncherPath() != ""
}

// Enabled reports whether Gateway starts at login.
func Enabled() (bool, error) {
	return enabled()
}

// Set writes or removes the login entry. Removing one needs no launcher, so an
// entry left behind by an installation that has since been stripped down can
// still be turned off.
func Set(enable bool) error {
	if !enable {
		return set("", false)
	}

	launcher := LauncherPath()
	if launcher == "" {
		return ErrNoLauncher
	}
	return set(launcher, true)
}

func launcherName() string {
	if runtime.GOOS == "windows" {
		return "casbin-gateway-desktop.exe"
	}
	return "casbin-gateway-desktop"
}

// LauncherPath is the windowed executable beside the server, or "" where the
// server was installed without one.
func LauncherPath() string {
	if bundled := bundleLauncher(); bundled != "" {
		return bundled
	}

	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}

	launcher := filepath.Join(filepath.Dir(executable), launcherName())
	if info, err := os.Stat(launcher); err != nil || info.IsDir() {
		return ""
	}
	return launcher
}

func bundleLauncher() string {
	if runtime.GOOS != "darwin" {
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	bundled := filepath.Join(home, filepath.FromSlash(macOsBundleLauncher))
	if _, err := os.Stat(bundled); err != nil {
		return ""
	}
	return bundled
}
