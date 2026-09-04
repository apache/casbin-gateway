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
)

func installShortcuts() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	icon, err := writeAppIcon()
	if err != nil {
		return "", err
	}

	dataDir := xdgDataHome()
	if dataDir == "" {
		return "", fmt.Errorf("cannot find the home directory")
	}

	// The entry points at the icon by name, which only resolves once the file
	// is in the icon theme.
	iconDir := filepath.Join(dataDir, "icons", "hicolor", "512x512", "apps")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return "", err
	}
	if err := copyFile(icon, filepath.Join(iconDir, appId+".png"), 0o644); err != nil {
		return "", err
	}

	entryDir := filepath.Join(dataDir, "applications")
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		return "", err
	}
	entry := filepath.Join(entryDir, appId+".desktop")
	if err := os.WriteFile(entry, []byte(desktopEntry(executable)), 0o644); err != nil {
		return "", err
	}
	_ = exec.Command("update-desktop-database", entryDir).Run()

	installDesktopIcon(desktopEntry(executable))
	return entry, nil
}

func removeShortcuts() error {
	paths := []string{}
	if dataDir := xdgDataHome(); dataDir != "" {
		paths = append(paths,
			filepath.Join(dataDir, "applications", appId+".desktop"),
			filepath.Join(dataDir, "icons", "hicolor", "512x512", "apps", appId+".png"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Desktop", appId+".desktop"))
	}

	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func desktopEntry(executable string) string {
	// Path is where the entry starts it: the Gateway keeps its database, logs
	// and temporary files in the working directory, so an entry that started it
	// elsewhere would start a second, empty installation.
	// The %u is what hands a clicked link to the launcher, and MimeType is what
	// offers the entry as the thing that opens one. Started from the menu the
	// %u expands to nothing, which is the ordinary start.
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=Local gateway for AI agents
Exec="%s" %%u
Path=%s
Icon=%s
Terminal=false
Categories=Development;Network;
MimeType=%s;
StartupWMClass=%s-desktop
`, shortcutName, executable, gatewayHome(), appId, schemeMimeType, serverName)
}

// installDesktopIcon puts the same entry on the desktop itself. Most desktops
// only show one there if the file is executable and marked trusted; the chmod
// is the half that is portable.
func installDesktopIcon(entry string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	dir := filepath.Join(home, "Desktop")
	if _, err := os.Stat(dir); err != nil {
		return
	}
	path := filepath.Join(dir, appId+".desktop")
	if err := os.WriteFile(path, []byte(entry), 0o755); err != nil {
		return
	}
	// WriteFile only sets the mode on a file it creates, and this one may be
	// left over from an earlier install.
	_ = os.Chmod(path, 0o755)
	_ = exec.Command("gio", "set", path, "metadata::trusted", "true").Run()
}

func xdgDataHome() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}
