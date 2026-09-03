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

package agentprocess

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// maxCmdlineSize limits reads from a process command line.
const maxCmdlineSize = 64 * 1024

func list(ctx context.Context, withCommands bool) []Process {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var result []Process
	for _, entry := range entries {
		if ctx.Err() != nil {
			return result
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		path, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		command, parent := "", 0
		if withCommands {
			command, parent = processCommand(pid), processParent(pid)
		}
		if path == "" && command == "" {
			continue
		}
		result = append(result, Process{Pid: pid, Parent: parent, Path: path, Command: command})
	}
	return result
}

// processParent reads the ppid procfs keeps as the fourth field of stat. The
// executable name before it may hold spaces and brackets, so the fields are read
// from after the closing one.
func processParent(pid int) int {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0
	}
	end := strings.LastIndexByte(string(data), ')')
	if end < 0 {
		return 0
	}
	fields := strings.Fields(string(data)[end+1:])
	if len(fields) < 2 {
		return 0
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0
	}
	return parent
}

// processCommand joins the NUL-separated arguments procfs stores.
func processCommand(pid int) string {
	file, err := os.Open(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return ""
	}
	defer file.Close()

	data := make([]byte, maxCmdlineSize)
	read, err := file.Read(data)
	if err != nil || read <= 0 {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(data[:read]), "\x00", " "))
}
