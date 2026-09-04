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
	"unsafe"

	"golang.org/x/sys/windows"
)

func list(_ context.Context, withCommands bool) []Process {
	processes := listImages()
	if !withCommands {
		return processes
	}
	for i := range processes {
		processes[i].Command = commandLine(uint32(processes[i].Pid))
	}
	return processes
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

// commandLine reads what a process was started with, which is the only way an
// agent running under an interpreter names itself. It comes out of the
// process's own parameter block: asking WMI for the whole table instead costs
// about three seconds on a host with a thousand processes, and that is the
// wait the agents page used to open with.
//
// An empty answer is normal — a process this one may not open, or a 32-bit one
// whose parameter block is laid out differently — and means the installation is
// recognised by its image path alone, as it is when commands are not read.
func commandLine(pid uint32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	var wow64 bool
	if windows.IsWow64Process(handle, &wow64) != nil || wow64 {
		return ""
	}

	var info windows.PROCESS_BASIC_INFORMATION
	if windows.NtQueryInformationProcess(handle, windows.ProcessBasicInformation,
		unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), nil) != nil {
		return ""
	}

	var peb windows.PEB
	if !readMemory(handle, uintptr(unsafe.Pointer(info.PebBaseAddress)), unsafe.Pointer(&peb), unsafe.Sizeof(peb)) {
		return ""
	}
	var parameters windows.RTL_USER_PROCESS_PARAMETERS
	if !readMemory(handle, uintptr(unsafe.Pointer(peb.ProcessParameters)), unsafe.Pointer(&parameters), unsafe.Sizeof(parameters)) {
		return ""
	}

	// Length is a uint16, so it bounds the read on its own.
	length := parameters.CommandLine.Length
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, length/2)
	if !readMemory(handle, uintptr(unsafe.Pointer(parameters.CommandLine.Buffer)), unsafe.Pointer(&buffer[0]), uintptr(length)) {
		return ""
	}
	return windows.UTF16ToString(buffer)
}

func readMemory(handle windows.Handle, address uintptr, into unsafe.Pointer, size uintptr) bool {
	if address == 0 {
		return false
	}
	var read uintptr
	if windows.ReadProcessMemory(handle, address, (*byte)(into), size, &read) != nil {
		return false
	}
	return read == size
}
