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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// start hands the launch to "cmd /c start", which detaches the app from Gateway
// and gives a CLI the console window it needs. Its quoting rules are its own, so
// the command line is written literally rather than assembled by os/exec.
func start(target Target) error {
	command := `"` + target.Executable + `"`
	if len(target.Args) > 0 {
		command += " " + strings.Join(target.Args, " ")
	}
	line := `cmd.exe /c start "" ` + command
	if !target.Desktop {
		line = `cmd.exe /c start "" cmd.exe /k ` + command
	}

	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	return cmd.Run()
}

// stop takes the whole tree: a CLI runs under the console window that started
// it, and killing the window alone leaves the agent behind.
func stop(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
