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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/cloudsync"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
	"github.com/xorm-io/core"
)

// Whether the backups are copied somewhere that is not this machine. It is off
// until somebody says where to: a snapshot carries every API key on the host,
// and nothing leaves the machine on a guess.
const (
	CloudSyncAuto = "auto"
	CloudSyncOff  = "off"
)

// cloudSyncTimeout bounds one run. A snapshot is a few kilobytes, so anything
// past this is a target that is not answering rather than a slow upload.
const cloudSyncTimeout = 5 * time.Minute

// cloudSyncMaxBytes is the largest file one run copies. Backups are kilobytes;
// this is only here so that a folder somebody else fills cannot be pulled into
// memory whole.
const cloudSyncMaxBytes = 64 << 20

// cloudSyncWait is how long a run waits for one already in flight. A push of a
// few kilobytes is over in a moment, so waiting is how pressing the button just
// after a backup stops looking like a collision.
const (
	cloudSyncWait = 15 * time.Second
	cloudSyncPoll = 200 * time.Millisecond
)

// errCloudSyncBusy is a run that waited and still found the target busy.
var errCloudSyncBusy = errors.New("a cloud sync is already running")

var (
	cloudSyncMutex   sync.Mutex
	cloudSyncRunning bool
	cloudSyncPending bool
	cloudSyncTime    time.Time
	cloudSyncError   string
	cloudSyncResult  *cloudsync.Result
)

// CloudSyncState is where the copies go and how the last run went, which is
// what the Settings page reports. Like BackupState it lives in memory: it
// describes what this process has done, not a stored setting.
type CloudSyncState struct {
	Mode string `json:"mode"`
	Kind string `json:"kind"`
	// Options are the stored ones with the credentials decrypted, which is what
	// the admin typed and what the form has to show again to be edited.
	Options map[string]string `json:"options"`
	// Kinds is every storage Gateway can sync to, described well enough for the
	// page to draw the form for one it has never heard of.
	Kinds []*cloudsync.Kind `json:"kinds"`
	// Folders are the synced folders found on this machine, so that pointing
	// Gateway at Dropbox or a NAS share is a click rather than a path to find.
	Folders []cloudsync.Folder `json:"folders"`
	// Target is where the copies land, in one line and without credentials.
	Target     string            `json:"target"`
	Running    bool              `json:"running"`
	SyncedTime string            `json:"syncedTime"`
	Error      string            `json:"error"`
	Result     *cloudsync.Result `json:"result"`
}

func GetCloudSyncMode() string {
	if strings.EqualFold(conf.GetConfigStringUnquoted("cloudSyncMode"), CloudSyncAuto) {
		return CloudSyncAuto
	}
	return CloudSyncOff
}

func GetCloudSyncKind() string {
	return conf.GetConfigStringUnquoted("cloudSyncKind")
}

// storedCloudSyncOptions is the options column as it sits in the database,
// credentials still encrypted.
func storedCloudSyncOptions() map[string]string {
	options := map[string]string{}

	raw := conf.GetConfigString("cloudSyncOptions")
	if strings.TrimSpace(raw) == "" {
		return options
	}
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		beego.Error("the cloud sync options could not be read:", err)
		return map[string]string{}
	}
	return options
}

// cloudSyncAad binds a credential's ciphertext to the option it was typed
// into, so a WebDAV password moved into the S3 key no longer decrypts.
func cloudSyncAad(kind string, name string) string {
	return fmt.Sprintf("%s/cloudSync/%s/%s", BuiltInSettingId, kind, name)
}

func encryptCloudSyncOptions(kind string, options map[string]string) (map[string]string, error) {
	res := map[string]string{}
	for name, value := range options {
		res[name] = value
	}

	for _, name := range cloudsync.SecretOptions(kind) {
		encrypted, err := util.EncryptWithKey(apiKeyEncryptionSecret(), res[name], cloudSyncAad(kind, name))
		if err != nil {
			return nil, err
		}
		res[name] = encrypted
	}
	return res, nil
}

