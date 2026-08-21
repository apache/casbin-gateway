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
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
	"github.com/xorm-io/xorm"
)

const llmRecordSummaryRunes = 160

// LlmRecord is one client request relayed by the LLM proxy, together with the
// outcome it ended in. Payload is filled only in "full" mode, and only after
// auditutil has removed the credentials the body may carry.
type LlmRecord struct {
	Id          int64  `xorm:"int notnull pk autoincr" json:"id"`
	CreatedTime string `xorm:"varchar(100) notnull index" json:"createdTime"`

	Protocol string `xorm:"varchar(20)" json:"protocol"`
	Endpoint string `xorm:"varchar(100)" json:"endpoint"`
	Model    string `xorm:"varchar(255) index" json:"model"`
	Channel  string `xorm:"varchar(201) index" json:"channel"`
	Agent    string `xorm:"varchar(201) index" json:"agent"`
	ClientIp string `xorm:"varchar(100) index" json:"clientIp"`
	Stream   bool   `xorm:"bool" json:"stream"`

	Status     int    `xorm:"int index" json:"status"`
	DurationMs int64  `xorm:"bigint" json:"durationMs"`
	Attempts   int    `xorm:"int" json:"attempts"`
	Error      string `xorm:"varchar(500)" json:"error"`

	PromptTokens     int `xorm:"int" json:"promptTokens"`
	CompletionTokens int `xorm:"int" json:"completionTokens"`
	TotalTokens      int `xorm:"int" json:"totalTokens"`

	Summary    string `xorm:"varchar(500)" json:"summary"`
	Payload    string `xorm:"mediumtext" json:"payload"`
	Redactions int    `xorm:"int" json:"redactions"`
	Truncated  bool   `xorm:"bool" json:"truncated"`
	Bytes      int    `xorm:"int" json:"bytes"`
}

// LlmRecordFilter narrows the record list. Empty fields match everything.
type LlmRecordFilter struct {
	Model    string
	Channel  string
	Agent    string
	ClientIp string
	Outcome  string
}

// LlmRecordStatus is what the management page shows about the recorder itself,
// so that a gap in the records is visible instead of silent.
type LlmRecordStatus struct {
	Mode          string `json:"mode"`
	RetentionDays int    `json:"retentionDays"`
	MaxRecords    int    `json:"maxRecords"`
	Dropped       int64  `json:"dropped"`
	Count         int64  `json:"count"`
}

type llmRecordTask struct {
	record  *LlmRecord
	rawBody []byte
}

type llmRecordWriter struct {
	mutex sync.RWMutex
	queue chan llmRecordTask
	done  chan struct{}

	dropped     atomic.Int64
	dropLogTime atomic.Int64
}

var llmWriter llmRecordWriter

// StartLlmRecordWriter starts the writer that owns every parse, sanitization
// and database write, so the proxy goroutine never does any of them.
func StartLlmRecordWriter() {
	if conf.GetLlmRecordMode() == conf.LlmRecordOff {
		return
	}

	llmWriter.mutex.Lock()
	defer llmWriter.mutex.Unlock()
	if llmWriter.queue != nil {
		return
	}
	llmWriter.queue = make(chan llmRecordTask, conf.GetLlmRecordQueueCapacity())
	llmWriter.done = make(chan struct{})
	go llmWriter.run(llmWriter.queue, llmWriter.done)
}

// StopLlmRecordWriter drains the records already accepted.
func StopLlmRecordWriter() {
	llmWriter.mutex.Lock()
	queue, done := llmWriter.queue, llmWriter.done
	llmWriter.queue, llmWriter.done = nil, nil
	llmWriter.mutex.Unlock()

	if queue == nil {
		return
	}
	close(queue)
	<-done
}

// IsLlmRecording reports whether a record would be kept at all, so the proxy
// can skip building one.
func IsLlmRecording() bool {
	return conf.GetLlmRecordMode() != conf.LlmRecordOff
}

// AddLlmRecord queues one finished request. Recording is best effort: a full
// queue drops the record rather than holding the request up.
func AddLlmRecord(record *LlmRecord, rawBody []byte) {
	if record == nil {
		return
	}
	record.CreatedTime = util.GetCurrentTime()
	record.Bytes = len(rawBody)
	if conf.GetLlmRecordMode() != conf.LlmRecordFull {
		rawBody = nil
	} else if len(rawBody) > auditutil.MaxPayloadBytes {
		record.Truncated = true
		rawBody = nil
	}

	llmWriter.mutex.RLock()
	defer llmWriter.mutex.RUnlock()
	if llmWriter.queue == nil {
		return
	}
	select {
	case llmWriter.queue <- llmRecordTask{record: record, rawBody: rawBody}:
	default:
		llmWriter.noteDrop()
	}
}

