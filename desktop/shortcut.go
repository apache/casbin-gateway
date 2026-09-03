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
	"path/filepath"
)

// shortcutName is what the entry is called wherever the desktop shows a name.
const shortcutName = "Casbin Gateway"

// shortcutMarker records that the entries were created once. Without it every
// start would put back a shortcut the user deleted on purpose.
func shortcutMarker() string {
	return filepath.Join(gatewayHome(), ".shortcuts")
}

// setShortcuts installs the desktop entries, or removes them, and records
// either way so that the next start leaves the choice alone. It returns the
// entry worth naming in the installer's output.
func setShortcuts(enabled bool) (string, error) {
	path := ""
	if enabled {
		installed, err := installShortcuts()
		if err != nil {
			return "", err
		}
		path = installed
	} else if err := removeShortcuts(); err != nil {
		return "", err
	}

	_ = os.WriteFile(shortcutMarker(), []byte(path), 0o644)
	return path, nil
}

// ensureShortcuts gives an archive that was unpacked by hand the same entries
// the installers create, on its first run. It is best-effort: a Gateway with no
// shortcut still runs, which is what the user double-clicked for.
func ensureShortcuts() {
	if _, err := os.Stat(shortcutMarker()); err == nil {
		return
	}

	if _, err := setShortcuts(true); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop: could not create the shortcut:", err)
	}
}
