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

//go:build windows

package cloudsync

import (
	"golang.org/x/sys/windows"
)

// mountCandidates is the mapped network drives, which is what a NAS looks like
// on Windows. The letters are read from the drive mask rather than by opening
// each one: a disconnected mapping answers a stat only after its timeout.
func mountCandidates() []candidate {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}

	items := []candidate{}
	for letter := 'A'; letter <= 'Z'; letter++ {
		if mask&(1<<uint(letter-'A')) == 0 {
			continue
		}

		root := string(letter) + `:\`
		path, err := windows.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		if windows.GetDriveType(path) != windows.DRIVE_REMOTE {
			continue
		}
		items = append(items, candidate{string(letter) + ":", root})
	}
	return items
}
