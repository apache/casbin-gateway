// Copyright 2025 The casbin Authors. All Rights Reserved.
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

//go:build darwin

package localserver

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	lsofPath              = "/usr/sbin/lsof"
	psPath                = "/bin/ps"
	listenerCommandBudget = 3 * time.Second
)

// Listeners resolves listening processes through lsof and ps.
func Listeners(ctx context.Context, port int) []Process {
	// -t prints bare PIDs, one per line, of the listening sockets alone.
	out, ok := listenerCommand(ctx, lsofPath, "-nP", "-t", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN")
	if !ok {
		return nil
	}

	var result []Process
	seen := map[int]bool{}
	for _, line := range strings.Fields(out) {
		if ctx.Err() != nil {
			return result
		}
		pid, err := strconv.Atoi(line)
		if err != nil || pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true

		// On macOS "comm" is the executable's full path, not just its name.
		path, ok := listenerCommand(ctx, psPath, "-o", "comm=", "-p", line)
		path = strings.TrimSpace(path)
		if !ok || !filepath.IsAbs(path) {
			continue
		}
		owner, _ := listenerCommand(ctx, psPath, "-o", "user=", "-p", line)
		result = append(result, Process{Pid: pid, Path: path, Owner: strings.TrimSpace(owner)})
	}
	return result
}

func listenerCommand(ctx context.Context, path string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, listenerCommandBudget)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	return string(out), err == nil
}