func (writer *llmRecordWriter) noteDrop() {
	dropped := writer.dropped.Add(1)
	last := writer.dropLogTime.Load()
	now := time.Now().Unix()
	if now-last < 60 || !writer.dropLogTime.CompareAndSwap(last, now) {
		return
	}
	beego.Error("LLM record queue is full, dropped records so far:", dropped)
}

func (writer *llmRecordWriter) run(queue chan llmRecordTask, done chan struct{}) {
	defer close(done)

	writer.prune()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case task, ok := <-queue:
			if !ok {
				return
			}
			writer.write(task)
		case <-ticker.C:
			writer.prune()
		}
	}
}

func (writer *llmRecordWriter) write(task llmRecordTask) {
	if ormer == nil || ormer.Engine == nil {
		return
	}
	if len(task.rawBody) > 0 {
		fillLlmRecordBody(task.record, task.rawBody)
	}
	if _, err := ormer.Engine.Insert(task.record); err != nil {
		beego.Error("LLM record write failed:", err)
	}
}

func (writer *llmRecordWriter) prune() {
	if ormer == nil || ormer.Engine == nil {
		return
	}

	cutoff := util.FormatTime(time.Now().AddDate(0, 0, -conf.GetLlmRecordRetentionDays()))
	if _, err := ormer.Engine.Where("created_time < ?", cutoff).Delete(&LlmRecord{}); err != nil {
		beego.Error("LLM record retention cleanup failed:", err)
		return
	}

	count, err := ormer.Engine.Count(&LlmRecord{})
	if err != nil {
		beego.Error("LLM record retention count failed:", err)
		return
	}
	maximum := int64(conf.GetLlmRecordMaxRecords())
	if count <= maximum {
		return
	}

	oldest := []LlmRecord{}
	if err := ormer.Engine.Cols("id").Asc("id").Limit(int(count - maximum)).Find(&oldest); err != nil {
		beego.Error("LLM record retention lookup failed:", err)
		return
	}
	if len(oldest) == 0 {
		return
	}
	ids := make([]int64, 0, len(oldest))
	for _, record := range oldest {
		ids = append(ids, record.Id)
	}
	if _, err := ormer.Engine.In("id", ids).Delete(&LlmRecord{}); err != nil {
		beego.Error("LLM record retention deletion failed:", err)
	}
}

// fillLlmRecordBody is the only place a request body is retained, and it runs
// on the writer goroutine. Request headers, which is where the inbound
// credentials are, never reach a record at all.
func fillLlmRecordBody(record *LlmRecord, rawBody []byte) {
	var decoded any
	if json.Unmarshal(rawBody, &decoded) != nil {
		return
	}

	sanitized := auditutil.SanitizeValue("", decoded)
	encoded, err := json.Marshal(sanitized)
	if err != nil {
		return
	}
	if len(encoded) > auditutil.MaxPayloadBytes {
		record.Truncated = true
		record.Payload = auditutil.EncodeBoundedJSON(sanitized, auditutil.MaxPayloadBytes)
	} else {
		record.Payload = string(encoded)
	}
	record.Redactions = strings.Count(record.Payload, "[REDACTED")
	record.Summary = llmRecordSummary(sanitized)
}

// llmRecordSummary is the last thing the user asked for, which is what makes a
// row recognizable without opening it. Both APIs name the field the same way.
func llmRecordSummary(value any) string {
	body, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	messages, ok := body["messages"].([]any)
	if !ok {
		return ""
	}

	for i := len(messages) - 1; i >= 0; i-- {
		message, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		if role, _ := message["role"].(string); role != "user" {
			continue
		}
		if text := llmMessageText(message["content"]); text != "" {
			return boundRunes(strings.Join(strings.Fields(text), " "), llmRecordSummaryRunes)
		}
	}
	return ""
}

