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

package agentauth

import (
	"os/exec"
	"strconv"
	"syscall"
)

// prepare keeps the console the agent would open to itself: the sign-in happens
// in a browser and what it prints is read from the pipe.
func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// terminate takes the whole tree. Codex is usually reached through a shim that
// runs the real program as a child, and killing the shim alone would leave that
// child holding the port the next sign-in listens on.
func terminate(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := kill.Run(); err != nil {
		// taskkill fails on a process that is already gone, which is the
		// outcome being asked for anyway.
		return cmd.Process.Kill()
	}
	return nil
}
