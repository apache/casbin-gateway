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

package agentsession

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// newCommand runs the launcher with its output on a pipe and no console of its
// own. A package manager's shim is a batch file, which Windows cannot start
// directly, so those go through cmd.exe.
func newCommand(ctx context.Context, executable string, args []string) *exec.Cmd {
	switch strings.ToLower(filepath.Ext(executable)) {
	case ".cmd", ".bat":
		cmd := exec.Command("cmd.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CmdLine: batchLine(executable, args)}
		return cmd
	default:
		cmd := exec.Command(executable, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd
	}
}

// batchLine writes the command line for a shim run through cmd.exe, which parses
// what it is given a second time: os/exec quotes for the program's own runtime,
// and an argument that cmd.exe would read as punctuation - "&" and its like -
// survives that only inside quotes. So every argument is quoted here, whether it
// needs it or not, and the whole command is wrapped for "/s", which takes what
// is between the outer quotes literally.
func batchLine(executable string, args []string) string {
	line := quoteForCmd(executable)
	for _, arg := range args {
		line += " " + quoteForCmd(arg)
	}
	return `cmd.exe /s /c "` + line + `"`
}

// quoteForCmd wraps one argument in quotes, escaping the quotes and backslashes
// inside it the way a program's own runtime unpicks them.
func quoteForCmd(arg string) string {
	quoted := strings.Builder{}
	quoted.WriteByte('"')

	backslashes := 0
	for i := 0; i < len(arg); i++ {
		switch character := arg[i]; character {
		case '\\':
			backslashes++
		case '"':
			// The backslashes before a quote are doubled, then the quote itself
			// is escaped with one more.
			quoted.WriteString(strings.Repeat(`\`, backslashes*2+1))
			quoted.WriteByte('"')
			backslashes = 0
		default:
			quoted.WriteString(strings.Repeat(`\`, backslashes))
			quoted.WriteByte(character)
			backslashes = 0
		}
	}
	// Trailing backslashes would otherwise escape the closing quote.
	quoted.WriteString(strings.Repeat(`\`, backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}

// killTree ends the launcher and everything it started. Every one of these
// agents is a shim in front of a runtime, so killing the shim alone would leave
// the agent running.
func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	kill := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	kill.Run()
}
