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

//go:build windows

package localserver

import (
	"context"
	"encoding/binary"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	iphlpapi                = windows.NewLazySystemDLL("iphlpapi.dll")
	procGetExtendedTcpTable = iphlpapi.NewProc("GetExtendedTcpTable")
)

const (
	// TCP_TABLE_OWNER_PID_LISTENER: listening sockets with their owning PID.
	tcpTableOwnerPidListener = 3
	// Both tables are a DWORD entry count followed by fixed-size rows.
	tcpTableHeaderSize = 4
	tcpRowSize         = 24
	tcpRowPortIndex    = 8
	tcpRowPidIndex     = 20
	tcp6RowSize        = 56
	tcp6RowPortIndex   = 20
	tcp6RowPidIndex    = 52
	tcpTableAttempts   = 4
	initialTcpTable    = 32 * 1024
	maxProcessPathLen  = windows.MAX_LONG_PATH
)

// Listeners resolves listening processes through Windows TCP tables.
func Listeners(ctx context.Context, port int) []Process {
	var result []Process
	seen := map[uint32]bool{}
	for _, pid := range listeningPids(ctx, port) {
		if ctx.Err() != nil {
			return result
		}
		if pid == 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		if process, ok := describeProcess(ctx, pid); ok {
			result = append(result, process)
		}
	}
	return result
}

func listeningPids(ctx context.Context, port int) []uint32 {
	var pids []uint32
	for _, table := range []struct {
		family    uint32
		rowSize   int
		portIndex int
		pidIndex  int
	}{
		{windows.AF_INET, tcpRowSize, tcpRowPortIndex, tcpRowPidIndex},
		{windows.AF_INET6, tcp6RowSize, tcp6RowPortIndex, tcp6RowPidIndex},
	} {
		if ctx.Err() != nil {
			return pids
		}
		buffer, err := extendedTcpTable(ctx, table.family)
		if err != nil {
			continue
		}
		pids = append(pids, tablePids(buffer, port, table.rowSize, table.portIndex, table.pidIndex)...)
	}
	return pids
}

// tablePids returns PIDs for rows matching port.
func tablePids(table []byte, port, rowSize, portIndex, pidIndex int) []uint32 {
	if len(table) < tcpTableHeaderSize {
		return nil
	}
	entries := int(binary.LittleEndian.Uint32(table[:tcpTableHeaderSize]))

	var pids []uint32
	for i := 0; i < entries; i++ {
		start := tcpTableHeaderSize + i*rowSize
		if start+rowSize > len(table) {
			break
		}
		row := table[start : start+rowSize]
		if int(binary.BigEndian.Uint16(row[portIndex:portIndex+2])) != port {
			continue
		}
		pids = append(pids, binary.LittleEndian.Uint32(row[pidIndex:pidIndex+4]))
	}
	return pids
}

func extendedTcpTable(ctx context.Context, family uint32) ([]byte, error) {
	size := uint32(initialTcpTable)
	for attempt := 0; attempt < tcpTableAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		buffer := make([]byte, size)
		ret, _, _ := procGetExtendedTcpTable.Call(
			uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)),
			0, uintptr(family), tcpTableOwnerPidListener, 0)
		switch syscall.Errno(ret) {
		case 0:
			return buffer, nil
		case windows.ERROR_INSUFFICIENT_BUFFER:
			// size now holds the length the table needs; take another turn.
			continue
		default:
			return nil, syscall.Errno(ret)
		}
	}
	return nil, windows.ERROR_INSUFFICIENT_BUFFER
}

func describeProcess(ctx context.Context, pid uint32) (Process, bool) {
	if ctx.Err() != nil {
		return Process{}, false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return Process{}, false
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, maxProcessPathLen)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return Process{}, false
	}
	return Process{
		Pid:   int(pid),
		Path:  windows.UTF16ToString(buffer[:size]),
		Owner: processOwner(handle),
	}, true
}

// processOwner returns the process account as DOMAIN\user when available.
func processOwner(handle windows.Handle) string {
	var token windows.Token
	if err := windows.OpenProcessToken(handle, windows.TOKEN_QUERY, &token); err != nil {
		return ""
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return ""
	}
	account, domain, _, err := tokenUser.User.Sid.LookupAccount("")
	if err != nil {
		return ""
	}
	if domain == "" {
		return account
	}
	return domain + `\` + account
}
