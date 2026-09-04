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
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const imagePathMaxLen = 32768

// findProcessName resolves a pid to an executable name such as "nginx.exe".
// tasklist would enumerate every process on the machine, which takes over two
// seconds on a busy host and leaves the holder unnamed, so the kernel is asked
// about this one pid instead. tasklist stays as the fallback for a process this
// one may not open.
func findProcessName(pid int) string {
	if name := processImageName(pid); name != "" {
		return name
	}

	output := runLookup("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
	// A CSV row looks like: "nginx.exe","1234","Console","1","5,000 K"
	name := strings.TrimSpace(strings.SplitN(output, ",", 2)[0])
	return strings.Trim(name, `"`)
}

// processImageName returns the file name of a pid's executable, or "" when the
// process is gone or may not be queried.
func processImageName(pid int) string {
	if pid <= 0 {
		return ""
	}

	// QUERY_LIMITED_INFORMATION is the least that still reads an image path,
	// which is what lets this work across accounts.
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH)
	for {
		size := uint32(len(buffer))
		err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size)
		if err == nil {
			return filepath.Base(windows.UTF16ToString(buffer[:size]))
		}
		if err != windows.ERROR_INSUFFICIENT_BUFFER || len(buffer) >= imagePathMaxLen {
			return ""
		}
		buffer = make([]uint16, imagePathMaxLen)
	}
}