func llmMessageText(content any) string {
	switch typed := content.(type) {
	case string:
		return typed
	case []any:
		parts := []string{}
		for _, block := range typed {
			blockMap, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := blockMap["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func boundRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum]) + "..."
}

func llmRecordSession(filter LlmRecordFilter) *xorm.Session {
	session := ormer.Engine.NewSession()
	if filter.Model != "" {
		session = session.And("model like ?", "%"+filter.Model+"%")
	}
	if filter.Channel != "" {
		session = session.And("channel like ?", "%"+filter.Channel+"%")
	}
	if filter.Agent != "" {
		session = session.And("agent = ?", filter.Agent)
	}
	if filter.ClientIp != "" {
		session = session.And("client_ip = ?", filter.ClientIp)
	}
	switch filter.Outcome {
	case "ok":
		session = session.And("status >= 200").And("status < 300")
	case "error":
		session = session.And("status = 0 or status < 200 or status >= 300")
	}
	return session
}

// GetLlmRecords lists one page. The payload is left out on purpose: a list of
// 25 prompts would be megabytes, and the page only needs it once a row is
// opened.
func GetLlmRecords(filter LlmRecordFilter, offset int, limit int) ([]*LlmRecord, int64, error) {
	countSession := llmRecordSession(filter)
	defer countSession.Close()
	count, err := countSession.Count(&LlmRecord{})
	if err != nil {
		return nil, 0, err
	}

	session := llmRecordSession(filter)
	defer session.Close()
	records := []*LlmRecord{}
	err = session.Omit("payload").Desc("id").Limit(limit, offset).Find(&records)
	return records, count, err
}

func GetLlmRecord(id int64) (*LlmRecord, error) {
	record := LlmRecord{Id: id}
	existed, err := ormer.Engine.Get(&record)
	if err != nil || !existed {
		return nil, err
	}
	return &record, nil
}

func DeleteLlmRecord(id int64) error {
	_, err := ormer.Engine.ID(id).Delete(&LlmRecord{})
	return err
}

func ClearLlmRecords() error {
	_, err := ormer.Engine.Where("id > 0").Delete(&LlmRecord{})
	return err
}

// LlmUsageTotals is the headline token usage over the queried window.
type LlmUsageTotals struct {
	Requests         int64 `json:"requests"`
	PromptTokens     int64 `json:"promptTokens"`
	CompletionTokens int64 `json:"completionTokens"`
	TotalTokens      int64 `json:"totalTokens"`
}

// llmUsageDimensions whitelists the columns a caller may group usage by. The
// value is interpolated into the SELECT/GROUP BY clause, so it must never come
// straight from the request.
var llmUsageDimensions = map[string]bool{"model": true, "channel": true, "agent": true}

// llmUsageSession applies the record filter and the time window shared by every
// usage query.
func llmUsageSession(filter LlmRecordFilter, startAt time.Time) *xorm.Session {
	return llmRecordSession(filter).Table("llm_record").And("created_time > ?", util.FormatTime(startAt))
}

// GetLlmUsageTotals returns the request count and summed token usage over the
// window.
func GetLlmUsageTotals(filter LlmRecordFilter, startAt time.Time) (*LlmUsageTotals, error) {
	session := llmUsageSession(filter, startAt)
	defer session.Close()

	totals := &LlmUsageTotals{}
	_, err := session.Select("COUNT(*) AS requests, " +
		"COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, " +
		"COALESCE(SUM(completion_tokens), 0) AS completion_tokens, " +
		"COALESCE(SUM(total_tokens), 0) AS total_tokens").Get(totals)
	return totals, err
}

// GetLlmTokensByDimension totals the tokens per model, channel or agent, largest
// first, so the caller can chart where the usage goes. Rows with an empty
// dimension (e.g. the agent of a model-routed request) are left out.
func GetLlmTokensByDimension(filter LlmRecordFilter, startAt time.Time, dimension string, top int) ([]*DataCount, error) {
	if !llmUsageDimensions[dimension] {
		return nil, fmt.Errorf("invalid usage dimension: %s", dimension)
	}

	session := llmUsageSession(filter, startAt)
	defer session.Close()

	rows := []*DataCount{}
	err := session.
		And(dimension + " <> ''").
		Select(dimension + " AS data, COALESCE(SUM(total_tokens), 0) AS count").
		GroupBy(dimension).
		Desc("count").
		Limit(top).
		Find(&rows)
	return rows, err
}

// GetLlmTokensOverTime totals the tokens per time bucket (hour, day, month) so
// the caller can chart usage over the window.
func GetLlmTokensOverTime(filter LlmRecordFilter, startAt time.Time, timeType string) ([]*DataCount, error) {
	bucket := getCreatedTimeBucket(ormer.driverName, timeType)

	session := llmUsageSession(filter, startAt)
	defer session.Close()

	rows := []*DataCount{}
	err := session.
		Select(bucket + " AS data, COALESCE(SUM(total_tokens), 0) AS count").
		GroupBy(bucket).
		Asc("data").
		Find(&rows)
	return rows, err
}

func GetLlmRecordStatus() (*LlmRecordStatus, error) {
	count, err := ormer.Engine.Count(&LlmRecord{})
	if err != nil {
		return nil, err
	}
	return &LlmRecordStatus{
		Mode:          conf.GetLlmRecordMode(),
		RetentionDays: conf.GetLlmRecordRetentionDays(),
		MaxRecords:    conf.GetLlmRecordMaxRecords(),
		Dropped:       llmWriter.dropped.Load(),
		Count:         count,
	}, nil
}
