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

package object

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/cloudsync"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

// Whether a backup is taken on a schedule. It is on by default: a snapshot of
// the configuration is a few kilobytes, and the copy nobody took is the one
// that is missed.
const (
	BackupAuto = "auto"
	BackupOff  = "off"
)

// What took a backup, which is also the second half of its file name.
const (
	BackupManual   = "manual"
	BackupSchedule = "schedule"
	// BackupBeforeImport is taken in front of anything that writes a snapshot
	// back, so an import that turns out to be the wrong one is one restore away.
	BackupBeforeImport = "before-import"
)

// backupDelay lets the process finish starting before it writes anything, and
// backupTick is how often the schedule is reconsidered - turning it on does not
// wait out a whole interval to take effect.
const (
	backupDelay = 120 * time.Second
	backupTick  = 10 * time.Minute
)

// backupNamePattern is what a file in the backup directory has to look like to
// be one of ours. Names arrive from the browser, so nothing else is opened.
var backupNamePattern = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}(-[a-z0-9-]+)?\.json$`)

var (
	backupMutex  sync.Mutex
	backupTaken  time.Time
	backupError  string
	backupLatest string
)

// Backup is one snapshot on disk, described without reading the whole of it
// back to the browser.
type Backup struct {
	Name        string `json:"name"`
	CreatedTime string `json:"createdTime"`
	Reason      string `json:"reason"`
	Gateway     string `json:"gateway"`
	Host        string `json:"host"`
	Size        int64  `json:"size"`
	// Secrets is whether the API keys are in it, which decides whether restoring
	// it leaves a working Gateway or one with the keys to type back in.
	Secrets bool           `json:"secrets"`
	Counts  SnapshotCounts `json:"counts"`
}

// BackupState is the schedule and the last run, which is what the Settings page
// reports. Like ProviderHealth it lives in memory: it describes what this
// process has done, not a stored setting.
type BackupState struct {
	Mode          string `json:"mode"`
	IntervalHours int    `json:"intervalHours"`
	Retention     int    `json:"retention"`
	Dir           string `json:"dir"`
	// Folders are the synced folders this machine has, so that putting the
	// backups straight into Dropbox, OneDrive, iCloud Drive or a NAS share is a
	// choice on the page rather than a path to go and find.
	Folders []cloudsync.Folder `json:"folders"`
	// NextTime is when the schedule runs again, empty when nothing is scheduled.
	TakenTime string    `json:"takenTime"`
	NextTime  string    `json:"nextTime"`
	Latest    string    `json:"latest"`
	Error     string    `json:"error"`
	Backups   []*Backup `json:"backups"`
}

func GetBackupMode() string {
	if strings.EqualFold(conf.GetConfigStringUnquoted("backupMode"), BackupOff) {
		return BackupOff
	}
	return BackupAuto
}

// backupDir is where the snapshots are written. The path is expanded here, so
// that pointing it at "~/Dropbox/Casbin Gateway" or "%OneDrive%\Backups" works
// wherever it was typed.
func backupDir() string {
	return cloudsync.ExpandPath(conf.GetBackupDir())
}

// backupPath resolves one name inside the backup directory. A name that is not
// one of ours never becomes a path, so nothing outside that directory is read,
// written or deleted.
func backupPath(name string) (string, error) {
	if !backupNamePattern.MatchString(name) {
		return "", fmt.Errorf("this is not the name of a backup: %s", name)
	}
	return filepath.Join(backupDir(), name), nil
}

func backupFileName(at time.Time, reason string) string {
	if reason == "" {
		reason = BackupManual
	}
	return fmt.Sprintf("%s-%s.json", at.Format("20060102-150405"), reason)
}

// CreateBackup writes the whole configuration, secrets included, to a file
// beside the database, then drops the oldest files past the retention.
func CreateBackup(reason string) (*Backup, error) {
	snapshot, err := BuildSnapshot(FullScope(), reason)
	if err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}

	dir := backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	at := time.Now()
	name := backupFileName(at, reason)
	path := filepath.Join(dir, name)
	// A second backup within the same second would otherwise overwrite the
	// first, which is the one case where two snapshots differ by an import.
	for suffix := 2; util.FileExist(path) && suffix < 100; suffix++ {
		name = fmt.Sprintf("%s-%d.json", strings.TrimSuffix(backupFileName(at, reason), ".json"), suffix)
		path = filepath.Join(dir, name)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}

	backupMutex.Lock()
	backupTaken = time.Now()
	backupError = ""
	backupLatest = name
	backupMutex.Unlock()

	if err := pruneBackups(); err != nil {
		beego.Error("the old backups could not be pruned:", err)
	}

	// A copy on this disk is not a backup of this disk, so the snapshot goes to
	// wherever the cloud sync points as soon as it has been written.
	cloudSyncAfterBackup()

	return describeBackup(name, int64(len(data)), snapshot), nil
}

func describeBackup(name string, size int64, snapshot *Snapshot) *Backup {
	return &Backup{
		Name:        name,
		CreatedTime: snapshot.CreatedTime,
		Reason:      snapshot.Reason,
		Gateway:     snapshot.Gateway,
		Host:        snapshot.Host,
		Size:        size,
		Secrets:     snapshot.Scope.Secrets,
		Counts:      snapshot.Counts(),
	}
}

// ListBackups describes every snapshot in the directory, newest first. A file
// that will not parse is left out rather than failing the listing: the
// directory is on disk, and anything can put a file there.
func ListBackups() ([]*Backup, error) {
	entries, err := os.ReadDir(backupDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []*Backup{}, nil
		}
		return nil, err
	}

	backups := []*Backup{}
	for _, entry := range entries {
		if entry.IsDir() || !backupNamePattern.MatchString(entry.Name()) {
			continue
		}

		snapshot, size, err := readBackupFile(entry.Name())
		if err != nil {
			beego.Error("a backup could not be read:", entry.Name(), err)
			continue
		}
		backups = append(backups, describeBackup(entry.Name(), size, snapshot))
	}

	// The name starts with the timestamp, so it sorts the same way the contents
	// would and needs nothing parsed to do it.
	sort.Slice(backups, func(i, j int) bool { return backups[i].Name > backups[j].Name })
	return backups, nil
}

func readBackupFile(name string) (*Snapshot, int64, error) {
	path, err := backupPath(name)
	if err != nil {
		return nil, 0, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	snapshot := &Snapshot{}
	if err := json.Unmarshal(data, snapshot); err != nil {
		return nil, 0, err
	}
	return snapshot, int64(len(data)), nil
}

// ReadBackup returns one snapshot in full, which is what downloading a backup
// and restoring one both need.
func ReadBackup(name string) (*Snapshot, error) {
	snapshot, _, err := readBackupFile(name)
	return snapshot, err
}

func DeleteBackup(name string) error {
	path, err := backupPath(name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

// RestoreBackup puts one snapshot back exactly as it was taken, so a section it
// covers ends up with the rows it held and nothing else.
func RestoreBackup(name string, dryRun bool) (*ImportReport, error) {
	snapshot, err := ReadBackup(name)
	if err != nil {
		return nil, err
	}
	return ImportSnapshot(snapshot, snapshot.Scope, ImportReplace, dryRun)
}

// pruneBackups keeps the newest few. A backup taken in front of an import is
// counted with the rest: it is only worth keeping until the import it guards
// has been lived with.
func pruneBackups() error {
	backups, err := ListBackups()
	if err != nil {
		return err
	}

	retention := conf.GetBackupRetention()
	if len(backups) <= retention {
		return nil
	}

	for _, backup := range backups[retention:] {
		if err := DeleteBackup(backup.Name); err != nil {
			return err
		}
	}
	return nil
}

// GetBackupState answers the Settings page: the schedule, the last run, and the
// files themselves.
func GetBackupState() (*BackupState, error) {
	backups, err := ListBackups()
	if err != nil {
		return nil, err
	}

	backupMutex.Lock()
	defer backupMutex.Unlock()

	state := &BackupState{
		Mode:          GetBackupMode(),
		IntervalHours: conf.GetBackupIntervalHours(),
		Retention:     conf.GetBackupRetention(),
		Dir:           backupDir(),
		Folders:       cloudsync.DetectFolders(),
		Latest:        backupLatest,
		Error:         backupError,
		Backups:       backups,
	}

	// The files outlive the process, so the last backup is the newest of them
	// rather than only the one this process took.
	taken := backupTaken
	if len(backups) > 0 {
		state.Latest = backups[0].Name
		// The name was written from the wall clock, so it is read back in the
		// same zone rather than as UTC.
		if from, err := time.ParseInLocation("20060102-150405", backups[0].Name[:15], time.Local); err == nil && from.After(taken) {
			taken = from
		}
	}
	if !taken.IsZero() {
		state.TakenTime = util.FormatTime(taken)
		if state.Mode == BackupAuto {
			state.NextTime = util.FormatTime(taken.Add(backupInterval()))
		}
	}
	return state, nil
}

// StartBackupSchedule takes a snapshot of the configuration every so often, and
// copies the snapshots to the cloud sync target on the same round. The schedule
// is read each time round rather than held, so turning it off stops it.
func StartBackupSchedule() {
	go func() {
		time.Sleep(backupDelay)
		for {
			if isBackupDue() {
				if _, err := CreateBackup(BackupSchedule); err != nil {
					beego.Error("the scheduled backup failed:", err)
					backupMutex.Lock()
					backupError = err.Error()
					backupMutex.Unlock()
				}
			}
			// Copying the backups off the machine is on the same schedule as
			// taking them, and runs whether or not one was due: the pull is how
			// this machine learns what another one backed up.
			cloudSyncTick()
			time.Sleep(backupTick)
		}
	}()
}

func backupInterval() time.Duration {
	return time.Duration(conf.GetBackupIntervalHours()) * time.Hour
}

func isBackupDue() bool {
	if GetBackupMode() != BackupAuto {
		return false
	}

	state, err := GetBackupState()
	if err != nil {
		beego.Error("the backups could not be listed:", err)
		return false
	}
	if state.TakenTime == "" {
		return true
	}

	taken, err := time.Parse(time.RFC3339, state.TakenTime)
	if err != nil {
		return true
	}
	return time.Since(taken) >= backupInterval()
}

// BackupBeforeWrite is the copy taken in front of an import or a restore. It
// never stops one: a Gateway whose backup directory cannot be written is still
// a Gateway somebody is allowed to import into.
func BackupBeforeWrite() string {
	backup, err := CreateBackup(BackupBeforeImport)
	if err != nil {
		beego.Error("the backup before an import could not be taken:", err)
		return ""
	}
	return backup.Name
}
