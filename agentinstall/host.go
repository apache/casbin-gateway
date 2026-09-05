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

package agentinstall

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/apache/casbin-gateway/agent"
)

// launcherOf is the program of an installation, for the drivers that run the
// agent itself. Empty for one found by its state directory alone, which has no
// program to run, and for a path that has since gone.
func launcherOf(installation agent.Installation) string {
	path := installation.Path
	if path == "" || installation.InstallMethod == agent.InstallMethodConfig {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

// shellCommand wraps a vendor's install command in the shell that reads it. The
// command is one string from a fingerprint, with the pipe the vendor wrote in
// it, so it needs a shell rather than an argument list.
func shellCommand(script string) (string, []string) {
	if runtime.GOOS == "windows" {
		// PowerShell 7 first: Windows PowerShell reads no proxy from its
		// environment, so a vendor script run under it would download direct
		// however the Settings page is filled in.
		program := lookup("pwsh")
		if program == "" {
			program = lookup("powershell")
		}
		if program == "" {
			return "", nil
		}
		return program, []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script}
	}

	program := lookup("bash")
	if program == "" {
		program = lookup("sh")
	}
	if program == "" {
		return "", nil
	}
	return program, []string{"-c", script}
}

// filesDriver removes what an install left on disk, which is the last thing
// left for one that registered no uninstaller anywhere. It deletes the
// directory the fingerprint declares for this agent, or, where that directory
// is shared with other programs, only the launcher in it. An agent's state
// directory is not touched either way.
func filesDriver(plan Plan, spec target) (Plan, bool) {
	launcher := launcherOf(spec.installation)
	if launcher == "" {
		return plan, false
	}
	if !ownedByUser(launcher) {
		spec.note("this installation is outside the account Gateway runs as, so it can only be removed by the account that owns it")
		return plan, false
	}

	removed, whole := agent.RemovablePathOf(spec.installation)
	if removed == "" {
		return plan, false
	}

	program, args := removeCommand(removed, whole)
	if program == "" {
		return plan, false
	}
	built := fill(plan, ManagerFiles, program, args...)
	built.Command = removeDisplay(removed, whole)
	if whole {
		built.Warning = "this deletes the whole directory the agent was installed into"
	} else {
		built.Warning = "this deletes the agent's program, which shares its directory with others"
	}
	return built, true
}

// ownedByUser keeps a removal inside the account Gateway runs as: a scan sees
// every profile on the host, and deleting out of another one would fail halfway
// through rather than cleanly.
func ownedByUser(path string) bool {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	return within(home, path)
}

// within reports whether path is inside root, comparing whole segments so that
// a directory is never confused with one whose name merely starts the same.
func within(root string, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
