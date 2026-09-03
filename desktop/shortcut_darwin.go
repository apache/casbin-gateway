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

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// macOS has no desktop shortcuts to speak of; the application bundle in
// ~/Applications is what Launchpad, Spotlight and the Dock look for.
func installShortcuts() (string, error) {
	bundle, err := bundlePath()
	if err != nil {
		return "", err
	}
	// A start from inside the bundle has nothing left to install, and would
	// otherwise delete the launcher it is running.
	if inBundle(bundle) {
		return bundle, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	icon, err := writeAppIcon()
	if err != nil {
		return "", err
	}

	if err := os.RemoveAll(bundle); err != nil {
		return "", err
	}
	macOsDir := filepath.Join(bundle, "Contents", "MacOS")
	resources := filepath.Join(bundle, "Contents", "Resources")
	for _, dir := range []string{macOsDir, resources} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	// The launcher is copied in rather than symlinked or wrapped: a bundle
	// whose executable runs from outside it gets neither the Dock icon nor the
	// name. Which is also why the data directory is recorded next to it — the
	// copy cannot find it by looking beside itself.
	if err := copyFile(executable, filepath.Join(macOsDir, serverName+"-desktop"), 0o755); err != nil {
		return "", err
	}
	if err := copyFile(icon, filepath.Join(resources, "appicon.icns"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(resources, "home"), []byte(gatewayHome()+"\n"), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(infoPlist()), 0o644); err != nil {
		return "", err
	}

	// The build is not signed or notarized, and an ad-hoc signature is the most
	// this can do without a developer certificate. Finder caches the bundle by
	// path, so a reinstall over an old one has to say the directory changed or
	// it keeps showing the old icon. All three are best-effort.
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", bundle).Run()
	_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", bundle).Run()
	now := time.Now()
	_ = os.Chtimes(bundle, now, now)

	return bundle, nil
}

func bundlePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Applications", shortcutName+".app"), nil
}

func inBundle(bundle string) bool {
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	return strings.HasPrefix(executable, bundle+string(filepath.Separator))
}

func removeShortcuts() error {
	bundle, err := bundlePath()
	if err != nil {
		return err
	}

	// Removing the bundle a start came out of would delete the running
	// launcher, so that one is left where it is.
	if inBundle(bundle) {
		return nil
	}
	return os.RemoveAll(bundle)
}

func infoPlist() string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>%s</string>
	<key>CFBundleDisplayName</key><string>%s</string>
	<key>CFBundleIdentifier</key><string>%s</string>
	<key>CFBundleExecutable</key><string>%s-desktop</string>
	<key>CFBundleIconFile</key><string>appicon</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`, shortcutName, shortcutName, appId, serverName)
}
