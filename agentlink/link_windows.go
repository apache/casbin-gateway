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

package agentlink

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// desktopLauncher is the windowed executable beside the server, preferred as
// the handler because it has no console to flash when a link opens.
const desktopLauncher = "casbin-gateway-desktop.exe"

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procFreeConsole      = kernel32.NewProc("FreeConsole")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procShowWindow       = user32.NewProc("ShowWindow")
)

func schemesSupported() bool { return true }

// schemeKey is where Windows records the command one URL scheme opens with, for
// this account alone.
func schemeKey(scheme string) string {
	return `Software\Classes\` + scheme + `\shell\open\command`
}

func readHandler(scheme string) (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, schemeKey(scheme), registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	defer key.Close()

	value, _, err := key.GetStringValue("")
	if errors.Is(err, registry.ErrNotExist) {
		return "", nil
	}
	return value, err
}

func writeHandler(scheme string, command string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, schemeKey(scheme), registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	return key.SetStringValue("", command)
}

// handlerCommand is what Windows runs for a captured scheme.
func handlerCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	if launcher := filepath.Join(filepath.Dir(executable), desktopLauncher); isFile(launcher) {
		executable = launcher
	}
	return `"` + executable + `" ` + Subcommand + ` "%1"`, nil
}

// start runs one copy of an agent with the link appended to the command it is
// started with. The command line is written literally, which is what keeps the
// link one argument.
func start(executable string, args []string) error {
	line := `"` + executable + `"`
	for _, argument := range args {
		line += ` "` + argument + `"`
	}

	return run(executable, line)
}

// openWith runs the command the agent had registered, as Windows would have
// run it: the link takes the place of the %1 in it.
func openWith(command string, link string) error {
	executable := commandExecutable(command)
	if executable == "" {
		return errors.New("no command is recorded for this scheme")
	}
	return run(executable, strings.ReplaceAll(command, "%1", link))
}

// run starts one command line, written literally so that a link stays one
// argument. A script is not a program CreateProcess can start, so the
// interpreter that runs one is started instead, with the command quoted a
// second time for it.
func run(executable string, line string) error {
	switch strings.ToLower(filepath.Ext(executable)) {
	case ".cmd", ".bat":
		executable = filepath.Join(os.Getenv("SystemRoot"), "System32", "cmd.exe")
		line = `"` + executable + `" /s /c "` + line + `"`
	}

	cmd := exec.Command(executable)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	return cmd.Start()
}

// commandExecutable is the program a registered command line runs.
func commandExecutable(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, `"`) {
		if end := strings.Index(command[1:], `"`); end > 0 {
			return command[1 : end+1]
		}
		return ""
	}
	if space := strings.IndexAny(command, " \t"); space > 0 {
		return command[:space]
	}
	return command
}

// detachConsole drops the console a handler process would otherwise flash. The
// released server is a console program, and a link opens it without a terminal.
func detachConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(hwnd, 0) // SW_HIDE
	_, _, _ = procFreeConsole.Call()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
