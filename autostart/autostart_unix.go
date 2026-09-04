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

//go:build !windows && !darwin

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
)

// XDG autostart rather than a systemd user unit: this one starts a tray icon,
// which needs the desktop session that a unit does not wait for.
func autostartPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "autostart", appId+".desktop")
}

func enabled() (bool, error) {
	path := autostartPath()
	if path == "" {
		return false, nil
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func set(launcher string, enable bool) error {
	path := autostartPath()
	if path == "" {
		return fmt.Errorf("cannot find the config directory")
	}

	if !enable {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Casbin Gateway
Exec=%s
Icon=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, launcher, appId)

	return os.WriteFile(path, []byte(entry), 0o644)
}
