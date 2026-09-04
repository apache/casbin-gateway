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

//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// schemeMimeType is how a URL scheme is named to the desktop.
const schemeMimeType = "x-scheme-handler/" + importScheme

// registerScheme points "ccswitch://" links at the desktop entry, which is
// what declares the handler here — the entry carries the MimeType line and the
// %u that passes the link on. So there is nothing to register where the entry
// is gone: whoever deleted the shortcut is not asking for it back.
//
// Like on Windows, the scheme is taken from whatever held it only the first
// time, and what it replaced is kept for unregisterScheme to put back.
func registerScheme() error {
	entry := appId + ".desktop"

	dataDir := xdgDataHome()
	if dataDir == "" {
		return nil
	}
	applications := filepath.Join(dataDir, "applications")
	if _, err := os.Stat(filepath.Join(applications, entry)); err != nil {
		return nil
	}
	// The MimeType line is read from the index, which a hand-unpacked archive
	// writing the entry itself has not rebuilt. Best-effort: the association
	// below is written either way, and desktops that keep no index have no
	// command for it.
	_ = exec.Command("update-desktop-database", applications).Run()

	registered := schemeDefault()
	switch {
	case registered == entry:
		return nil
	case claimedScheme():
		// Ours once, and not ours now: the scheme was given back on purpose.
		return nil
	}

	if err := exec.Command("xdg-mime", "default", entry, schemeMimeType).Run(); err != nil {
		return err
	}
	return os.WriteFile(schemeMarker(), []byte(registered), 0o644)
}

// unregisterScheme hands the scheme back to whatever held it before Gateway.
// Where nothing did there is nothing to undo: removing the desktop entry is
// what takes the handler away.
func unregisterScheme() error {
	replaced, err := os.ReadFile(schemeMarker())
	if err != nil {
		return nil
	}

	if schemeDefault() == appId+".desktop" {
		if previous := strings.TrimSpace(string(replaced)); previous != "" {
			if err := exec.Command("xdg-mime", "default", previous, schemeMimeType).Run(); err != nil {
				return err
			}
		}
	}
	if err := os.Remove(schemeMarker()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// schemeDefault is the desktop entry that opens these links now, empty where
// nothing does or where the desktop has no xdg-mime to ask.
func schemeDefault() string {
	out, err := exec.Command("xdg-mime", "query", "default", schemeMimeType).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
