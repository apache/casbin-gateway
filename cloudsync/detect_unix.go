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

//go:build !windows

package cloudsync

import (
	"os"
	"path/filepath"
	"runtime"
)

// mountCandidates is where a NAS share ends up once it is mounted. Every entry
// is one share, so they are listed rather than guessed at.
func mountCandidates() []candidate {
	if runtime.GOOS == "darwin" {
		return mountedUnder("/Volumes")
	}

	items := mountedUnder("/mnt")
	items = append(items, mountedUnder("/media")...)
	if user := os.Getenv("USER"); user != "" {
		items = append(items, mountedUnder(filepath.Join("/media", user))...)
		items = append(items, mountedUnder(filepath.Join("/run/user", user, "gvfs"))...)
	}
	return items
}
