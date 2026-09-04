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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/casbin-gateway/agenthistory"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/conf"
	"github.com/beego/beego"
	"github.com/xorm-io/xorm"
)

// agentRecordQueueCapacity bounds the records waiting to be written. A tailer
// must never wait on the database, so a full queue drops instead.
const agentRecordQueueCapacity = 1000

// AgentRecord is one observed agent behaviour, reported by a log tailer, a hook
// or the MCP recorder. Unlike LlmRecord it says nothing about what Gateway
// relayed: an agent talking to its vendor directly is recorded here all the same.
type AgentRecord struct {
	Id          int64  `xorm:"int notnull pk autoincr" json:"id"`
	CreatedTime string `xorm:"varchar(100) notnull" json:"createdTime"`
	// SortKey is the created time in Unix nanoseconds. A tailer reports times in
	// whichever zone the log it reads is written in, so the text column alone
	// does not order rows the way a clock would.
	SortKey int64 `xorm:"bigint index" json:"-"`

	Agent     string `xorm:"varchar(201) index" json:"agent"`
	AgentPath string `xorm:"varchar(500)" json:"agentPath,omitempty"`
	ClientIp  string `xorm:"varchar(100)" json:"clientIp,omitempty"`
	User      string `xorm:"varchar(201)" json:"user,omitempty"`

	EventType  string `xorm:"varchar(50) index" json:"eventType"`
	Action     string `xorm:"varchar(100)" json:"action"`
	Outcome    string `xorm:"varchar(50) index" json:"outcome,omitempty"`
	SessionKey string `xorm:"varchar(201) index" json:"sessionKey,omitempty"`
	Title      string `xorm:"varchar(500)" json:"title,omitempty"`

	PromptId   string `xorm:"varchar(201)" json:"promptId,omitempty"`
	ToolUseId  string `xorm:"varchar(201)" json:"toolUseId,omitempty"`
	ToolName   string `xorm:"varchar(201)" json:"toolName,omitempty"`
	McpServer  string `xorm:"varchar(201)" json:"mcpServer,omitempty"`
	McpTool    string `xorm:"varchar(201)" json:"mcpTool,omitempty"`
	Model      string `xorm:"varchar(255)" json:"model,omitempty"`
	DurationMs int64  `xorm:"bigint" json:"durationMs,omitempty"`

	Object string `xorm:"mediumtext" json:"object,omitempty"`
	Detail string `xorm:"mediumtext" json:"detail,omitempty"`
}

// AgentRecordFilter narrows the record list. Empty fields match everything and
// a non-positive Limit returns every matching record.
type AgentRecordFilter struct {
	Agent     string
	EventType string
	Outcome   string
	Session   string
	Limit     int
}

// agentSessionColumns are what grouping records into sessions reads. A record
// may carry tens of kilobytes of payload, so the summary never selects one.
var agentSessionColumns = []string{
	"agent", "session_key", "title", "created_time",
	"event_type", "action", "tool_name", "mcp_server", "mcp_tool",
}

type agentRecordWriter struct {
	mutex sync.RWMutex
	queue chan *AgentRecord
	done  chan struct{}

	dropped     atomic.Int64
	dropLogTime atomic.Int64
}

var agentWriter agentRecordWriter

// StartAgentRecordWriter starts the goroutine that owns every agent record
// write, so a tailer or a hook never blocks on the database.
func StartAgentRecordWriter() {
	agentWriter.mutex.Lock()
	defer agentWriter.mutex.Unlock()
	if agentWriter.queue != nil {
		return
	}
	agentWriter.queue = make(chan *AgentRecord, agentRecordQueueCapacity)
	agentWriter.done = make(chan struct{})
	go agentWriter.run(agentWriter.queue, agentWriter.done)
}

// StopAgentRecordWriter drains the records already accepted.
func StopAgentRecordWriter() {
	agentWriter.mutex.Lock()
	queue, done := agentWriter.queue, agentWriter.done
	agentWriter.queue, agentWriter.done = nil, nil
	agentWriter.mutex.Unlock()

	if queue == nil {
		return
	}
	close(queue)
	<-done
}

// AddAgentRecord queues one observed behaviour. It is the sink agentmonitor
// reports to, and is best effort: a full queue drops the record rather than
// holding a tailer up.
func AddAgentRecord(record *agentmonitor.Record) {
	if record == nil {
		return
	}

	agentWriter.mutex.RLock()
	defer agentWriter.mutex.RUnlock()
	if agentWriter.queue == nil {
		return
	}
	select {
	case agentWriter.queue <- newAgentRecord(record):
	default:
		agentWriter.noteDrop()
	}
}

func newAgentRecord(record *agentmonitor.Record) *AgentRecord {
	return &AgentRecord{
		CreatedTime: record.CreatedTime,
		SortKey:     agentRecordSortKey(record.CreatedTime),
		Agent:       record.Agent,
		AgentPath:   record.AgentPath,
		ClientIp:    record.ClientIp,
		User:        record.User,
		EventType:   record.EventType,
		Action:      record.Action,
		Outcome:     record.Outcome,
		SessionKey:  record.SessionKey,
		Title:       record.Title,
		PromptId:    record.PromptId,
		ToolUseId:   record.ToolUseId,
		ToolName:    record.ToolName,
		McpServer:   record.McpServer,
		McpTool:     record.McpTool,
		Model:       record.Model,
		DurationMs:  record.DurationMs,
		Object:      record.Object,
		Detail:      record.Detail,
	}
}

