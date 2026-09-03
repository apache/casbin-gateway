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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A .lnk file has no documented format, so the shell writes it: CoCreateInstance
// makes a shell link, IShellLinkW fills it in, and IPersistFile saves it.
var (
	ole32 = syscall.NewLazyDLL("ole32.dll")

	procCoCreateInstance = ole32.NewProc("CoCreateInstance")

	clsidShellLink = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidShellLinkW  = windows.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidPersistFile = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

// The vtable slots of the methods used below, counted from QueryInterface.
const (
	comQueryInterface = 0
	comRelease        = 2

	shellLinkSetDescription      = 7
	shellLinkSetWorkingDirectory = 9
	shellLinkSetIconLocation     = 17
	shellLinkSetPath             = 20

	persistFileSave = 6
)

func installShortcuts() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	icon, err := writeAppIcon()
	if err != nil {
		return "", err
	}

	// COM is per-thread, so it is set up on a thread of this call's own rather
	// than on one the tray or the window goes on to run its own loop on.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := coInitialize(); err != nil {
		return "", err
	}
	defer windows.CoUninitialize()

	desktop, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
	if err != nil {
		return "", err
	}
	shortcut := filepath.Join(desktop, shortcutName+".lnk")
	if err := writeShellLink(shortcut, executable, icon); err != nil {
		return "", err
	}

	// The Start menu entry as well, because that is where search finds an
	// application and what "Pin to Start" pins. Its own failure is not worth
	// losing the desktop shortcut over.
	if programs, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0); err == nil {
		_ = writeShellLink(filepath.Join(programs, shortcutName+".lnk"), executable, icon)
	}
	return shortcut, nil
}

func removeShortcuts() error {
	for _, folder := range []*windows.KNOWNFOLDERID{windows.FOLDERID_Desktop, windows.FOLDERID_Programs} {
		dir, err := windows.KnownFolderPath(folder, 0)
		if err != nil {
			continue
		}
		if err := os.Remove(filepath.Join(dir, shortcutName+".lnk")); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// coInitialize treats S_FALSE, which says the thread was already initialized,
// as the success it is.
func coInitialize() error {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
	if errno, ok := err.(syscall.Errno); ok && errno == 1 {
		return nil
	}
	return err
}

func writeShellLink(path string, target string, icon string) error {
	var link unsafe.Pointer
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidShellLink)),
		0,
		windows.CLSCTX_INPROC_SERVER,
		uintptr(unsafe.Pointer(&iidShellLinkW)),
		uintptr(unsafe.Pointer(&link)),
	)
	if int32(hr) < 0 {
		return fmt.Errorf("cannot create a shell link: %s", hresult(hr))
	}
	defer comCall(link, comRelease)

	for _, field := range []struct {
		slot  uintptr
		value string
	}{
		{shellLinkSetPath, target},
		// The Gateway keeps its database, logs and temporary files in the
		// working directory, so a shortcut that starts it elsewhere would start
		// a second, empty installation.
		{shellLinkSetWorkingDirectory, gatewayHome()},
		{shellLinkSetDescription, shortcutName},
	} {
		if err := setLinkString(link, field.slot, field.value); err != nil {
			return err
		}
	}
	if err := setLinkIcon(link, icon); err != nil {
		return err
	}

	var persist unsafe.Pointer
	if err := comCall(link, comQueryInterface, uintptr(unsafe.Pointer(&iidPersistFile)), uintptr(unsafe.Pointer(&persist))); err != nil {
		return err
	}
	defer comCall(persist, comRelease)

	return saveLink(persist, path)
}

func setLinkString(link unsafe.Pointer, slot uintptr, value string) error {
	text, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return err
	}

	err = comCall(link, slot, uintptr(unsafe.Pointer(text)))
	runtime.KeepAlive(text)
	return err
}

func setLinkIcon(link unsafe.Pointer, icon string) error {
	path, err := syscall.UTF16PtrFromString(icon)
	if err != nil {
		return err
	}
	err = comCall(link, shellLinkSetIconLocation, uintptr(unsafe.Pointer(path)), 0)
	runtime.KeepAlive(path)
	return err
}

func saveLink(persist unsafe.Pointer, path string) error {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	err = comCall(persist, persistFileSave, uintptr(unsafe.Pointer(name)), 1)
	runtime.KeepAlive(name)
	return err
}

// comCall invokes the method in the object's vtable at slot, passing the object
// itself as the "this" every COM method takes first.
func comCall(object unsafe.Pointer, slot uintptr, args ...uintptr) error {
	vtable := *(**[32]uintptr)(object)
	call := append([]uintptr{uintptr(object)}, args...)

	hr, _, _ := syscall.SyscallN(vtable[slot], call...)
	if int32(hr) < 0 {
		return fmt.Errorf("the shell refused the shortcut: %s", hresult(hr))
	}
	return nil
}

func hresult(hr uintptr) string {
	return fmt.Sprintf("0x%08x", uint32(hr))
}
