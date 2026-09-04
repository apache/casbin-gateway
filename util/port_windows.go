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

// imagePathMaxLen is the buffer a full executable path is read into. Windows
// paths reach 32767 characters, but a process image path that long cannot be
// launched in the first place, so one MAX_PATH-sized try and one large one are
// all this ever needs.
const imagePathMaxLen = 32768

// findProcessName resolves a pid to an executable name such as "nginx.exe".
//
// The obvious way to do this on Windows is "tasklist", but tasklist enumerates
// and formats every process on the machine: on a host with a thousand of them
// it takes over two seconds, and a lookup that times out reports no name at
// all. That is not a cosmetic loss - an unnamed holder is treated as a foreign
// program, so "stop" refuses to stop Gateway's own server and a restart cannot
// reclaim its own port. Asking the kernel about the one pid we care about
// answers in microseconds and cannot be starved by an unrelated process count.
func findProcessName(pid int) string {
	if name := processImageName(pid); name != "" {
		return name
	}

	// A process owned by another account can refuse to be opened. tasklist runs
	// with the same rights, but it is worth one slow try before giving up.
	output := runLookup("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
	// A CSV row looks like: "nginx.exe","1234","Console","1","5,000 K"
	name := strings.TrimSpace(strings.SplitN(output, ",", 2)[0])
	return strings.Trim(name, `"`)
}

// processImageName reads one process's executable path straight from the
// kernel, and returns just its file name. It returns "" when the process is
// gone or this one may not query it.
func processImageName(pid int) string {
	if pid <= 0 {
		return ""
	}

	// QUERY_LIMITED_INFORMATION is the least this can ask for and still read an
	// image path, which is what lets it work across accounts.
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
