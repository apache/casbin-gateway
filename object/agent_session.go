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

	"github.com/apache/casbin-gateway/agentsession"
	"github.com/beego/beego"
	"github.com/xorm-io/core"
)

// AgentSession is one conversation Gateway is driving, kept so that it outlives
// a restart: the agent's own id for it is what a later turn resumes, and losing
// that would strand the conversation inside the agent.
//
// What was said is not stored here. The agent keeps its own transcript, which
// the monitor already reads, and a second copy would only be one more place for
// somebody's prompts to sit.
type AgentSession struct {
	Owner string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name  string `xorm:"varchar(100) notnull pk" json:"name"`

	AgentId   string `xorm:"varchar(100) index" json:"agentId"`
	AgentPath string `xorm:"varchar(500)" json:"agentPath"`
	AgentUser string `xorm:"varchar(100)" json:"agentUser"`
	WorkDir   string `xorm:"varchar(500)" json:"workDir"`
	Model     string `xorm:"varchar(200)" json:"model"`
	// Source names who opened the session: "web", or "im:<platform>:<user>".
	Source string `xorm:"varchar(200)" json:"source"`

	// NativeId is the agent's own id for the conversation, which is the whole
	// reason a session is stored at all.
	NativeId  string `xorm:"varchar(200)" json:"nativeId"`
	Resumable bool   `json:"resumable"`

	Title       string `xorm:"varchar(200)" json:"title"`
	Turns       int    `json:"turns"`
	LastError   string `xorm:"varchar(1000)" json:"lastError"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100) index" json:"updatedTime"`
}

// agentSessionOwner is who every session belongs to. Gateway serves one machine,
// so that is its admin, the same as an agent's own row.
const agentSessionOwner = AgentOwner

func toAgentSession(session *agentsession.Session) *AgentSession {
	return &AgentSession{
		Owner:       agentSessionOwner,
		Name:        session.Id,
		AgentId:     session.AgentId,
		AgentPath:   session.AgentPath,
		AgentUser:   session.Owner,
		WorkDir:     session.WorkDir,
		Model:       session.Model,
		Source:      session.Source,
		NativeId:    session.NativeId,
		Resumable:   session.Resumable,
		Title:       session.Title,
		Turns:       session.Turns,
		LastError:   session.LastError,
		CreatedTime: session.CreatedTime,
		UpdatedTime: session.UpdatedTime,
	}
}

func (session *AgentSession) toDriven() agentsession.Session {
	return agentsession.Session{
		Id: session.Name,
		Spec: agentsession.Spec{
			AgentId:   session.AgentId,
			AgentPath: session.AgentPath,
			Owner:     session.AgentUser,
			WorkDir:   session.WorkDir,
			Model:     session.Model,
			Source:    session.Source,
		},
		NativeId:    session.NativeId,
		Resumable:   session.Resumable,
		Title:       session.Title,
		Turns:       session.Turns,
		LastError:   session.LastError,
		CreatedTime: session.CreatedTime,
		UpdatedTime: session.UpdatedTime,
	}
}

// SaveAgentSession writes one session, inserting it the first time. It is the
// sink the driver hands every change to, so it must not fail loudly: a session
// that cannot be stored still answers, it just cannot be resumed after a
// restart.
func SaveAgentSession(session *agentsession.Session) {
	row := toAgentSession(session)
	affected, err := ormer.Engine.ID(core.PK{row.Owner, row.Name}).AllCols().Update(row)
	if err == nil && affected == 0 {
		_, err = ormer.Engine.Insert(row)
	}
	if err != nil {
		beego.Error("agent session could not be stored:", err)
	}
}

// DeleteAgentSession removes one stored session.
func DeleteAgentSession(id string) error {
	_, err := ormer.Engine.ID(core.PK{agentSessionOwner, id}).Delete(&AgentSession{})
	return err
}

// GetAgentSessions is every stored session, most recently used first.
func GetAgentSessions() ([]*AgentSession, error) {
	sessions := []*AgentSession{}
	if err := ormer.Engine.Where("owner = ?", agentSessionOwner).Find(&sessions); err != nil {
		return nil, err
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedTime > sessions[j].UpdatedTime
	})
	return sessions, nil
}

// InitAgentSessions puts the stored sessions back in the driver and starts
// keeping it up to date. Nothing is running after this: a restored session only
// holds the agent's own id, and starts an agent again when it is next asked
// something.
func InitAgentSessions() {
	sessions, err := GetAgentSessions()
	if err != nil {
		panic(err)
	}
	for _, session := range sessions {
		agentsession.Restore(session.toDriven())
	}

	agentsession.SetSessionSink(SaveAgentSession)
}