func agentRecordSortKey(createdTime string) int64 {
	parsed, err := time.Parse(time.RFC3339Nano, createdTime)
	if err != nil {
		return time.Now().UnixNano()
	}
	return parsed.UnixNano()
}

func (writer *agentRecordWriter) noteDrop() {
	dropped := writer.dropped.Add(1)
	last := writer.dropLogTime.Load()
	now := time.Now().Unix()
	if now-last < 60 || !writer.dropLogTime.CompareAndSwap(last, now) {
		return
	}
	beego.Error("agent record queue is full, dropped records so far:", dropped)
}

func (writer *agentRecordWriter) run(queue chan *AgentRecord, done chan struct{}) {
	defer close(done)

	writer.prune()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case record, ok := <-queue:
			if !ok {
				return
			}
			writer.write(record)
		case <-ticker.C:
			writer.prune()
		}
	}
}

func (writer *agentRecordWriter) write(record *AgentRecord) {
	if ormer == nil || ormer.Engine == nil {
		return
	}
	if _, err := ormer.Engine.Insert(record); err != nil {
		beego.Error("agent record write failed:", err)
	}
}

// prune keeps the newest agentRecordCapacity rows. The records are a monitoring
// window rather than an audit trail, so the cap is the only retention rule.
func (writer *agentRecordWriter) prune() {
	if ormer == nil || ormer.Engine == nil {
		return
	}

	count, err := ormer.Engine.Count(&AgentRecord{})
	if err != nil {
		beego.Error("agent record retention count failed:", err)
		return
	}
	maximum := int64(conf.GetAgentRecordCapacity())
	if count <= maximum {
		return
	}

	oldest := []AgentRecord{}
	if err := ormer.Engine.Cols("id").Asc("id").Limit(int(count - maximum)).Find(&oldest); err != nil {
		beego.Error("agent record retention lookup failed:", err)
		return
	}
	if len(oldest) == 0 {
		return
	}
	ids := make([]int64, 0, len(oldest))
	for _, record := range oldest {
		ids = append(ids, record.Id)
	}
	if _, err := ormer.Engine.In("id", ids).Delete(&AgentRecord{}); err != nil {
		beego.Error("agent record retention deletion failed:", err)
	}
}

// GetAgentRecords returns matching records newest first.
func GetAgentRecords(filter AgentRecordFilter) []*AgentRecord {
	records := []*AgentRecord{}
	if ormer == nil || ormer.Engine == nil {
		return records
	}

	session := agentRecordSession(filter).Desc("sort_key").Desc("id")
	if filter.Limit > 0 {
		session = session.Limit(filter.Limit)
	}
	if err := session.Find(&records); err != nil {
		beego.Error("agent record lookup failed:", err)
		return []*AgentRecord{}
	}
	return records
}

// GetAgentRecordSessions groups the stored records into the sessions they
// belong to, newest first. It is the monitored half of the Sessions page, the
// other half being the transcripts on disk.
func GetAgentRecordSessions(agentId string) []agenthistory.Session {
	sessions := []agenthistory.Session{}
	if ormer == nil || ormer.Engine == nil {
		return sessions
	}

	records := []*AgentRecord{}
	query := agentRecordSession(AgentRecordFilter{Agent: agentId}).Cols(agentSessionColumns...)
	if err := query.And("session_key <> ?", "").Desc("sort_key").Desc("id").Find(&records); err != nil {
		beego.Error("agent session lookup failed:", err)
		return sessions
	}

	groups := map[string][]*AgentRecord{}
	order := []string{}
	for _, record := range records {
		key := record.Agent + "\x00" + record.SessionKey
		if _, found := groups[key]; !found {
			order = append(order, key)
		}
		groups[key] = append(groups[key], record)
	}

	for _, key := range order {
		group := groups[key]
		newest, oldest := group[0], group[len(group)-1]
		sessions = append(sessions, agenthistory.Session{
			Agent:       newest.Agent,
			SessionKey:  newest.SessionKey,
			Title:       agentSessionTitle(group),
			RecordCount: len(group),
			FirstTime:   oldest.CreatedTime,
			LastTime:    newest.CreatedTime,
		})
	}
	return sessions
}

func agentRecordSession(filter AgentRecordFilter) *xorm.Session {
	session := ormer.Engine.NewSession()
	if filter.Agent != "" {
		session = session.And("lower(agent) = ?", strings.ToLower(filter.Agent))
	}
	if filter.EventType != "" {
		session = session.And("lower(event_type) = ?", strings.ToLower(filter.EventType))
	}
	if filter.Outcome != "" {
		session = session.And("lower(outcome) = ?", strings.ToLower(filter.Outcome))
	}
	if filter.Session != "" {
		session = session.And("lower(session_key) = ?", strings.ToLower(filter.Session))
	}
	return session
}

// agentSessionTitle names a session by what it was seen doing: the title the
// agent reported, or failing that the first thing it called.
func agentSessionTitle(newestFirst []*AgentRecord) string {
	for _, record := range newestFirst {
		if record.Title != "" {
			return record.Title
		}
	}
	for index := len(newestFirst) - 1; index >= 0; index-- {
		record := newestFirst[index]
		if record.McpServer != "" {
			if record.McpTool != "" {
				return record.McpServer + " / " + record.McpTool
			}
			return record.McpServer
		}
		if record.ToolName != "" {
			return record.ToolName
		}
	}
	oldest := newestFirst[len(newestFirst)-1]
	return strings.TrimSpace(oldest.EventType + " " + oldest.Action)
}
