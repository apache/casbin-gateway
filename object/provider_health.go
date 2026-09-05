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
	"sort"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/util"
)

const (
	// providerFailureThreshold is how many failures in a row take a provider out
	// of rotation. One failure is not enough: an upstream that answers 500 once
	// is still the provider the operator picked.
	providerFailureThreshold = 2
	providerCooldown         = 15 * time.Second
	providerMaxCooldown      = 5 * time.Minute
)

// ProviderHealth is what the proxy has seen of one provider. It lives in memory
// only: it describes this process's view of an upstream, not a stored setting.
type ProviderHealth struct {
	Provider    string `json:"provider"`
	Healthy     bool   `json:"healthy"`
	Successes   int64  `json:"successes"`
	Failures    int64  `json:"failures"`
	Consecutive int    `json:"consecutive"`
	LastError   string `json:"lastError"`
	LastFailure string `json:"lastFailure"`
	RetryTime   string `json:"retryTime"`
}

type providerHealth struct {
	successes   int64
	failures    int64
	consecutive int
	lastError   string
	lastFailure time.Time
	retryTime   time.Time
}

var (
	providerHealthMutex sync.Mutex
	providerHealthMap   = map[string]*providerHealth{}
)

// ReportProviderSuccess closes the breaker: an upstream that answered is back in
// rotation whatever it did before.
func ReportProviderSuccess(providerId string) {
	providerHealthMutex.Lock()
	defer providerHealthMutex.Unlock()

	health := healthOf(providerId)
	health.successes++
	health.consecutive = 0
	health.retryTime = time.Time{}
}

// ReportProviderFailure records an attempt that could not be relayed. Once a
// provider has failed providerFailureThreshold times in a row it is suspended for
// a window that doubles with every further failure, so a dead upstream stops
// costing every request the time it takes to time out.
func ReportProviderFailure(providerId string, reason string) {
	providerHealthMutex.Lock()
	defer providerHealthMutex.Unlock()

	health := healthOf(providerId)
	health.failures++
	health.consecutive++
	health.lastError = reason
	health.lastFailure = time.Now()

	if health.consecutive < providerFailureThreshold {
		return
	}
	cooldown := providerCooldown << (health.consecutive - providerFailureThreshold)
	if cooldown > providerMaxCooldown || cooldown <= 0 {
		cooldown = providerMaxCooldown
	}
	health.retryTime = time.Now().Add(cooldown)
}

// IsProviderSuspended reports whether a provider is inside its cooldown window.
// A suspended provider is tried last rather than dropped: it may be the only one
// the agent has.
func IsProviderSuspended(providerId string) bool {
	providerHealthMutex.Lock()
	defer providerHealthMutex.Unlock()

	health, ok := providerHealthMap[providerId]
	return ok && time.Now().Before(health.retryTime)
}

// ClearProviderHealth forgets what the proxy saw of a provider, which is what an
// edited provider deserves: its base URL or key may be the thing that was fixed.
func ClearProviderHealth(providerId string) {
	providerHealthMutex.Lock()
	defer providerHealthMutex.Unlock()
	delete(providerHealthMap, providerId)
}

// GetProviderHealth lists what is known about every provider used since startup.
func GetProviderHealth() []*ProviderHealth {
	providerHealthMutex.Lock()
	defer providerHealthMutex.Unlock()

	now := time.Now()
	result := make([]*ProviderHealth, 0, len(providerHealthMap))
	for id, health := range providerHealthMap {
		item := &ProviderHealth{
			Provider:    id,
			Healthy:     !now.Before(health.retryTime),
			Successes:   health.successes,
			Failures:    health.failures,
			Consecutive: health.consecutive,
			LastError:   health.lastError,
		}
		if !health.lastFailure.IsZero() {
			item.LastFailure = util.FormatTime(health.lastFailure)
		}
		if now.Before(health.retryTime) {
			item.RetryTime = util.FormatTime(health.retryTime)
		}
		result = append(result, item)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Provider < result[j].Provider })
	return result
}

func healthOf(providerId string) *providerHealth {
	health, ok := providerHealthMap[providerId]
	if !ok {
		health = &providerHealth{}
		providerHealthMap[providerId] = health
	}
	return health
}
