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
	"sync"

	"github.com/apache/casbin-gateway/agentauth"
	"github.com/apache/casbin-gateway/agentusage"
)

// AgentAccountUsage is what one stored account has left, or why that could not
// be read.
type AgentAccountUsage struct {
	Usage *agentusage.Usage `json:"usage,omitempty"`
	Error string            `json:"error,omitempty"`
}

// GetAgentAccountsUsage reads what each stored sign-in of one agent has left,
// keyed by the stored name. An account it cannot ask about is left out rather
// than reported as a failure.
func GetAgentAccountsUsage(agentId string) (map[string]*AgentAccountUsage, error) {
	accounts, err := GetAgentAccounts(agentId)
	if err != nil {
		return nil, err
	}

	answers := map[string]*AgentAccountUsage{}
	var lock sync.Mutex
	var group sync.WaitGroup
	for _, account := range accounts {
		if account.Kind != agentauth.KindSubscription || !agentauth.Supports(account.AgentId) {
			continue
		}

		group.Add(1)
		go func(name string) {
			defer group.Done()
			answer := agentAccountUsage(name)

			lock.Lock()
			answers[name] = answer
			lock.Unlock()
		}(account.Name)
	}
	group.Wait()
	return answers, nil
}

func agentAccountUsage(name string) *AgentAccountUsage {
	account, err := GetAgentAccount(name)
	if err != nil {
		return &AgentAccountUsage{Error: err.Error()}
	}
	if account == nil {
		return &AgentAccountUsage{Error: fmt.Sprintf("no agent account is stored under this name: %s", name)}
	}

	usage, err := agentusage.Codex(account.Credential)
	if err != nil {
		return &AgentAccountUsage{Error: err.Error()}
	}
	return &AgentAccountUsage{Usage: usage}
}
