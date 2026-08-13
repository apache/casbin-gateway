// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	scanCacheTTL = 30 * time.Second
	scanTimeout  = 10 * time.Second
)

type scanCall struct {
	done   chan struct{}
	result []Installation
	err    error
}

var installationCache = struct {
	sync.Mutex
	result    []Installation
	updatedAt time.Time
	current   *scanCall
}{}

// Scan returns installations visible to Gateway and caches successful results.
func Scan(forceRefresh bool) ([]Installation, error) {
	installationCache.Lock()
	if !forceRefresh && time.Since(installationCache.updatedAt) < scanCacheTTL {
		result := cloneInstallations(installationCache.result)
		installationCache.Unlock()
		return result, nil
	}
	if installationCache.current != nil {
		current := installationCache.current
		installationCache.Unlock()
		<-current.done
		return cloneInstallations(current.result), current.err
	}

	current := &scanCall{done: make(chan struct{})}
	installationCache.current = current
	installationCache.Unlock()

	current.result, current.err = scanWithTimeout()

	installationCache.Lock()
	if current.err == nil {
		installationCache.result = cloneInstallations(current.result)
		installationCache.updatedAt = time.Now()
	}
	installationCache.current = nil
	close(current.done)
	installationCache.Unlock()

	return cloneInstallations(current.result), current.err
}

func scanWithTimeout() ([]Installation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), scanTimeout)
	defer cancel()

	result := make(chan []Installation, 1)
	go func() {
		result <- scan(ctx)
	}()

	select {
	case installations := <-result:
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("agent scan timed out after %s: %w", scanTimeout, err)
		}
		return installations, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("agent scan timed out after %s: %w", scanTimeout, ctx.Err())
	}
}

func cloneInstallations(installations []Installation) []Installation {
	if installations == nil {
		return nil
	}
	result := make([]Installation, len(installations))
	copy(result, installations)
	return result
}
