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

package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/apache/casbin-gateway/util"
)

// An update leaves two files next to the executable. One says a restart into a
// new version is under way, so the Gateway it starts knows it is that version
// and what it can be rolled back to. The other says the last attempt failed, so
// the Gateway that comes back says why instead of leaving the reader with a
// dialog that never moves.
const (
	restartFileName = ".update-restart"
	failureFileName = ".update-failure"
	// failedSuffix names the executable a rollback set aside. It is deleted on
	// the next start, once nothing runs from it.
	failedSuffix = ".failed"
	// failureMaxAge keeps a file written by an update nobody watched from
	// reporting a failure at some unrelated start weeks later.
	failureMaxAge = 10 * time.Minute
)

type restartFile struct {
	// Target is the version being installed, as the web UI shows it.
	Target string `json:"target"`
	// Backup is the executable this one replaced, and is where a rollback puts
	// the installation back to.
	Backup string `json:"backup"`
}

type failureFile struct {
	Target string    `json:"target"`
	Error  string    `json:"error"`
	At     time.Time `json:"at"`
}

// startedBy is the update that started this process, or nil when it was started
// the ordinary way.
var startedBy *restartFile

// BeginStartup takes over what an update left behind: whether this process is
// the version it installed, and whether the last attempt failed.
//
// It is also where the executable a previous update replaced is deleted - but
// not when this process is an update's new version, because that executable is
// what a failed start is put back from.
func BeginStartup() {
	executable, err := executablePath()
	if err != nil {
		return
	}
	dir := filepath.Dir(executable)

	if failure := readFailure(dir); failure != nil {
		setStatus(Status{Stage: StageFailed, Target: failure.Target, Error: failure.Error, RolledBack: true})
	}

	startedBy = readRestart(dir)
	if startedBy == nil {
		CleanupBackup()
		return
	}

	// The download is finished with either way; the backup is not.
	_ = os.RemoveAll(filepath.Join(dir, stagingDir))
}

// FinishStartup is the update taking. This version is about to serve, so there
// is nothing left to roll back to.
func FinishStartup() {
	if startedBy == nil {
		return
	}

	startedBy = nil
	if executable, err := executablePath(); err == nil {
		_ = os.Remove(filepath.Join(filepath.Dir(executable), restartFileName))
	}
	CleanupBackup()
}

// RollBackFailedStart puts back the executable an update replaced and starts it,
// so a version that cannot run does not leave the machine with no Gateway at
// all. It reports whether it did, and records why for the Gateway it starts to
// show in place of the update it was watching.
func RollBackFailedStart(reason string) bool {
	if startedBy == nil {
		return false
	}

	previous := startedBy
	startedBy = nil

	executable, err := executablePath()
	if err != nil {
		return false
	}
	if _, err := os.Stat(previous.Backup); err != nil {
		return false
	}

	// The executable of a running process can be renamed on every platform, and
	// replaced in place on none, so the one that failed is moved aside first.
	failed := executable + failedSuffix
	_ = os.Remove(failed)
	if err := os.Rename(executable, failed); err != nil {
		fmt.Printf("Casbin Gateway: the version that could not start is still installed: %v\n", err)
		return false
	}
	if err := os.Rename(previous.Backup, executable); err != nil {
		_ = os.Rename(failed, executable)
		fmt.Printf("Casbin Gateway: the previous version could not be put back: %v\n", err)
		return false
	}

	rollBackDesktopLauncher(filepath.Dir(executable))

	dir := filepath.Dir(executable)
	_ = os.Remove(filepath.Join(dir, restartFileName))
	writeFailure(dir, failureFile{Target: previous.Target, Error: reason, At: time.Now()})

	fmt.Printf("Casbin Gateway: %s could not start (%s), the previous version was put back and is starting\n", previous.Target, reason)

	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = dir
	}

	statusLock.Lock()
	daemonLog := logPath
	statusLock.Unlock()

	if _, err := util.StartDetached(executable, workingDir, daemonLog); err != nil {
		fmt.Printf("Casbin Gateway: the previous version is installed but did not start: %v\n", err)
	}

	return true
}

// RollBackOnPanic is deferred by a Gateway on its way up: a version that panics
// before it serves is one this machine cannot run, so the one it replaced is
// put back rather than left dead. An ordinary start has nothing to roll back
// to, so the panic is raised again as it would have been.
func RollBackOnPanic() {
	crash := recover()
	if crash == nil {
		return
	}
	// Nothing to roll back to: the panic is raised again as it would have been,
	// stack trace and all.
	if startedBy == nil {
		panic(crash)
	}

	fmt.Printf("Casbin Gateway: %v\n%s\n", crash, debug.Stack())
	if !RollBackFailedStart(fmt.Sprintf("%v", crash)) {
		panic(crash)
	}
}

// rollBackDesktopLauncher puts back the launcher the same update replaced. It
// is best-effort: an installation with no desktop has none, and a launcher left
// one version ahead of the server still starts it.
func rollBackDesktopLauncher(installDir string) {
	launcher := filepath.Join(installDir, desktopLauncherName())
	backup := launcher + backupSuffix
	if _, err := os.Stat(backup); err != nil {
		return
	}

	failed := launcher + failedSuffix
	_ = os.Remove(failed)
	if err := os.Rename(launcher, failed); err != nil {
		return
	}
	if err := os.Rename(backup, launcher); err != nil {
		_ = os.Rename(failed, launcher)
	}
}

// noteRestart records the update about to start a new version, and undoes the
// record when that version could not be started at all.
func noteRestart(executable string, target string) {
	writeRestart(filepath.Dir(executable), restartFile{
		Target: target,
		Backup: executable + backupSuffix,
	})
}

func forgetRestart(executable string) {
	_ = os.Remove(filepath.Join(filepath.Dir(executable), restartFileName))
}

func readRestart(dir string) *restartFile {
	content, err := os.ReadFile(filepath.Join(dir, restartFileName))
	if err != nil {
		return nil
	}

	var file restartFile
	if err := json.Unmarshal(content, &file); err != nil || file.Backup == "" {
		return nil
	}

	return &file
}

func writeRestart(dir string, file restartFile) {
	content, err := json.Marshal(file)
	if err != nil {
		return
	}

	_ = os.WriteFile(filepath.Join(dir, restartFileName), content, 0o600)
}

// readFailure takes the recorded failure and removes it, so it is reported to
// whoever is watching this start and never again.
func readFailure(dir string) *failureFile {
	path := filepath.Join(dir, failureFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	_ = os.Remove(path)

	var file failureFile
	if err := json.Unmarshal(content, &file); err != nil || file.Error == "" {
		return nil
	}
	if time.Since(file.At) > failureMaxAge {
		return nil
	}

	return &file
}

func writeFailure(dir string, file failureFile) {
	content, err := json.Marshal(file)
	if err != nil {
		return
	}

	_ = os.WriteFile(filepath.Join(dir, failureFileName), content, 0o600)
}
