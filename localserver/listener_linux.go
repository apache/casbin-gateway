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

//go:build linux

package localserver

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// tcpListen is the TCP_LISTEN state as /proc/net/tcp writes it.
const tcpListen = "0A"

// Listeners resolves listening socket inodes through procfs.
func Listeners(ctx context.Context, port int) []Process {
	inodes := listeningSocketInodes(ctx, port)
	if len(inodes) == 0 {
		return nil
	}

	var result []Process
	for _, pid := range pidsHoldingInodes(ctx, inodes) {
		if ctx.Err() != nil {
			return result
		}
		path, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			continue
		}
		result = append(result, Process{
			Pid: pid, Path: path, Owner: processOwner(pid),
		})
	}
	return result
}

// processOwner names the account a process runs as: /proc/<pid> is owned by it.
func processOwner(pid int) string {
	info, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return ""
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	id := strconv.FormatUint(uint64(stat.Uid), 10)
	account, err := user.LookupId(id)
	if err != nil {
		return id
	}
	return account.Username
}

func listeningSocketInodes(ctx context.Context, port int) map[string]bool {
	wantPort := fmt.Sprintf("%04X", port)
	inodes := map[string]bool{}
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if ctx.Err() != nil {
			return inodes
		}
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Scan() // header
		for scanner.Scan() {
			if ctx.Err() != nil {
				break
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 || !strings.EqualFold(fields[3], tcpListen) {
				continue
			}
			local := strings.Split(fields[1], ":")
			if len(local) != 2 || !strings.EqualFold(local[1], wantPort) {
				continue
			}
			inodes[fields[9]] = true
		}
		file.Close()
	}
	return inodes
}

func pidsHoldingInodes(ctx context.Context, inodes map[string]bool) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var pids []int
	for _, entry := range entries {
		if ctx.Err() != nil {
			return pids
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		descriptors, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, descriptor := range descriptors {
			link, err := os.Readlink(filepath.Join(fdDir, descriptor.Name()))
			if err != nil {
				continue
			}
			inode, ok := strings.CutPrefix(link, "socket:[")
			if !ok || !inodes[strings.TrimSuffix(inode, "]")] {
				continue
			}
			pids = append(pids, pid)
			break
		}
	}
	return pids
}
