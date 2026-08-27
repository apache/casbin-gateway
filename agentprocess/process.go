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

// Package agentprocess reports and controls the live processes of a discovered
// agent installation.
package agentprocess

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	listCacheTTL = 3 * time.Second
	listTimeout  = 10 * time.Second
)

// Target identifies one installation and says how it is run.
type Target struct {
	AgentId string
	Path    string
	Owner   string
	// Executable is the file to launch, empty when none was resolved.
	Executable string
	// Args are passed to the executable, literally.
	Args []string
	// Desktop marks a windowed app, which is started without a console.
	Desktop bool
}

// Status is the run state shown beside an agent installation.
type Status struct {
	Running  bool   `json:"running"`
	Pids     []int  `json:"pids"`
	CanStart bool   `json:"canStart"`
	Detail   string `json:"detail,omitempty"`
}

// Process is one live process on the host. The account it runs as is left out
// on purpose: an installation found under one account is routinely started by
// another, so the owner would rule out processes that do belong to it.
type Process struct {
	Pid     int
	Path    string
	Command string
}

var processCache = struct {
	sync.Mutex
	result       []Process
	withCommands bool
	updatedAt    time.Time
}{}

// StatusOf reports the processes belonging to one installation.
func StatusOf(target Target) Status {
	return statusOf(target, needsCommands(target))
}

// Refresh drops the cached listing, so the next status is read from the host.
// Whatever was just started or stopped is only visible after that.
func Refresh() {
	invalidate()
}

func statusOf(target Target, withCommands bool) Status {
	status := Status{Pids: []int{}, CanStart: target.Executable != ""}
	if !status.CanStart {
		status.Detail = "no launcher was found for this installation"
	}
	for _, process := range snapshot(withCommands) {
		if matches(target, process) {
			status.Pids = append(status.Pids, process.Pid)
		}
	}
	status.Running = len(status.Pids) > 0
	return status
}

// Start launches the agent: a desktop app opens on its own, a CLI opens in a
// console window, since that is the only way it is usable.
func Start(target Target) error {
	if target.Executable == "" {
		return errors.New("no launcher was found for this installation")
	}

	err := start(target)
	invalidate()
	return err
}

// Stop ends every process of the installation. The console window a CLI was
// started in owns it, and only a command line names the agent inside that
// window, so stopping always reads them however the status was reported.
func Stop(target Target) error {
	pids := statusOf(target, true).Pids
	if len(pids) == 0 {
		return nil
	}

	var failures []string
	for _, pid := range pids {
		if err := stop(pid); err != nil {
			failures = append(failures, strconv.Itoa(pid)+": "+err.Error())
		}
	}
	invalidate()

	// A process that exited between the listing and the signal is not a failure
	// worth reporting, so only a still-running one is.
	if len(failures) > 0 && statusOf(target, true).Running {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// needsCommands reports whether the installation can only be recognised by a
// command line: a package manager records the package directory rather than a
// program, and what runs is the interpreter.
func needsCommands(target Target) bool {
	return !samePath(target.Executable, target.Path)
}

// snapshot lists the host's processes, reusing a recent listing because one page
// load asks for the status of every installation. Reading command lines costs
// far more than reading image paths on some hosts, so it is only done for the
// installations that cannot be recognised without them.
func snapshot(withCommands bool) []Process {
	processCache.Lock()
	defer processCache.Unlock()

	fresh := time.Since(processCache.updatedAt) < listCacheTTL
	if fresh && (processCache.withCommands || !withCommands) {
		return processCache.result
	}

	ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
	defer cancel()
	processCache.result = list(ctx, withCommands)
	processCache.withCommands = withCommands
	processCache.updatedAt = time.Now()
	return processCache.result
}

func invalidate() {
	processCache.Lock()
	processCache.updatedAt = time.Time{}
	processCache.Unlock()
}

// matches recognises an installation among the running processes by its files:
// an agent installed by a package manager runs through an interpreter, so the
// process image is node rather than the agent, and only the command line names
// the installation.
func matches(target Target, process Process) bool {
	if process.Pid == os.Getpid() {
		return false
	}
	for _, candidate := range []string{target.Executable, target.Path} {
		if candidate == "" {
			continue
		}
		if samePath(process.Path, candidate) || containsPath(process.Command, candidate) {
			return true
		}
		if isRelocatedCopy(candidate, process.Path) {
			return true
		}
	}
	return false
}

// isRelocatedCopy recognises the copy a self-updating desktop app runs: the
// installed file stays a stub beside one directory per version, and what starts
// is the same program inside the current one.
func isRelocatedCopy(installed, path string) bool {
	if installed == "" || path == "" {
		return false
	}
	if !sameName(filepath.Base(installed), filepath.Base(path)) {
		return false
	}
	return isUnder(filepath.Dir(installed), path)
}

func sameName(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

// isUnder reports whether path sits somewhere below dir.
func isUnder(dir, path string) bool {
	dir, path = filepath.Clean(dir), filepath.Clean(path)
	if runtime.GOOS == "windows" {
		dir, path = strings.ToLower(dir), strings.ToLower(path)
	}
	relative, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func containsPath(command, path string) bool {
	if command == "" || path == "" {
		return false
	}
	path = filepath.Clean(path)
	if textContains(command, path) {
		return true
	}
	// A shim reaches its target through its own directory, so the command line
	// spells the file with a relative hop the recorded installation has not.
	for _, argument := range commandArguments(command) {
		if textContains(filepath.Clean(argument), path) {
			return true
		}
	}
	return false
}

func textContains(text, path string) bool {
	if runtime.GOOS == "windows" {
		return strings.Contains(strings.ToLower(text), strings.ToLower(path))
	}
	return strings.Contains(text, path)
}

// commandArguments splits a command line the way its quoting reads, so an
// argument holding a path with spaces stays one string.
func commandArguments(command string) []string {
	var arguments []string
	for i, part := range strings.Split(command, `"`) {
		if i%2 == 1 {
			arguments = append(arguments, part)
			continue
		}
		arguments = append(arguments, strings.Fields(part)...)
	}
	return arguments
}
