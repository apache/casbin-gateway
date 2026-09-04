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

package cloudsync

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// folderName is the subdirectory Gateway makes inside whatever folder it is
// pointed at, so that a shared Dropbox never ends up with loose snapshots in
// its root.
const folderName = "Casbin Gateway"

// Folder is a directory on this machine that something already syncs, offered
// as a choice so that nobody has to find out where their OneDrive lives.
type Folder struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Suggested is Path with Gateway's own subdirectory on the end, which is
	// what picking the folder fills the field with.
	Suggested string `json:"suggested"`
}

// candidate is one place a client is known to sync, relative to the home
// directory unless it starts at the root.
type candidate struct {
	name string
	path string
}

// DetectFolders finds the synced folders this machine actually has. It only
// reports directories that exist: an empty list means the clients are not
// installed here, not that the service is unsupported.
func DetectFolders() []Folder {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	folders := []Folder{}
	seen := map[string]bool{}
	add := func(name string, path string) {
		path = ExpandPath(path)
		if path == "" || seen[strings.ToLower(path)] {
			return
		}

		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return
		}

		seen[strings.ToLower(path)] = true
		folders = append(folders, Folder{Name: name, Path: path, Suggested: filepath.Join(path, folderName)})
	}

	// The clients that name their own folder in the environment are asked
	// first: OneDrive for Business puts the tenant name in the path, and
	// guessing it is hopeless.
	for _, key := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
		if value := os.Getenv(key); value != "" {
			// Named after the folder itself: a machine signed into a personal
			// and a work account has two of these, and "OneDrive" twice over
			// tells nobody which is which.
			add(filepath.Base(ExpandPath(value)), value)
		}
	}

	for _, item := range homeCandidates() {
		add(item.name, filepath.Join(home, item.path))
	}
	for _, item := range mountCandidates() {
		add(item.name, item.path)
	}

	return folders
}

// homeCandidates is where the desktop clients put their folder. The iCloud
// path is the only odd one: on macOS the Drive is a directory inside the
// container Apple syncs, and on Windows the client makes a plain one.
func homeCandidates() []candidate {
	items := []candidate{
		{"Dropbox", "Dropbox"},
		{"OneDrive", "OneDrive"},
		{"Google Drive", filepath.Join("Google Drive", "My Drive")},
		{"Google Drive", "Google Drive"},
		{"Nextcloud", "Nextcloud"},
		{"ownCloud", "ownCloud"},
		{"Syncthing", "Sync"},
		{"Seafile", "Seafile"},
		{"Nutstore", "Nutstore"},
	}

	switch runtime.GOOS {
	case "darwin":
		items = append(items, candidate{"iCloud Drive", filepath.Join("Library", "Mobile Documents", "com~apple~CloudDocs")})
	case "windows":
		items = append(items, candidate{"iCloud Drive", "iCloudDrive"})
	}
	return items
}

func mountedUnder(dir string) []candidate {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	items := []candidate{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		items = append(items, candidate{entry.Name(), filepath.Join(dir, entry.Name())})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].name < items[j].name })
	return items
}

// defaultFolderExample is the placeholder the folder field shows. The folders
// this machine really has are offered next to the field; this is only what an
// empty one looks like, so it is guessed rather than searched for.
func defaultFolderExample() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "~"
	}
	return filepath.Join(home, "Dropbox", folderName)
}
