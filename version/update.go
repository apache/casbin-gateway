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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/util"
)

// Stage is where an update has got to. The web UI polls for it, so these
// strings are part of the API.
type Stage string

const (
	StageIdle        Stage = "idle"
	StageDownloading Stage = "downloading"
	StageInstalling  Stage = "installing"
	StageRestarting  Stage = "restarting"
	StageFailed      Stage = "failed"
)

// Reasons an installation cannot update itself. The web UI turns them into
// something the reader can act on, so they are part of the API too.
const (
	BlockedNoExecutable = "no-executable"
	BlockedPlatform     = "unsupported-platform"
	BlockedReadOnly     = "read-only"
)

// Status is the progress of the running update, or of the last one that failed.
type Status struct {
	Stage      Stage  `json:"stage"`
	Percent    int    `json:"percent"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Target     string `json:"target"`
	Error      string `json:"error"`
	// Network means the failure was reaching GitHub, which a proxy can fix.
	Network bool `json:"network"`
}

// stagingDir holds the download and the unpacked executable. It sits next to
// the running executable so that putting the new one in place is a rename on
// one filesystem, which cannot half-finish.
const stagingDir = ".update"

// backupSuffix names the executable that was replaced. It is kept until the
// updated Gateway has started, and deleted on its first run.
const backupSuffix = ".old"

// maxBinarySize bounds what is unpacked, so a corrupt archive cannot fill the
// disk.
const maxBinarySize = 512 << 20

// restartDelay gives the API response time to reach the browser before this
// process goes away.
const restartDelay = 700 * time.Millisecond

// The executable an update replaced is still running for a moment after the new
// one starts, and Windows will not delete a file that is open, so removing it
// has to be attempted more than once.
const (
	cleanupAttempts = 15
	cleanupInterval = 2 * time.Second
)

var (
	statusLock sync.Mutex
	status     = Status{Stage: StageIdle}
	logPath    = "./logs/casbin-gateway.out"
)

// BeforeRestart is what the process about to be replaced does first, so that an
// update does not drop what it was still holding in memory.
var BeforeRestart func()

// Configure points an update at the log the restarted Gateway writes to.
func Configure(daemonLogPath string) {
	statusLock.Lock()
	defer statusLock.Unlock()

	logPath = daemonLogPath
}

// UpdateStatus is the progress of the update running in this process.
func UpdateStatus() Status {
	statusLock.Lock()
	defer statusLock.Unlock()

	return status
}

func setStatus(next Status) {
	statusLock.Lock()
	defer statusLock.Unlock()

	status = next
}

func fail(err error) {
	statusLock.Lock()
	defer statusLock.Unlock()

	status.Stage = StageFailed
	status.Error = err.Error()
	status.Network = IsNetworkError(err)
}

// BlockedReason says why this installation cannot replace its own executable,
// and is empty when it can.
func BlockedReason() string {
	if AssetName(runtime.GOOS, runtime.GOARCH) == "" {
		return BlockedPlatform
	}

	executable, err := executablePath()
	if err != nil {
		return BlockedNoExecutable
	}

	// Whether the executable can be replaced is a property of the directory
	// holding it, not of the file: replacing it is a rename, not a write.
	probe, err := os.CreateTemp(filepath.Dir(executable), ".update-probe-*")
	if err != nil {
		return BlockedReadOnly
	}
	probe.Close()
	os.Remove(probe.Name())

	return ""
}

// executablePath is where this executable actually lives, with any symlink
// followed: replacing the link rather than its target would leave the real
// executable behind.
func executablePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return executable, nil
	}

	return resolved, nil
}

// StartUpdate downloads the published build, puts it in place of this one and
// restarts into it. It returns as soon as the work is under way; UpdateStatus
// reports the rest.
func StartUpdate() error {
	statusLock.Lock()
	running := status.Stage == StageDownloading || status.Stage == StageInstalling || status.Stage == StageRestarting
	statusLock.Unlock()

	if running {
		return errors.New("an update is already running")
	}

	if reason := BlockedReason(); reason != "" {
		return errors.New(describeBlocked(reason))
	}

	release, err := LatestRelease(false)
	if err != nil {
		return err
	}
	if release == nil || release.AssetUrl == "" {
		return fmt.Errorf("no build is published for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if !IsNewer(release, Current()) {
		return errors.New("this Gateway is already the published build")
	}

	setStatus(Status{
		Stage:  StageDownloading,
		Target: releaseLabel(release),
		Total:  release.AssetSize,
	})

	go func() {
		if err := runUpdate(release); err != nil {
			fail(err)
		}
	}()

	return nil
}

func describeBlocked(reason string) string {
	switch reason {
	case BlockedPlatform:
		return fmt.Sprintf("no build is published for %s/%s, so this Gateway cannot update itself", runtime.GOOS, runtime.GOARCH)
	case BlockedReadOnly:
		return "the directory holding this executable cannot be written to, so it cannot be replaced"
	default:
		return "this Gateway cannot find its own executable, so it cannot replace it"
	}
}

func releaseLabel(release *Release) string {
	if release.ShortCommit == "" {
		return release.Tag
	}

	return release.Tag + " " + release.ShortCommit
}

func runUpdate(release *Release) error {
	executable, err := executablePath()
	if err != nil {
		return err
	}

	staging := filepath.Join(filepath.Dir(executable), stagingDir)
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	archive := filepath.Join(staging, release.AssetName)
	if err := download(release, archive); err != nil {
		return err
	}

	statusLock.Lock()
	status.Stage = StageInstalling
	status.Percent = 100
	statusLock.Unlock()

	staged := filepath.Join(staging, filepath.Base(executable))
	if err := extractBinary(archive, staged); err != nil {
		return err
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return err
	}

	// The archive is only known to hold a working Gateway once one has run, and
	// after the swap below there is no one left to notice that it does not.
	if err := smokeTest(staged); err != nil {
		return fmt.Errorf("the downloaded build does not run, so it was not installed: %w", err)
	}

	if err := swap(executable, staged); err != nil {
		return err
	}

	// The desktop launcher ships in the same archive. It is replaced after the
	// server, and separately, so that a failure there leaves an updated Gateway
	// rather than an aborted update.
	updateDesktopLauncher(archive, staging, filepath.Dir(executable))

	statusLock.Lock()
	status.Stage = StageRestarting
	statusLock.Unlock()

	return restart(executable)
}

// smokeTest runs the downloaded executable with the one sub-command that exits
// on its own, which catches a truncated download or an archive built for
// another platform.
func smokeTest(path string) error {
	cmd := exec.Command(path, "version")
	cmd.Dir = filepath.Dir(path)
	// A copy of this environment would carry the daemon marker into a process
	// that is only meant to print a line and exit.
	cmd.Env = []string{}

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		return errors.New("it did not answer within 30s")
	}
}

// swap puts the new executable where the old one was. The old one is renamed
// rather than deleted: it is the file this very process runs from, which
// Windows will not let go of, and keeping it means the rename can be undone.
func swap(executable string, staged string) error {
	backup := executable + backupSuffix
	_ = os.Remove(backup)

	if err := os.Rename(executable, backup); err != nil {
		return fmt.Errorf("could not move the running executable aside: %w", err)
	}

	if err := os.Rename(staged, executable); err != nil {
		// Nothing has been replaced yet, so putting the old one back leaves the
		// installation exactly as it was.
		_ = os.Rename(backup, executable)
		return fmt.Errorf("could not put the new executable in place: %w", err)
	}

	return nil
}

// restart starts the updated Gateway and ends this process. The new one stops
// whatever still holds the port, so the two never fight over it.
func restart(executable string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = filepath.Dir(executable)
	}

	statusLock.Lock()
	daemonLog := logPath
	statusLock.Unlock()

	if _, err := util.StartDetached(executable, workingDir, daemonLog); err != nil {
		return fmt.Errorf("the new executable is installed but did not start: %w", err)
	}

	go func() {
		time.Sleep(restartDelay)
		if BeforeRestart != nil {
			BeforeRestart()
		}
		os.Exit(0)
	}()

	return nil
}

// CleanupBackup removes what an update left behind. It runs at startup and
// keeps trying in the background for as long as the process it replaced takes
// to go away, rather than leaving a second copy of the executable on disk until
// the next restart.
func CleanupBackup() {
	executable, err := executablePath()
	if err != nil {
		return
	}

	_ = os.RemoveAll(filepath.Join(filepath.Dir(executable), stagingDir))

	removeWhenUnlocked(executable + backupSuffix)
	removeWhenUnlocked(filepath.Join(filepath.Dir(executable), desktopLauncherName()+backupSuffix))
}

// removeWhenUnlocked deletes a replaced executable once nothing runs from it,
// which on Windows is only true after the process using it has exited.
func removeWhenUnlocked(backup string) {
	if _, err := os.Stat(backup); err != nil {
		return
	}

	go func() {
		for attempt := 0; attempt < cleanupAttempts; attempt++ {
			if err := os.Remove(backup); err == nil {
				return
			}
			time.Sleep(cleanupInterval)
		}
	}()
}
