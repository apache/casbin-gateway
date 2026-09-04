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

package util

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// browseLimit bounds one listing: nobody picks a program out of a longer one.
const browseLimit = 2000

// BrowseEntry is one file or directory of a listing.
type BrowseEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size,omitempty"`
	// Executable marks a file this host would run, the only kind worth choosing.
	Executable bool `json:"executable,omitempty"`
}

// BrowseListing is one directory as a file picker needs it.
type BrowseListing struct {
	Path string `json:"path"`
	// Parent is empty at the top of a tree, where there is nowhere to go up to.
	Parent string `json:"parent"`
	Home   string `json:"home"`
	// Roots are where a walk can start: the drives on Windows, "/" elsewhere.
	Roots   []string      `json:"roots"`
	Entries []BrowseEntry `json:"entries"`
	// Truncated marks a directory with more in it than was listed.
	Truncated bool `json:"truncated,omitempty"`
}

// BrowseDir lists one directory of this host. An empty path starts in the home
// of the account Gateway runs as.
func BrowseDir(path string) (*BrowseListing, error) {
	home, _ := os.UserHomeDir()
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if path == "" {
		path = home
	}
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path = abs
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	listing := &BrowseListing{Path: path, Home: home, Roots: browseRoots(),
		Entries: []BrowseEntry{}}
	if parent := filepath.Dir(path); parent != path {
		listing.Parent = parent
	}
	for _, entry := range entries {
		if len(listing.Entries) >= browseLimit {
			listing.Truncated = true
			break
		}
		item := BrowseEntry{Name: entry.Name(), Path: filepath.Join(path, entry.Name()),
			Dir: entry.IsDir()}
		if !item.Dir {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			// A symlink to a program is worth choosing; a socket or device is not.
			if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				target, err := os.Stat(item.Path)
				if err != nil {
					continue
				}
				item.Dir = target.IsDir()
				info = target
			}
			item.Size = info.Size()
			item.Executable = !item.Dir && isExecutableFile(item.Path, info)
		}
		listing.Entries = append(listing.Entries, item)
	}

	sort.Slice(listing.Entries, func(i, j int) bool {
		left, right := listing.Entries[i], listing.Entries[j]
		if left.Dir != right.Dir {
			return left.Dir
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	return listing, nil
}