// decryptCloudSyncOptions is the other half. A credential that will not
// decrypt is left as it stands rather than dropping the whole configuration:
// the target then fails to authenticate, which is the truth of the matter.
func decryptCloudSyncOptions(kind string, options map[string]string) map[string]string {
	res := map[string]string{}
	for name, value := range options {
		res[name] = value
	}

	for _, name := range cloudsync.SecretOptions(kind) {
		plain, err := util.DecryptWithKey(apiKeyEncryptionSecret(), res[name], cloudSyncAad(kind, name))
		if err != nil {
			beego.Error("a cloud sync credential could not be decrypted:", name, err)
			continue
		}
		res[name] = plain
	}
	return res
}

// CloudSyncConfig is the stored configuration, ready to be handed to
// cloudsync. Outbound traffic goes through Gateway's own client, so a target
// reached over the configured proxy needs nothing said here.
func CloudSyncConfig() cloudsync.Config {
	kind := GetCloudSyncKind()
	return cloudsync.Config{
		Kind:       kind,
		Options:    decryptCloudSyncOptions(kind, storedCloudSyncOptions()),
		HttpClient: proxy.ProxyHttpClient,
	}
}

// SaveCloudSyncConfig stores where the copies go. The configuration is built
// once before it is written, so a target that cannot be made is reported here
// rather than by a schedule nobody is watching.
func SaveCloudSyncConfig(mode string, kind string, options map[string]string) error {
	if mode != CloudSyncAuto && mode != CloudSyncOff {
		return fmt.Errorf("cloudSyncMode must be \"auto\" or \"off\"")
	}

	if kind == "" {
		mode, options = CloudSyncOff, map[string]string{}
	} else if _, err := cloudsync.New(cloudsync.Config{Kind: kind, Options: options}); err != nil {
		return err
	}

	encrypted, err := encryptCloudSyncOptions(kind, options)
	if err != nil {
		return err
	}

	stored, err := json.Marshal(encrypted)
	if err != nil {
		return err
	}

	setting, err := GetBuiltInSetting()
	if err != nil {
		return err
	}
	if setting == nil {
		return fmt.Errorf("the built-in setting does not exist")
	}

	setting.CloudSyncMode, setting.CloudSyncKind, setting.CloudSyncOptions = mode, kind, string(stored)
	if _, err = UpdateSetting(BuiltInSettingId, setting); err != nil {
		return err
	}
	return nil
}

// GetCloudSyncState answers the Settings page.
func GetCloudSyncState() *CloudSyncState {
	cloudSyncMutex.Lock()
	defer cloudSyncMutex.Unlock()

	config := CloudSyncConfig()
	state := &CloudSyncState{
		Mode:    GetCloudSyncMode(),
		Kind:    config.Kind,
		Options: config.Options,
		Kinds:   cloudsync.Kinds(),
		Folders: cloudsync.DetectFolders(),
		Running: cloudSyncRunning,
		Error:   cloudSyncError,
		Result:  cloudSyncResult,
	}

	if config.Kind != "" {
		if target, err := cloudsync.New(config); err == nil {
			state.Target = target.Describe()
		}
	}
	if !cloudSyncTime.IsZero() {
		state.SyncedTime = util.FormatTime(cloudSyncTime)
	}
	return state
}

// TestCloudSync is what the "Test" button asks, of a configuration that is
// being typed rather than of the stored one.
func TestCloudSync(kind string, options map[string]string) (string, []cloudsync.File, error) {
	config := cloudsync.Config{Kind: kind, Options: options, HttpClient: proxy.ProxyHttpClient}

	ctx, cancel := context.WithTimeout(context.Background(), cloudSyncTimeout)
	defer cancel()

	target, files, err := cloudsync.Check(ctx, config)
	if err != nil {
		if target == nil {
			return "", nil, err
		}
		return target.Describe(), nil, err
	}
	return target.Describe(), files, nil
}

