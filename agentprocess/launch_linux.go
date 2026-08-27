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

//go:build linux

package agentprocess

import (
	"errors"
	"os/exec"
)

// terminals are tried in order, each with the flag that ends its own options.
var terminals = [][]string{
	{"x-terminal-emulator", "-e"},
	{"gnome-terminal", "--"},
	{"konsole", "-e"},
	{"xfce4-terminal", "-e"},
	{"alacritty", "-e"},
	{"xterm", "-e"},
}

func start(target Target) error {
	if target.Desktop {
		return spawn(target.Executable, target.Args...)
	}

	for _, terminal := range terminals {
		path, err := exec.LookPath(terminal[0])
		if err != nil {
			continue
		}
		arguments := append(append([]string{}, terminal[1:]...), target.Executable)
		return spawn(path, append(arguments, target.Args...)...)
	}
	return errors.New("no terminal emulator was found to run this agent in")
}
