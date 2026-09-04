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

package controllers

import (
	"encoding/json"
	"os"

	"github.com/apache/casbin-gateway/cloudsync"
	"github.com/apache/casbin-gateway/object"
)

// ExportSnapshot answers with the configuration as one document, which the page
// saves as a file. Everything here is admin-only: a snapshot carries the API
// keys of every provider on the machine.
func (c *ApiController) ExportSnapshot() {
	if c.RequireAdmin() {
		return
	}

	var scope object.SnapshotScope
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &scope); err != nil {
		c.ResponseError(err.Error())
		return
	}

	snapshot, err := object.BuildSnapshot(scope, object.BackupManual)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(snapshot)
}

// ImportSnapshot writes a snapshot back. A dry run decides everything the real
// one would and writes nothing, so the page can show what an import will do
// before it does it.
func (c *ApiController) ImportSnapshot() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Snapshot *object.Snapshot     `json:"snapshot"`
		Scope    object.SnapshotScope `json:"scope"`
		Mode     string               `json:"mode"`
		DryRun   bool                 `json:"dryRun"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if request.Snapshot == nil {
		c.ResponseError("this file is not a Gateway snapshot")
		return
	}

	// Taken before anything is written, so an import that turns out to be the
	// wrong one is a restore away rather than a loss.
	backup := ""
	if !request.DryRun {
		backup = object.BackupBeforeWrite()
	}

	report, err := object.ImportSnapshot(request.Snapshot, request.Scope, request.Mode, request.DryRun)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	report.Backup = backup

	c.ResponseOk(report)
}

// GetBackupState is the schedule, the last run and the files themselves.
func (c *ApiController) GetBackupState() {
	if c.RequireAdmin() {
		return
	}

	state, err := object.GetBackupState()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(state)
}

func (c *ApiController) CreateBackup() {
	if c.RequireAdmin() {
		return
	}

	if _, err := object.CreateBackup(object.BackupManual); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.GetBackupState()
}

// GetBackup returns one stored snapshot in full, which is what downloading a
// backup from the list needs.
func (c *ApiController) GetBackup() {
	if c.RequireAdmin() {
		return
	}

	snapshot, err := object.ReadBackup(c.Input().Get("name"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(snapshot)
}

// RestoreBackup puts one snapshot back exactly as it was taken. Like an import
// it can be asked what it would do first.
func (c *ApiController) RestoreBackup() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Name   string `json:"name"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	backup := ""
	if !request.DryRun {
		backup = object.BackupBeforeWrite()
	}

	report, err := object.RestoreBackup(request.Name, request.DryRun)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	report.Backup = backup

	c.ResponseOk(report)
}

func (c *ApiController) DeleteBackup() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := object.DeleteBackup(request.Name); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.GetBackupState()
}

// UpdateBackupSchedule stores the schedule, and the directory the snapshots go
// in, on the built-in setting row - the same place the Settings page saves
// everything else. The directory is the other way to put the data in Dropbox,
// OneDrive, iCloud Drive or a NAS share: point it at the folder their client
// already syncs and every snapshot is uploaded as it is written.
func (c *ApiController) UpdateBackupSchedule() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Mode          string `json:"mode"`
		IntervalHours int    `json:"intervalHours"`
		Retention     int    `json:"retention"`
		Dir           string `json:"dir"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if request.Mode != object.BackupAuto && request.Mode != object.BackupOff {
		c.ResponseError("backupMode must be \"auto\" or \"off\"")
		return
	}
	if request.IntervalHours <= 0 {
		c.ResponseError("the backup interval must be at least one hour")
		return
	}
	if request.Retention <= 0 {
		c.ResponseError("at least one backup has to be kept")
		return
	}

	// An empty directory is the default one beside the database. Anything else
	// is created here: a folder that cannot be made is a setting that would
	// otherwise fail every backup from now on, silently.
	dir := cloudsync.ExpandPath(request.Dir)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			c.ResponseError(err.Error())
			return
		}
	}

	setting, err := object.GetBuiltInSetting()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if setting == nil {
		c.ResponseError("the built-in setting does not exist")
		return
	}

	setting.BackupMode = request.Mode
	setting.BackupIntervalHours = request.IntervalHours
	setting.BackupRetention = request.Retention
	setting.BackupDir = dir
	if _, err := object.UpdateSetting(object.BuiltInSettingId, setting); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.GetBackupState()
}
