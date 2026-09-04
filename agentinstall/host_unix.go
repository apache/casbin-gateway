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

//go:build !windows

package agentinstall

// uninstallerDriver has nothing to read off Windows: a registered uninstaller
// is a Windows idea, and everywhere else an installation is either a package or
// the files it left.
func uninstallerDriver(plan Plan, _ target) (Plan, bool) { return plan, false }

// appxDriver is the same for Store packages.
func appxDriver(plan Plan, _ target) (Plan, bool) { return plan, false }

// removeCommand deletes a path, through rm so that what it did is in the job's
// own output rather than only in an error.
func removeCommand(path string, whole bool) (string, []string) {
	program := lookup("rm")
	if program == "" {
		return "", nil
	}
	if whole {
		return program, []string{"-rf", path}
	}
	return program, []string{"-f", path}
}

func removeDisplay(path string, whole bool) string {
	if whole {
		return "rm -rf " + path
	}
	return "rm -f " + path
}