// RunCloudSync copies the backup directory and the target onto each other. The
// files are immutable snapshots named after the moment they were taken, so
// there is nothing to merge: each side ends up holding what the other had.
func RunCloudSync(direction string) (*cloudsync.Result, error) {
	config := CloudSyncConfig()
	if config.Kind == "" {
		return nil, fmt.Errorf("no cloud sync target is configured")
	}

	target, err := cloudsync.New(config)
	if err != nil {
		return nil, err
	}

	if !acquireCloudSync() {
		return nil, errCloudSyncBusy
	}

	ctx, cancel := context.WithTimeout(context.Background(), cloudSyncTimeout)
	defer cancel()

	result, err := cloudsync.SyncDir(ctx, target, backupDir(), cloudsync.Options{
		Match:     backupNamePattern.MatchString,
		Retention: conf.GetBackupRetention(),
		Direction: direction,
		MaxBytes:  cloudSyncMaxBytes,
	})

	cloudSyncMutex.Lock()
	cloudSyncRunning = false
	cloudSyncTime = time.Now()
	cloudSyncResult = result
	cloudSyncError = ""
	if err != nil {
		cloudSyncError = err.Error()
	} else if len(result.Errors) > 0 {
		cloudSyncError = strings.Join(result.Errors, "; ")
	}
	// A snapshot taken while this run was going may have missed it, so the run
	// that was turned away happens now rather than at the next interval.
	pending := cloudSyncPending
	cloudSyncPending = false
	cloudSyncMutex.Unlock()

	if pending {
		go func() {
			if _, err := RunCloudSync(cloudsync.DirectionUp); err != nil && !errors.Is(err, errCloudSyncBusy) {
				beego.Error("the cloud sync that was waiting failed:", err)
			}
		}()
	}

	return result, err
}

// acquireCloudSync makes this the only run going. One that gives up leaves the
// work behind it: whichever run is in flight picks it up as it finishes.
func acquireCloudSync() bool {
	deadline := time.Now().Add(cloudSyncWait)
	for {
		cloudSyncMutex.Lock()
		if !cloudSyncRunning {
			cloudSyncRunning, cloudSyncPending = true, false
			cloudSyncMutex.Unlock()
			return true
		}
		cloudSyncMutex.Unlock()

		if time.Now().After(deadline) {
			cloudSyncMutex.Lock()
			cloudSyncPending = true
			cloudSyncMutex.Unlock()
			return false
		}
		time.Sleep(cloudSyncPoll)
	}
}

// cloudSyncTick is the scheduled half, called from the same loop that takes the
// backups. The interval is the backup one: a copy of a snapshot is due exactly
// as often as the snapshot is.
func cloudSyncTick() {
	if GetCloudSyncMode() != CloudSyncAuto || GetCloudSyncKind() == "" {
		return
	}

	cloudSyncMutex.Lock()
	due := !cloudSyncRunning && (cloudSyncTime.IsZero() || time.Since(cloudSyncTime) >= backupInterval())
	cloudSyncMutex.Unlock()

	if !due {
		return
	}
	// The first run of a process pulls as well as pushes, which is how a second
	// machine, or a reinstalled one, finds the backups it never took.
	if _, err := RunCloudSync(cloudsync.DirectionBoth); err != nil {
		beego.Error("the cloud sync failed:", err)
	}
}

// cloudSyncAfterBackup pushes the snapshot that was just taken. It runs in the
// background: a backup is not held up by a NAS that is asleep.
func cloudSyncAfterBackup() {
	if GetCloudSyncMode() != CloudSyncAuto || GetCloudSyncKind() == "" {
		return
	}

	go func() {
		// A run already going is not a failure: it either carries this snapshot
		// along or leaves the push queued behind it.
		if _, err := RunCloudSync(cloudsync.DirectionUp); err != nil && !errors.Is(err, errCloudSyncBusy) {
			beego.Error("the backup could not be copied to the cloud sync target:", err)
		}
	}()
}

// initCloudSyncSetting fills the column in for an installation that predates
// it, the way every other setting added later is filled in.
func initCloudSyncSetting(setting *Setting) {
	if setting.CloudSyncMode != "" {
		return
	}

	setting.CloudSyncMode = CloudSyncOff
	if _, err := ormer.Engine.ID(core.PK{setting.Owner, setting.Name}).Cols("cloud_sync_mode").Update(setting); err != nil {
		panic(err)
	}
}
