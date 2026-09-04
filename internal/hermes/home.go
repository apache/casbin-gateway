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

package hermes

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Home is the directory Hermes keeps config.yaml, its plugins and its
// credentials in, for the user whose home directory is userHome. Windows
// installs live under %LOCALAPPDATA%\hermes, every other platform under
// ~/.hermes. HERMES_HOME and a relocated LOCALAPPDATA come from this process's
// environment, so they only speak for the account Gateway runs as.
func Home(userHome string) string {
	current := isProcessHome(userHome)
	home := filepath.Join(userHome, ".hermes")
	if runtime.GOOS == "windows" {
		home = filepath.Join(userHome, "AppData", "Local", "hermes")
		if local := os.Getenv("LOCALAPPDATA"); local != "" && current {
			home = filepath.Join(local, "hermes")
		}
	}
	if configured := os.Getenv("HERMES_HOME"); configured != "" && current {
		home = configured
	}
	return home
}

// ConfigPath is the config.yaml Hermes reads its settings from.
func ConfigPath(userHome string) string {
	return filepath.Join(Home(userHome), "config.yaml")
}

func isProcessHome(home string) bool {
	current, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	left, right := filepath.Clean(home), filepath.Clean(current)
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = resolved
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = resolved
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
