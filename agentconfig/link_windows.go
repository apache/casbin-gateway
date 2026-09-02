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

package agentconfig

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// makeLink points target at the source folder. Windows only lets an
// unprivileged process make a symbolic link with Developer Mode on, so a
// directory junction is the fallback: it needs no privilege, and every program
// reading the folder — the agents included — sees the files behind it exactly
// as a symbolic link would show them.
func makeLink(from string, target string) error {
	symlinkErr := os.Symlink(from, target)
	if symlinkErr == nil {
		return nil
	}

	// mklink is a shell builtin rather than a program, so it is run through
	// cmd. /J makes the junction.
	command := exec.Command("cmd", "/c", "mklink", "/J", target, from)
	output, err := command.CombinedOutput()
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s, and a directory junction failed too: %s",
		symlinkErr, strings.TrimSpace(string(output)))
}
