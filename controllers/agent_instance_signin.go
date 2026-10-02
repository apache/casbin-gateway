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
	"bytes"
	"os"
	"path/filepath"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentlink"
	"github.com/apache/casbin-gateway/object"
	"github.com/beego/beego"
)

// signInPollInterval is well inside the seconds a browser sign-in takes to
// come back as a link.
const signInPollInterval = time.Second

// maxSignInLogRead caps what one poll reads of a log that grew a lot at once.
const maxSignInLogRead = 1 << 20

// WatchAgentSignIns hands an agent's next link to whichever copy just sent a
// sign-in to the browser. A missing account at start is not enough to go by:
// a copy whose sign-in went stale still records the old account, and the link
// it signs in again with would open the first copy instead.
func WatchAgentSignIns() {
	if !agentlink.Supported() {
		return
	}

	go func() {
		offsets := map[string]int64{}
		for {
			time.Sleep(signInPollInterval)
			pollAgentSignIns(offsets)
		}
	}()
}

func pollAgentSignIns(offsets map[string]int64) {
	// The first copy signing in gives the scheme back: the agent's own command
	// already opens that copy, and a capture left from an abandoned sign-in in
	// another one would take its link.
	for _, installation := range agent.Cached() {
		scheme := agent.LinkSchemeOf(installation.AgentId)
		_, marker := agent.SignInLogOf(installation.AgentId)
		if scheme == "" || marker == "" {
			continue
		}
		for _, path := range agent.DefaultSignInLogsOf(installation.AgentId, installation.Owner) {
			if !logGained(path, []byte(marker), offsets) {
				continue
			}
			if err := agentlink.Release(scheme); err != nil {
				beego.Error("the sign-in link of the first", installation.AgentId, "cannot be routed to it:", err)
			}
		}
	}

	instances, err := object.GetAgentInstances("")
	if err != nil {
		return
	}

	for _, instance := range instances {
		logName, marker := agent.SignInLogOf(instance.AgentId)
		if logName == "" || marker == "" || agent.LinkSchemeOf(instance.AgentId) == "" {
			continue
		}
		path := filepath.Join(instance.DataDir, filepath.FromSlash(logName))
		if !logGained(path, []byte(marker), offsets) {
			continue
		}

		if err := captureSignIn(instance); err != nil {
			beego.Error("the sign-in link of", instance.Name, "cannot be routed to it:", err)
		}
	}
}

func captureSignIn(instance *object.AgentInstance) error {
	installation, err := findInstallation(instance.AgentId, instance.Path, instance.HostUser)
	if err != nil {
		return err
	}
	target, err := instanceTarget(installation, instance)
	if err != nil {
		return err
	}
	return captureLink(instance, target)
}

// logGained reports whether marker was written to a log since the last poll.
// What a log held when Gateway first looked is old news; one that did not exist
// yet is read from its start once it does.
func logGained(path string, marker []byte, offsets map[string]int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		offsets[path] = 0
		return false
	}
	size := info.Size()
	offset, seen := offsets[path]
	offsets[path] = size
	if !seen || size == offset {
		return false
	}
	// A rotated log starts over.
	if size < offset {
		offset = 0
	}
	if size-offset > maxSignInLogRead {
		offset = size - maxSignInLogRead
	}

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	data := make([]byte, size-offset)
	n, _ := file.ReadAt(data, offset)
	return bytes.Contains(data[:n], marker)
}
