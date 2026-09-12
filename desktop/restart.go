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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/systray"
)

// handoverEnvKey carries what a launcher on its way out tells the one taking
// over. It is read once and cleared, so nothing started from here inherits it.
const handoverEnvKey = "CASBIN_GATEWAY_HANDOVER"

const (
	handoverServer = "server"
	handoverWindow = "window"
	// backupSuffix is what an update renames the launcher it replaces to.
	launcherBaseName = "casbin-gateway-desktop"
	backupSuffix     = ".old"
)

type handover struct {
	taken      bool
	ownsServer bool
	window     bool
}

var handedOver handover

func takeHandover() handover {
	value, found := os.LookupEnv(handoverEnvKey)
	if !found {
		return handover{}
	}
	_ = os.Unsetenv(handoverEnvKey)

	state := handover{taken: true}
	for _, part := range strings.Split(value, ",") {
		switch part {
		case handoverServer:
			state.ownsServer = true
		case handoverWindow:
			state.window = true
		}
	}
	return state
}

// launcherPath is the executable a restart starts. Linux reports the name the
// binary this process runs from was renamed to, which is not the one to start.
func launcherPath() string {
	if executable, err := os.Executable(); err == nil {
		path := strings.TrimSuffix(strings.TrimSuffix(executable, " (deleted)"), backupSuffix)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	name := launcherBaseName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(gatewayHome(), name)
}

// restartLauncher starts the launcher an update just put in place and leaves.
// The server behind it stays up: this is the shell catching up with the build
// it shows, which otherwise waits for someone to close the app and open it.
func restartLauncher() {
	window.Lock()
	hadWindow := window.process != nil
	window.Unlock()

	closeWindow()
	// The marker goes first, so that the launcher started below holds the
	// socket rather than handing its launch back to this process.
	releaseInstance()

	state := []string{}
	ownsServer.Lock()
	if ownsServer.value {
		state = append(state, handoverServer)
	}
	ownsServer.Unlock()
	if hadWindow {
		state = append(state, handoverWindow)
	}

	cmd := exec.Command(launcherPath())
	cmd.Dir = gatewayHome()
	cmd.Env = append(os.Environ(), handoverEnvKey+"="+strings.Join(state, ","))
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop: could not restart into the new version:", err)
		restoreInstance()
		if hadWindow {
			showWindow()
		}
		return
	}

	systray.Quit()
}
