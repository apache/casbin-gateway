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

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework CoreServices
#import <Foundation/Foundation.h>
#import <CoreServices/CoreServices.h>
#include <stdlib.h>

#pragma clang diagnostic ignored "-Wdeprecated-declarations"
static int registerHandler(const char *path) {
	@autoreleasepool {
		NSURL *url = [NSURL fileURLWithPath:[NSString stringWithUTF8String:path] isDirectory:YES];
		return LSRegisterURL((CFURLRef)url, true);
	}
}
static char *defaultHandler(const char *scheme) {
	@autoreleasepool {
		NSString *handler = (NSString *)LSCopyDefaultHandlerForURLScheme((CFStringRef)[NSString stringWithUTF8String:scheme]);
		char *result = handler ? strdup([handler UTF8String]) : NULL;
		[handler release];
		return result;
	}
}
static int setHandler(const char *scheme, const char *identifier) {
	@autoreleasepool {
		return LSSetDefaultHandlerForURLScheme((CFStringRef)[NSString stringWithUTF8String:scheme],
			(CFStringRef)[NSString stringWithUTF8String:identifier]);
	}
}
*/
import "C"

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"
)

// The applet receives Apple events independently of the tray and webview processes.
//
//go:embed import_darwin.applescript
var importApplet string

const importAppID = appId + ".import"

func importHelper(bundle string) string {
	return filepath.Join(bundle, "Contents", "Helpers", shortcutName+" Import.app")
}

func installImportHelper(bundle string) error {
	handler := importHelper(bundle)
	if _, err := os.Stat(handler); err == nil {
		return nil
	}
	parent := filepath.Dir(handler)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(parent, ".import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	applet := filepath.Join(staging, filepath.Base(handler))
	cmd := exec.Command("/usr/bin/osacompile", "-l", "AppleScript", "-o", applet, "-")
	cmd.Stdin = strings.NewReader(importApplet)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compile import handler: %w: %s", err, out)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
	<key>CFBundleIdentifier</key><string>%s</string>
	<key>CFBundleName</key><string>%s Import</string>
	<key>CFBundleExecutable</key><string>applet</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>LSUIElement</key><true/>
	<key>OSAAppletShowStartupScreen</key><false/>
	<key>CFBundleURLTypes</key><array><dict>
		<key>CFBundleTypeRole</key><string>Viewer</string>
		<key>CFBundleURLSchemes</key><array><string>%s</string></array>
	</dict></array>
</dict></plist>
`, importAppID, shortcutName, importScheme)
	if err := os.WriteFile(filepath.Join(applet, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return err
	}
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", applet).CombinedOutput(); err != nil {
		return fmt.Errorf("sign import handler: %w: %s", err, out)
	}
	return os.Rename(applet, handler)
}

func schemeDefault() string {
	scheme := C.CString(importScheme)
	defer C.free(unsafe.Pointer(scheme))
	handler := C.defaultHandler(scheme)
	defer C.free(unsafe.Pointer(handler))
	return C.GoString(handler)
}

func setSchemeDefault(identifier string) error {
	scheme := C.CString(importScheme)
	defer C.free(unsafe.Pointer(scheme))
	handler := C.CString(identifier)
	defer C.free(unsafe.Pointer(handler))
	if status := C.setHandler(scheme, handler); status != 0 {
		return fmt.Errorf("set default import handler: Launch Services error %d", status)
	}
	return nil
}

func registerScheme() error {
	bundle, err := bundlePath()
	if err != nil {
		return err
	}
	// An omitted or deleted shortcut, or another installation's bundle.
	if readBundleHome(filepath.Join(bundle, "Contents", "MacOS")) != gatewayHome() {
		return nil
	}
	previous := schemeDefault()
	handler := importHelper(bundle)
	if _, err := os.Stat(handler); os.IsNotExist(err) {
		// Self-updates replace the launcher inside the existing app bundle.
		if err := installImportHelper(bundle); err != nil {
			return err
		}
		if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", bundle).CombinedOutput(); err != nil {
			return fmt.Errorf("sign updated application: %w: %s", err, out)
		}
	}
	path := C.CString(handler)
	defer C.free(unsafe.Pointer(path))
	if status := C.registerHandler(path); status != 0 {
		return fmt.Errorf("register import handler: Launch Services error %d", status)
	}
	if claimedScheme() {
		return nil
	}
	if strings.EqualFold(previous, importAppID) {
		previous = ""
	}
	if !strings.EqualFold(schemeDefault(), importAppID) {
		if err := setSchemeDefault(importAppID); err != nil {
			return err
		}
	}
	return os.WriteFile(schemeMarker(), []byte(previous), 0o644)
}

func unregisterScheme() error {
	previous, err := os.ReadFile(schemeMarker())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(previous) > 0 && strings.EqualFold(schemeDefault(), importAppID) {
		if err := setSchemeDefault(string(previous)); err != nil {
			return err
		}
	}
	return os.Remove(schemeMarker())
}
