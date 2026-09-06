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

package agent

import "github.com/apache/casbin-gateway/agenthome"

// ConfigOwnerOf is the account whose home an agent's configuration files live
// in. It is not the account a Gateway row is stored under: that one names who
// signed in here, and writing a file needs the account the agent runs as.
//
// Installations of one agent under several accounts share an id, so the first
// one whose home this Gateway can actually reach wins. An agent no scan found
// falls back to the account Gateway runs as, which is where its files would be.
func ConfigOwnerOf(agentId string) string {
	installations, err := Scan(false)
	if err != nil {
		return ""
	}

	for _, installation := range installations {
		if installation.AgentId != agentId {
			continue
		}
		if _, err := agenthome.Resolve(installation.Owner); err == nil {
			return installation.Owner
		}
	}
	return ""
}
