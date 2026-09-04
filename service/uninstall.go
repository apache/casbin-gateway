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

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/autostart"
	"github.com/apache/casbin-gateway/util"
)

// launcherTimeout bounds the launcher call that hands back the desktop entries
// and the URL scheme.
const launcherTimeout = 30 * time.Second

// RunUninstall handles "casbin-gateway uninstall" and reports whether it did.
//
// Deleting the install directory is the part of removing Gateway anyone can do.
// What it cannot undo is everything Gateway wrote outside that directory: a
// monitoring hook in every agent's own configuration, the provider each agent
// was pointed at, the login entry, the desktop entries and the ccswitch:// URL
// scheme taken from whatever held it. A deleted directory leaves all of it
// naming a program that is not there any more.
func RunUninstall(args []string, port int) bool {
	if !isUninstall(args) {
		return false
	}

	fmt.Println()
	fmt.Println("Casbin Gateway: giving back what installing took outside this directory.")
	fmt.Println()

	stopServer(port)
	releaseAgents()
	clearAutostart()
	releaseDesktop()
	printLeftovers()
	return true
}

func isUninstall(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "uninstall" || arg == "--uninstall" {
			return true
		}
	}
	return false
}

func stopServer(port int) {
	if err := util.StopOldInstance(port); err != nil {
		var foreign *util.ForeignPortError
		if errors.As(err, &foreign) {
			fmt.Printf("  server           port %d is held by %s, left alone\n", port, foreign.Holder)
			return
		}
		fmt.Printf("  server           could not be stopped: %v\n", err)
		return
	}
	fmt.Println("  server           stopped")
}

// releaseAgents puts every agent installation back the way Gateway found it:
// monitoring hooks removed, and the provider configuration Gateway rewrote
// restored from the backup taken before it was written.
func releaseAgents() {
	installations, err := agent.Scan(true)
	if err != nil {
		fmt.Printf("  agents           could not be listed: %v\n", err)
		return
	}

	released, failures := 0, []string{}
	for _, installation := range installations {
		target := agentpatch.Target{AgentId: installation.AgentId, Path: installation.Path, Owner: installation.Owner}
		touched := false
		if err := agentpatch.Unpatch(target); err != nil {
			if !errors.Is(err, agentpatch.ErrNotSupported) {
				failures = append(failures, installation.AgentId+": "+err.Error())
			}
		} else {
			touched = true
		}
		provider := agentprovider.Target{AgentId: target.AgentId, Path: target.Path, Owner: target.Owner}
		if err := agentprovider.Restore(provider); err != nil {
			failures = append(failures, installation.AgentId+": "+err.Error())
		} else {
			touched = true
		}
		if touched {
			released++
		}
	}

	fmt.Printf("  agents           %d installations restored\n", released)
	for _, failure := range failures {
		fmt.Printf("                   %s\n", failure)
	}
}

func clearAutostart() {
	if !autostart.Supported() {
		return
	}
	if err := autostart.Set(false); err != nil {
		fmt.Printf("  start at login   could not be removed: %v\n", err)
		return
	}
	fmt.Println("  start at login   removed")
}

// releaseDesktop asks the launcher to drop the desktop entries and hand the
// ccswitch:// scheme back to whatever held it before Gateway, which is the one
// part of this only the launcher knows how to undo.
func releaseDesktop() {
	launcher := filepath.Join(filepath.Dir(executablePath()), launcherName())
	if _, err := os.Stat(launcher); err != nil {
		fmt.Println("  desktop entries  launcher not found, nothing changed")
		return
	}

	command := exec.Command(launcher, "shortcut", "off")
	command.Dir = filepath.Dir(launcher)
	timer := time.AfterFunc(launcherTimeout, func() {
		if command.Process != nil {
			_ = command.Process.Kill()
		}
	})
	defer timer.Stop()
	if err := command.Run(); err != nil {
		fmt.Printf("  desktop entries  could not be removed: %v\n", err)
		return
	}
	fmt.Println("  desktop entries  removed, ccswitch:// links handed back")
}

func printLeftovers() {
	directory := filepath.Dir(executablePath())
	fmt.Println()
	fmt.Println("Delete this directory to finish, once nothing is running from it:")
	fmt.Printf("  %s\n", directory)
	fmt.Println("Then take its \"bin\" directory off your PATH:")
	fmt.Printf("  %s\n", filepath.Join(directory, "bin"))
	fmt.Println()
}

func executablePath() string {
	path, err := os.Executable()
	if err != nil {
		return "."
	}
	return path
}

func launcherName() string {
	if runtime.GOOS == "windows" {
		return "casbin-gateway-desktop.exe"
	}
	return "casbin-gateway-desktop"
}
