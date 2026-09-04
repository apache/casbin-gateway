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

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const launchAgentLabel = appId + ".desktop"

func launchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func enabled() (bool, error) {
	path := launchAgentPath()
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
	path := launchAgentPath()
	if path == "" {
		return fmt.Errorf("cannot find the home directory")
	}

	if !enable {
		_ = exec.Command("launchctl", "unload", path).Run()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// The launcher starts the server in its own working directory, which is
	// where Gateway keeps its database.
	home, err := os.Getwd()
	if err != nil {
		return err
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>%s</string>
	<key>ProgramArguments</key>
	<array><string>%s</string></array>
	<key>WorkingDirectory</key><string>%s</string>
	<key>RunAtLoad</key><true/>
</dict>
</plist>
`, launchAgentLabel, launcher, home)

	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "load", path).Run()
	return nil
}
