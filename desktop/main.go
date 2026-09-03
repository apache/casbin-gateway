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

// Command casbin-gateway-desktop is the desktop shell around the Gateway
// server: a tray icon that keeps the server running and a native window that
// shows its web UI. It is a separate module and a separate executable so that
// the server binary stays pure Go and cross-compiles as it does today.
//
// The window runs as a child process rather than in this one. Both the tray and
// the webview want to own the main thread and its event loop, and splitting them
// is what makes "close the window, stay in the tray" a process exit instead of a
// fight between two run loops.
package main

import (
	"fmt"
	"os"
)

func main() {
	// "shortcut on|off" is what the installers call, so that the entries the
	// first start would have created are the ones they create — or, with off,
	// the ones they leave the machine without. It prints what to open the
	// Gateway from.
	if len(os.Args) > 2 && os.Args[1] == "shortcut" {
		path, err := setShortcuts(os.Args[2] == "on")
		if err != nil {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
			os.Exit(1)
		}
		if path != "" {
			fmt.Println(path)
		}
		return
	}

	// "autostart on|off" is what the installers call, so that the login entry
	// they create is the same one the tray checkbox turns off.
	if len(os.Args) > 2 && os.Args[1] == "autostart" {
		if err := setAutostart(os.Args[2] == "on"); err != nil {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "window" {
		url := ""
		if len(os.Args) > 2 {
			url = os.Args[2]
		}
		if url == "" {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop: window needs a URL")
			os.Exit(2)
		}
		if err := runWindow(url); err != nil {
			fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
			os.Exit(1)
		}
		return
	}

	dropOwnConsole()
	runTray()
}
