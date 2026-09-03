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

package agentprocess

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// commandScript reads what the process was started with, which is the only way
// an agent running under an interpreter names itself. WMI answers this in about
// a second, so it is asked for only when an image path cannot settle it.
const commandScript = `Get-CimInstance Win32_Process | ForEach-Object { [PSCustomObject]@{ pid = $_.ProcessId; parent = $_.ParentProcessId; path = $_.ExecutablePath; command = $_.CommandLine } } | ConvertTo-Json -Compress`

func list(ctx context.Context, withCommands bool) []Process {
	if withCommands {
		if result := listWithCommands(ctx); len(result) > 0 {
			return result
		}
	}
	return listImages()
}

// listImages walks the process table for image paths alone, which the OS
// answers directly.
func listImages() []Process {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if windows.Process32First(snapshot, &entry) != nil {
		return nil
	}

	var result []Process
	for {
		if entry.ProcessID > 0 {
			if path := imagePath(entry.ProcessID); path != "" {
				result = append(result, Process{
					Pid: int(entry.ProcessID), Parent: int(entry.ParentProcessID), Path: path,
				})
			}
		}
		if windows.Process32Next(snapshot, &entry) != nil {
			return result
		}
	}
}

// imagePath is empty for a process this one may not open, such as one owned by
// another account when Gateway is not elevated.
func imagePath(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size) != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

func listWithCommands(ctx context.Context) []Process {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", commandScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	body := strings.TrimSpace(string(output))
	// ConvertTo-Json writes a bare object when only one process came back.
	if strings.HasPrefix(body, "{") {
		body = "[" + body + "]"
	}
	var rows []struct {
		Pid     int    `json:"pid"`
		Parent  int    `json:"parent"`
		Path    string `json:"path"`
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(body), &rows) != nil {
		return nil
	}

	result := make([]Process, 0, len(rows))
	for _, row := range rows {
		if row.Pid <= 0 {
			continue
		}
		result = append(result, Process{Pid: row.Pid, Parent: row.Parent, Path: row.Path, Command: row.Command})
	}
	return result
}
