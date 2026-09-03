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
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

// Whether the catalogue is read on a schedule. A sync is off by default: it
// rewrites what every recorded request is said to have cost, and that is a
// change somebody should ask for rather than find already made.
const (
	ModelsDevSyncAuto = "auto"
	ModelsDevSyncOff  = "off"
)

// modelsDevSyncDelay lets the process finish starting before it pulls a few
// megabytes, and modelsDevSyncTick is how often the schedule is reconsidered -
// turning the sync on does not wait out a whole interval to take effect.
const (
	modelsDevSyncDelay = 90 * time.Second
	modelsDevSyncTick  = 10 * time.Minute
)

// ModelsDevSyncState is the schedule and the last run, which is what the
// Pricing page reports. Like ProviderHealth it lives in memory: it describes
// what this process has done, not a stored setting.
type ModelsDevSyncState struct {
	Mode          string `json:"mode"`
	IntervalHours int    `json:"intervalHours"`
	Running       bool   `json:"running"`
	SyncedTime    string `json:"syncedTime"`
	// NextTime is when the schedule runs again, empty when nothing is scheduled.
	NextTime string         `json:"nextTime"`
	Error    string         `json:"error"`
	Result   *ModelsDevSync `json:"result"`
}

var (
	modelsDevSyncMutex   sync.Mutex
	modelsDevSyncRunning bool
	modelsDevSynced      time.Time
	modelsDevSyncError   string
	modelsDevSyncResult  *ModelsDevSync
)

func GetModelsDevSyncMode() string {
	if strings.EqualFold(conf.GetConfigStringUnquoted("modelsDevSyncMode"), ModelsDevSyncAuto) {
		return ModelsDevSyncAuto
	}
	return ModelsDevSyncOff
}

func GetModelsDevSyncState() *ModelsDevSyncState {
	modelsDevSyncMutex.Lock()
	defer modelsDevSyncMutex.Unlock()

	state := &ModelsDevSyncState{
		Mode:          GetModelsDevSyncMode(),
		IntervalHours: conf.GetModelsDevSyncIntervalHours(),
		Running:       modelsDevSyncRunning,
		Error:         modelsDevSyncError,
		Result:        modelsDevSyncResult,
	}
	if !modelsDevSynced.IsZero() {
		state.SyncedTime = util.FormatTime(modelsDevSynced)
		if state.Mode == ModelsDevSyncAuto {
			state.NextTime = util.FormatTime(modelsDevSynced.Add(modelsDevSyncInterval()))
		}
	}
	return state
}

// StartModelsDevSync keeps the prices of the models this machine runs in step
// with the catalogue. Nothing happens until the sync is turned on, and the
// schedule is read each time round rather than held, so turning it off stops it.
func StartModelsDevSync() {
	go func() {
		time.Sleep(modelsDevSyncDelay)
		for {
			if isModelsDevSyncDue() {
				if _, err := RunModelsDevSync(true); err != nil {
					beego.Error("models.dev sync failed:", err)
				}
			}
			time.Sleep(modelsDevSyncTick)
		}
	}()
}

func modelsDevSyncInterval() time.Duration {
	return time.Duration(conf.GetModelsDevSyncIntervalHours()) * time.Hour
}

func isModelsDevSyncDue() bool {
	if GetModelsDevSyncMode() != ModelsDevSyncAuto {
		return false
	}

	modelsDevSyncMutex.Lock()
	defer modelsDevSyncMutex.Unlock()
	if modelsDevSyncRunning {
		return false
	}
	// A run that failed is retried on the next tick rather than the next
	// interval: the usual reason is a network that was down for a minute.
	if modelsDevSynced.IsZero() {
		return true
	}
	return time.Since(modelsDevSynced) >= modelsDevSyncInterval()
}

// RunModelsDevSync prices every model this machine has been seen running and
// keeps what it did, so the page reports the last run whether it was asked for
// or scheduled. Two syncs at once would only fetch the same catalogue twice.
func RunModelsDevSync(force bool) (*ModelsDevSync, error) {
	if !beginModelsDevSync() {
		return nil, fmt.Errorf("a models.dev sync is already running")
	}

	result, err := SyncModelsDevPrices(SeenModelNames(), force)
	endModelsDevSync(result, err)
	return result, err
}

func beginModelsDevSync() bool {
	modelsDevSyncMutex.Lock()
	defer modelsDevSyncMutex.Unlock()

	if modelsDevSyncRunning {
		return false
	}
	modelsDevSyncRunning = true
	return true
}

func endModelsDevSync(result *ModelsDevSync, err error) {
	modelsDevSyncMutex.Lock()
	defer modelsDevSyncMutex.Unlock()

	modelsDevSyncRunning = false
	if err != nil {
		// The last successful run stays on the state: what it priced is still
		// in effect, and the error says why there has been nothing since.
		modelsDevSyncError = err.Error()
		return
	}
	modelsDevSyncError = ""
	modelsDevSyncResult = result
	modelsDevSynced = time.Now()
}

// SeenModelNames is every model name this machine has a record of running: the
// relayed requests, the agents' own transcripts, and the models the configured
// providers say they serve. A source that cannot be read contributes nothing
// rather than failing the sync, which the other two can still be useful without.
func SeenModelNames() []string {
	found := map[string]bool{}
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			found[name] = true
		}
	}

	if models, err := GetSeenLlmModels(); err == nil {
		for _, model := range models {
			add(model)
		}
	}

	for _, stat := range GetAgentUsage(HistoricalSessions(""), "").Models {
		add(stat.Name)
	}

	if providers, err := GetProviders(""); err == nil {
		for _, provider := range providers {
			for _, model := range provider.Models {
				add(model)
			}
		}
	}

	models := make([]string, 0, len(found))
	for model := range found {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}
