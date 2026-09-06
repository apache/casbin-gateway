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

package agentsession

import (
	"os"
	"sync"
)

// EnvSource is what one driven agent is started with on top of Gateway's own
// environment: the provider bound to it, for that one process. It lives outside
// this package for the same reason the store does - driving an agent must not
// depend on the database.
type EnvSource func(Session) []string

var envSource struct {
	sync.RWMutex
	source EnvSource
}

// SetEnvSource installs where a driven agent's provider is read from.
func SetEnvSource(source EnvSource) {
	envSource.Lock()
	envSource.source = source
	envSource.Unlock()
}

// environ is the environment one turn runs in. What the source adds is appended
// rather than merged: os/exec keeps the last of a repeated variable, so a
// provider Gateway resolved wins over one this process happens to carry.
func environ(session Session) []string {
	envSource.RLock()
	source := envSource.source
	envSource.RUnlock()

	env := os.Environ()
	if source == nil {
		return env
	}
	return append(env, source(session)...)
}
