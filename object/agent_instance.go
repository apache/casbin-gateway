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

	"github.com/apache/casbin-gateway/util"
	"github.com/xorm-io/core"
)

// AgentInstance is one extra copy of an agent, kept apart from the others by a
// state directory of its own so that each can be signed in to a different
// account and used at the same time.
//
// The installation is shared: an instance is a second set of state, not a
// second copy of the program.
type AgentInstance struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(300) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	AgentId string `xorm:"varchar(100) index" json:"agentId"`
	// Instance is the name within the agent, which is also the last segment of
	// the state directory.
	Instance    string `xorm:"varchar(100)" json:"instance"`
	DisplayName string `xorm:"varchar(200)" json:"displayName"`
	// DataDir is the private state directory this instance is started with.
	DataDir string `xorm:"varchar(500)" json:"dataDir"`
	// Path and HostUser are the installation the instance runs, as a scan
	// reported it.
	Path     string `xorm:"varchar(500)" json:"path"`
	HostUser string `xorm:"varchar(200)" json:"hostUser"`
}

func (instance *AgentInstance) GetId() string {
	return fmt.Sprintf("%s/%s", instance.Owner, instance.Name)
}

// AgentInstanceName is the stored name of one instance: the agent it belongs to
// and the name chosen for it, since the same name may be used under two agents.
func AgentInstanceName(agentId string, instance string) string {
	return agentId + "/" + instance
}

// GetAgentInstances returns the instances of one agent, or of every agent when
// agentId is empty, oldest first so the list does not reshuffle.
func GetAgentInstances(agentId string) ([]*AgentInstance, error) {
	instances := []*AgentInstance{}
	session := ormer.Engine.Where("owner = ?", AgentOwner)
	if agentId != "" {
		session = session.And("agent_id = ?", agentId)
	}
	if err := session.Asc("created_time").Find(&instances); err != nil {
		return nil, err
	}
	return instances, nil
}

func GetAgentInstance(name string) (*AgentInstance, error) {
	instance := &AgentInstance{Owner: AgentOwner, Name: name}
	existed, err := ormer.Engine.Get(instance)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, nil
	}
	return instance, nil
}

// AddAgentInstance stores one instance. The name is what the state directory is
// built from, so a second row under it would hand two instances the same state.
func AddAgentInstance(instance *AgentInstance) error {
	if instance.AgentId == "" || instance.Instance == "" {
		return errors.New("the agent and the instance name are required")
	}

	instance.Owner = AgentOwner
	instance.Name = AgentInstanceName(instance.AgentId, instance.Instance)
	stored, err := GetAgentInstance(instance.Name)
	if err != nil {
		return err
	}
	if stored != nil {
		return fmt.Errorf("this agent already has an instance named %s", instance.Instance)
	}

	instance.CreatedTime = util.GetCurrentTime()
	instance.UpdatedTime = instance.CreatedTime
	_, err = ormer.Engine.Insert(instance)
	return err
}

// DeleteAgentInstance forgets one instance. The state directory is left where it
// is: it holds a signed-in account and whatever that account did, which is not
// Gateway's to throw away.
func DeleteAgentInstance(name string) error {
	_, err := ormer.Engine.Delete(&AgentInstance{Owner: AgentOwner, Name: name})
	return err
}

// SetAgentInstanceDisplayName renames one instance in the lists. Only the label
// moves: the name is what the state directory was laid out under, and renaming
// that would leave the account behind.
func SetAgentInstanceDisplayName(name string, displayName string) error {
	stored, err := GetAgentInstance(name)
	if err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("no agent instance is stored under this name: %s", name)
	}

	// Cols() is what writes an empty label: xorm skips zero values otherwise.
	_, err = ormer.Engine.ID(core.PK{AgentOwner, name}).
		Cols("display_name", "updated_time").
		Update(&AgentInstance{DisplayName: displayName, UpdatedTime: util.GetCurrentTime()})
	return err
}
