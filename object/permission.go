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
	"errors"
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// How a list of models or providers is read. "all" ignores the list, "allow"
// permits nothing but what it holds, and "deny" permits everything else.
const (
	ListAll   = "all"
	ListAllow = "allow"
	ListDeny  = "deny"
)

// AgentPermission is what one agent is allowed to ask the gateway for. It is
// the friendly form of the rules: the switches of the Permissions card are
// stored as they are set, and compiled into the casbin policy that decides
// every request. See permission_casbin.go for that compilation.
type AgentPermission struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	// Enabled turns the rules on. An agent with it off is left unrestricted,
	// which is what every agent is until someone says otherwise.
	Enabled bool `xorm:"bool" json:"enabled"`

	// ModelMode reads Models: ListAll, ListAllow or ListDeny. An entry may end
	// in "*", so "claude-opus-*" covers a whole family.
	ModelMode string   `xorm:"varchar(20)" json:"modelMode"`
	Models    []string `xorm:"mediumtext" json:"models"`

	// ProviderMode reads Providers, which hold "owner/name" provider ids.
	ProviderMode string   `xorm:"varchar(20)" json:"providerMode"`
	Providers    []string `xorm:"mediumtext" json:"providers"`

	// Tools is one entry per tool group, false where the group is switched off.
	// A group that is missing is allowed, so a group added in a later version
	// does not silently block the agents configured before it existed.
	Tools map[string]bool `xorm:"mediumtext" json:"tools"`

	// Rules are extra casbin policy lines, written by hand on the advanced view
	// and appended to the compiled ones. Each is "sub, obj, act, eft".
	Rules []string `xorm:"mediumtext" json:"rules"`
}

func (permission *AgentPermission) GetId() string {
	return fmt.Sprintf("%s/%s", permission.Owner, permission.Name)
}

// DefaultAgentPermission is what an agent nobody has configured is held to:
// nothing at all.
func DefaultAgentPermission(agentId string) *AgentPermission {
	return &AgentPermission{
		Owner:        AgentOwner,
		Name:         agentId,
		Enabled:      false,
		ModelMode:    ListAll,
		Models:       []string{},
		ProviderMode: ListAll,
		Providers:    []string{},
		Tools:        map[string]bool{},
		Rules:        []string{},
	}
}

// GetAgentPermission reads one agent's permissions, answering with the default
// ones when it has none stored.
func GetAgentPermission(agentId string) (*AgentPermission, error) {
	if agentId == "" {
		return nil, errors.New("the agent id is empty")
	}

	permission := &AgentPermission{Owner: AgentOwner, Name: agentId}
	existed, err := ormer.Engine.Get(permission)
	if err != nil {
		return nil, err
	}
	if !existed {
		return DefaultAgentPermission(agentId), nil
	}

	normalizeAgentPermission(permission)
	return permission, nil
}

// GetAgentPermissions returns every stored permission, keyed by agent id. The
// agents nobody has configured are absent rather than listed as unrestricted.
func GetAgentPermissions() (map[string]*AgentPermission, error) {
	permissions := []*AgentPermission{}
	if err := ormer.Engine.Where("owner = ?", AgentOwner).Find(&permissions); err != nil {
		return nil, err
	}

	result := map[string]*AgentPermission{}
	for _, permission := range permissions {
		normalizeAgentPermission(permission)
		result[permission.Name] = permission
	}
	return result, nil
}

// UpdateAgentPermission stores one agent's permissions, creating the row on
// first save. The enforcer built from them is dropped, so the next request is
// decided by what was just saved.
func UpdateAgentPermission(agentId string, permission *AgentPermission) error {
	if agentId == "" {
		return errors.New("the agent id is empty")
	}
	if err := validateAgentPermission(permission); err != nil {
		return err
	}

	permission.Owner = AgentOwner
	permission.Name = agentId
	permission.UpdatedTime = util.GetCurrentTime()
	normalizeAgentPermission(permission)

	stored := &AgentPermission{Owner: AgentOwner, Name: agentId}
	existed, err := ormer.Engine.Get(stored)
	if err != nil {
		return err
	}

	if !existed {
		permission.CreatedTime = permission.UpdatedTime
		if _, err = ormer.Engine.Insert(permission); err != nil {
			return err
		}
	} else {
		permission.CreatedTime = stored.CreatedTime
		// Cols() is what writes an emptied list or a switch turned off: xorm
		// skips zero values otherwise.
		_, err = ormer.Engine.ID(core.PK{AgentOwner, agentId}).
			Cols("enabled", "model_mode", "models", "provider_mode", "providers", "tools", "rules", "updated_time").
			Update(permission)
		if err != nil {
			return err
		}
	}

	dropAgentEnforcer(agentId)
	return nil
}

// validateAgentPermission refuses what would be stored but never understood,
// so a typo fails at the form rather than on the next relayed request.
func validateAgentPermission(permission *AgentPermission) error {
	if permission == nil {
		return errors.New("the permission is empty")
	}

	for _, mode := range []string{permission.ModelMode, permission.ProviderMode} {
		switch mode {
		case "", ListAll, ListAllow, ListDeny:
		default:
			return fmt.Errorf("invalid list mode: %s", mode)
		}
	}

	for _, rule := range permission.Rules {
		if strings.TrimSpace(rule) == "" {
			continue
		}
		if _, err := parsePolicyRule(rule); err != nil {
			return err
		}
	}
	return nil
}

// normalizeAgentPermission fills in what an older row, or a client that left a
// field out, does not carry.
func normalizeAgentPermission(permission *AgentPermission) {
	if permission.ModelMode == "" {
		permission.ModelMode = ListAll
	}
	if permission.ProviderMode == "" {
		permission.ProviderMode = ListAll
	}
	if permission.Models == nil {
		permission.Models = []string{}
	}
	if permission.Providers == nil {
		permission.Providers = []string{}
	}
	if permission.Tools == nil {
		permission.Tools = map[string]bool{}
	}
	if permission.Rules == nil {
		permission.Rules = []string{}
	}

	permission.Models = trimList(permission.Models)
	permission.Providers = trimList(permission.Providers)
	permission.Rules = trimList(permission.Rules)
}

func trimList(values []string) []string {
	result := []string{}
	seen := map[string]bool{"": true}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}
