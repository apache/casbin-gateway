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

package imbridge

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

// forgetSession removes a stored session, so a conversation ended from a chat
// does not come back after a restart. Storage lives outside this package, so it
// is handed in the same way the driver's own is.
var forgetSession = func(string) {}

// SetSessionForgetter installs how a closed conversation is removed from store.
func SetSessionForgetter(forget func(string)) {
	forgetSession = forget
}

// Status is what one channel is doing, for the page that lists them.
type Status struct {
	Name string `json:"name"`
	// Running says the poll loop is up. It is not a promise that the credential
	// works: that only shows once the platform answers, in Error.
	Running   bool   `json:"running"`
	Error     string `json:"error,omitempty"`
	StartTime string `json:"startTime,omitempty"`
}

type listener struct {
	channel Channel
	cancel  context.CancelFunc
	status  Status
}

var manager = struct {
	sync.Mutex
	listeners map[string]*listener
}{listeners: map[string]*listener{}}

// Reload brings the running channels in line with the stored ones: what is gone
// is stopped, what changed is restarted, and the rest is left alone so an open
// poll is not dropped for nothing.
func Reload(channels []Channel) {
	manager.Lock()
	defer manager.Unlock()

	wanted := map[string]Channel{}
	for _, channel := range channels {
		wanted[channel.Name] = channel
	}

	for name, running := range manager.listeners {
		channel, keep := wanted[name]
		if keep && sameChannel(channel, running.channel) {
			continue
		}
		running.cancel()
		delete(manager.listeners, name)
	}

	for name, channel := range wanted {
		if _, running := manager.listeners[name]; running {
			continue
		}
		start(name, channel)
	}
}

// Stop ends every channel, for a shutdown.
func Stop() {
	manager.Lock()
	defer manager.Unlock()

	for name, running := range manager.listeners {
		running.cancel()
		delete(manager.listeners, name)
	}
}

// Statuses is what every channel is doing.
func Statuses() []Status {
	manager.Lock()
	defer manager.Unlock()

	result := []Status{}
	for _, running := range manager.listeners {
		result = append(result, running.status)
	}
	return result
}

// start runs one channel. The caller holds the lock.
func start(name string, channel Channel) {
	platform, err := platformOf(channel)
	if err != nil {
		manager.listeners[name] = &listener{
			channel: channel,
			cancel:  func() {},
			status:  Status{Name: name, Error: err.Error()},
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &listener{
		channel: channel,
		cancel:  cancel,
		status:  Status{Name: name, Running: true, StartTime: time.Now().Format(time.RFC3339)},
	}
	manager.listeners[name] = entry

	router := newRouter(channel, platform)
	go func() {
		err := platform.Receive(ctx, router.handle)
		manager.Lock()
		defer manager.Unlock()
		// A channel restarted while this one was winding down has already taken
		// the slot, and its status is not this one's to overwrite.
		if manager.listeners[name] != entry {
			return
		}
		entry.status.Running = false
		if err != nil && ctx.Err() == nil {
			entry.status.Error = err.Error()
		}
	}()
}

// sameChannel reports whether nothing a listener depends on has changed. A
// Channel carries a slice, so it cannot simply be compared.
func sameChannel(left, right Channel) bool {
	return left.Platform == right.Platform && left.Token == right.Token &&
		left.AgentId == right.AgentId && left.AgentPath == right.AgentPath &&
		left.AgentUser == right.AgentUser && left.WorkDir == right.WorkDir &&
		left.Model == right.Model && slices.Equal(left.AllowedUsers, right.AllowedUsers)
}

func platformOf(channel Channel) (Platform, error) {
	if channel.Token == "" {
		return nil, fmt.Errorf("the %s channel has no credential yet", channel.Name)
	}

	switch channel.Platform {
	case PlatformTelegram:
		return NewTelegram(channel.Name, channel.Token), nil
	case PlatformWeixin:
		return NewWeixin(channel.Name, channel.Token), nil
	}
	return nil, fmt.Errorf("no chat platform named %q", channel.Platform)
}
