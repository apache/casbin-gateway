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

// llmOversizeFactor is how far past the payload limit a body may still be
// worth shrinking rather than dropping.
const llmOversizeFactor = 4

// llmStringCaps are tried in turn on a body over the limit, so that every
// message and tool stays visible where dropping the body would keep none.
var llmStringCaps = []int{16384, 4096, 1024, 256}

// LlmFailure is one provider attempt that did not answer, kept on the record of
// the request it was made for.
type LlmFailure struct {
	Provider string `json:"provider"`
	Status   int    `json:"status"`
	Error    string `json:"error"`
}

// LlmRecord is one client request relayed by the LLM proxy, together with the
// outcome it ended in. Payload is filled only in "full" mode, and only after
// auditutil has removed the credentials the body may carry.
type LlmRecord struct {
	Id          int64  `xorm:"int notnull pk autoincr" json:"id"`
	CreatedTime string `xorm:"varchar(100) notnull index" json:"createdTime"`

	Protocol string `xorm:"varchar(20)" json:"protocol"`
	Endpoint string `xorm:"varchar(100)" json:"endpoint"`
	Model    string `xorm:"varchar(255) index" json:"model"`
	Provider string `xorm:"varchar(201) index" json:"provider"`
	Agent    string `xorm:"varchar(201) index" json:"agent"`
	ClientIp string `xorm:"varchar(100) index" json:"clientIp"`
	Stream   bool   `xorm:"bool" json:"stream"`

	Status     int    `xorm:"int index" json:"status"`
	DurationMs int64  `xorm:"bigint" json:"durationMs"`
	Attempts   int    `xorm:"int" json:"attempts"`
	Error      string `xorm:"varchar(500)" json:"error"`

	// ErrorBody is what the upstream itself answered with, which is where the
	// reason for a failure normally is: the status code alone rarely says.
	ErrorBody string `xorm:"mediumtext" json:"errorBody"`
	// Failures are the attempts the chain failed over from, so a record shows
	// which providers were tried and why each of them did not answer.
	Failures []LlmFailure `xorm:"mediumtext" json:"failures"`

	// PromptTokens counts only the input billed as fresh: the cached part is
	// reported separately, so the counters can be added up without counting a
	// token twice.
	PromptTokens     int `xorm:"int" json:"promptTokens"`
	CompletionTokens int `xorm:"int" json:"completionTokens"`
	CacheReadTokens  int `xorm:"int" json:"cacheReadTokens"`
	CacheWriteTokens int `xorm:"int" json:"cacheWriteTokens"`
	ReasoningTokens  int `xorm:"int" json:"reasoningTokens"`
	TotalTokens      int `xorm:"int" json:"totalTokens"`

	// Cost is in US dollars, and means anything only where Priced is true.
	Cost   float64 `xorm:"double" json:"cost"`
	Priced bool    `xorm:"bool" json:"priced"`

	// Counted while the body is parsed, so the list can show the shape of a
	// request without loading it.
	SystemBytes  int `xorm:"int" json:"systemBytes"`
	MessageCount int `xorm:"int" json:"messageCount"`
	ToolCount    int `xorm:"int" json:"toolCount"`

	Summary    string `xorm:"varchar(500)" json:"summary"`
	Payload    string `xorm:"mediumtext" json:"payload"`
	Redactions int    `xorm:"int" json:"redactions"`
	Truncated  bool   `xorm:"bool" json:"truncated"`
	Bytes      int    `xorm:"int" json:"bytes"`
}

// LlmRecordFilter narrows the record list. Empty fields match everything.
type LlmRecordFilter struct {
	Model    string
	Provider string
	Agent    string
	ClientIp string
	Outcome  string
	// Since bounds the window to records created after it, formatted the way
	// CreatedTime is. Empty means every record still retained.
	Since string
}

// LlmModelStat is one model's share of the window the stats cover.
type LlmModelStat struct {
	Model    string  `json:"model"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// LlmProviderStat is one provider's share of the window the stats cover, which is
// what tells two providers serving the same models apart.
type LlmProviderStat struct {
	Provider string  `json:"provider"`
	Requests int64   `json:"requests"`
	Failed   int64   `json:"failed"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// LlmAgentStat is one agent's share of the records, with what it last asked
// for: the page listing every agent shows the model in use, not only totals.
type LlmAgentStat struct {
	Agent        string  `json:"agent"`
	Requests     int64   `json:"requests"`
	Failed       int64   `json:"failed"`
	Tokens       int64   `json:"tokens"`
	Cost         float64 `json:"cost"`
	LastTime     string  `json:"lastTime"`
	LastModel    string  `json:"lastModel"`
	LastProvider string  `json:"lastProvider"`
}

// LlmRecordStats totals the records a filter matches.
type LlmRecordStats struct {
	Requests         int64             `json:"requests"`
	Failed           int64             `json:"failed"`
	PromptTokens     int64             `json:"promptTokens"`
	CompletionTokens int64             `json:"completionTokens"`
	CacheReadTokens  int64             `json:"cacheReadTokens"`
	CacheWriteTokens int64             `json:"cacheWriteTokens"`
	TotalTokens      int64             `json:"totalTokens"`
	Cost             float64           `json:"cost"`
	Unpriced         int64             `json:"unpriced"`
	Models           []LlmModelStat    `json:"models"`
	Providers        []LlmProviderStat `json:"providers"`
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
	} else if len(rawBody) > conf.GetLlmRecordMaxPayloadBytes()*llmOversizeFactor {
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
	fillLlmRecordCost(task.record)
	if _, err := ormer.Engine.Insert(task.record); err != nil {
		beego.Error("LLM record write failed:", err)
		return
	}
	publishLlmRecord(task.record)
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
	describeLlmRequest(record, sanitized)

	payload, truncated := encodeLlmBody(sanitized, conf.GetLlmRecordMaxPayloadBytes())
	record.Payload = payload
	record.Truncated = record.Truncated || truncated
	record.Redactions = strings.Count(record.Payload, "[REDACTED")
	record.Summary = llmRecordSummary(sanitized)
}

// fillLlmRecordCost prices a record from its token counters.
func fillLlmRecordCost(record *LlmRecord) {
	record.Cost, record.Priced = GetLlmCost(
		record.Model, record.PromptTokens, record.CompletionTokens, record.CacheWriteTokens, record.CacheReadTokens)
}

// describeLlmRequest counts what the request was made of.
func describeLlmRequest(record *LlmRecord, value any) {
	body, ok := value.(map[string]any)
	if !ok {
		return
	}
	record.SystemBytes = len(llmSystemText(body))
	if messages, ok := body["messages"].([]any); ok {
		record.MessageCount = len(messages)
	} else if input, ok := body["input"].([]any); ok {
		// The Responses API calls the conversation "input".
		record.MessageCount = len(input)
	}
	if tools, ok := body["tools"].([]any); ok {
		record.ToolCount = len(tools)
	}
}

// llmSystemText is the system prompt however the API spells it: a top-level
// field for Anthropic, the first messages for OpenAI.
func llmSystemText(body map[string]any) string {
	if text := llmMessageText(body["system"]); text != "" {
		return text
	}
	// The Responses API calls it "instructions".
	if text := llmMessageText(body["instructions"]); text != "" {
		return text
	}

	messages, _ := body["messages"].([]any)
	parts := []string{}
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if role, _ := message["role"].(string); role != "system" && role != "developer" {
			continue
		}
		parts = append(parts, llmMessageText(message["content"]))
	}
	return strings.Join(parts, "\n")
}

// encodeLlmBody serializes a body within the limit, capping its strings a step
// at a time until it fits. It reports whether anything was cut.
func encodeLlmBody(value any, maximum int) (string, bool) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	if len(encoded) <= maximum {
		return string(encoded), false
	}

	for _, cap := range llmStringCaps {
		encoded, err = json.Marshal(boundStrings(value, cap))
		if err != nil {
			return "", true
		}
		if len(encoded) <= maximum {
			return string(encoded), true
		}
	}
	return auditutil.EncodeBoundedJSON(value, maximum), true
}

// boundStrings caps every string in a decoded body, leaving its structure alone.
func boundStrings(value any, maximum int) any {
	switch typed := value.(type) {
	case string:
		return auditutil.BoundString(typed, maximum)
	case []any:
		bounded := make([]any, len(typed))
		for i, item := range typed {
			bounded[i] = boundStrings(item, maximum)
		}
		return bounded
	case map[string]any:
		bounded := make(map[string]any, len(typed))
		for key, item := range typed {
			bounded[key] = boundStrings(item, maximum)
		}
		return bounded
	}
	return value
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
	if filter.Provider != "" {
		session = session.And("provider like ?", "%"+filter.Provider+"%")
	}
	if filter.Agent != "" {
		session = session.And("agent = ?", filter.Agent)
	}
	if filter.ClientIp != "" {
		session = session.And("client_ip = ?", filter.ClientIp)
	}
	if filter.Since != "" {
		session = session.And("created_time >= ?", filter.Since)
	}
	switch filter.Outcome {
	case "ok":
		session = session.And("status >= 200").And("status < 300")
	case "error":
		// Parenthesized because it sits next to the AND clauses above.
		session = session.And("(status = 0 or status < 200 or status >= 300)")
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
	err = session.Omit("payload", "error_body").Desc("id").Limit(limit, offset).Find(&records)
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

// GetLlmRecordStats totals the same records GetLlmRecords lists.
func GetLlmRecordStats(filter LlmRecordFilter, topModels int) (*LlmRecordStats, error) {
	countSession := llmRecordSession(filter)
	defer countSession.Close()
	requests, err := countSession.Count(&LlmRecord{})
	if err != nil {
		return nil, err
	}

	failedSession := llmRecordSession(filter)
	defer failedSession.Close()
	failed, err := failedSession.And("(status = 0 or status < 200 or status >= 300)").Count(&LlmRecord{})
	if err != nil {
		return nil, err
	}

	sumSession := llmRecordSession(filter)
	defer sumSession.Close()
	sums, err := sumSession.SumsInt(&LlmRecord{},
		"prompt_tokens", "completion_tokens", "cache_read_tokens", "cache_write_tokens", "total_tokens")
	if err != nil {
		return nil, err
	}

	costSession := llmRecordSession(filter)
	defer costSession.Close()
	cost, err := costSession.Sum(&LlmRecord{}, "cost")
	if err != nil {
		return nil, err
	}

	unpricedSession := llmRecordSession(filter)
	defer unpricedSession.Close()
	unpriced, err := unpricedSession.And("priced = ?", false).And("total_tokens > 0").Count(&LlmRecord{})
	if err != nil {
		return nil, err
	}

	stats := &LlmRecordStats{
		Requests:         requests,
		Failed:           failed,
		PromptTokens:     sums[0],
		CompletionTokens: sums[1],
		CacheReadTokens:  sums[2],
		CacheWriteTokens: sums[3],
		TotalTokens:      sums[4],
		Cost:             cost,
		Unpriced:         unpriced,
		Models:           []LlmModelStat{},
		Providers:        []LlmProviderStat{},
	}

	modelSession := llmRecordSession(filter)
	defer modelSession.Close()
	err = modelSession.Table("llm_record").
		Select("model as model, COUNT(*) as requests, SUM(total_tokens) as tokens, SUM(cost) as cost").
		GroupBy("model").
		Desc("requests").
		Limit(topModels).
		Find(&stats.Models)
	if err != nil {
		return nil, err
	}

	providerSession := llmRecordSession(filter)
	defer providerSession.Close()
	err = providerSession.Table("llm_record").
		Select("provider as provider, COUNT(*) as requests, " +
			"SUM(CASE WHEN status >= 200 AND status < 300 THEN 0 ELSE 1 END) as failed, " +
			"SUM(total_tokens) as tokens, SUM(cost) as cost").
		Where("provider <> ''").
		GroupBy("provider").
		Desc("requests").
		Limit(topModels).
		Find(&stats.Providers)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// GetLlmAgentStats totals every agent at once, so a page showing them side by
// side asks once rather than once per agent.
func GetLlmAgentStats(filter LlmRecordFilter) ([]*LlmAgentStat, error) {
	session := llmRecordSession(filter)
	defer session.Close()
	stats := []*LlmAgentStat{}
	err := session.Table("llm_record").
		Select("agent as agent, COUNT(*) as requests, " +
			"SUM(CASE WHEN status >= 200 AND status < 300 THEN 0 ELSE 1 END) as failed, " +
			"SUM(total_tokens) as tokens, SUM(cost) as cost").
		Where("agent <> ''").
		GroupBy("agent").
		Desc("requests").
		Find(&stats)
	if err != nil {
		return nil, err
	}

	// The newest record of each agent carries the model and provider that agent
	// is on right now, which no total can say.
	lastSession := llmRecordSession(filter)
	defer lastSession.Close()
	last := []*LlmAgentStat{}
	err = lastSession.Table("llm_record").
		Select("agent as agent, model as last_model, provider as last_provider, created_time as last_time").
		Where("id in (select max(id) from llm_record where agent <> '' group by agent)").
		Find(&last)
	if err != nil {
		return nil, err
	}

	for _, stat := range stats {
		for _, item := range last {
			if item.Agent == stat.Agent {
				stat.LastModel = item.LastModel
				stat.LastProvider = item.LastProvider
				stat.LastTime = item.LastTime
				break
			}
		}
	}
	return stats, nil
}

// llmSubscriberBuffer is how far a live watcher may fall behind before it
// starts losing records.
const llmSubscriberBuffer = 64

// llmRecordHub fans finished records out to the dashboards watching live. A
// watcher that cannot keep up loses records rather than holding the writer up.
type llmRecordHub struct {
	mutex       sync.Mutex
	subscribers map[int64]chan *LlmRecord
	nextId      int64
}

var llmHub = llmRecordHub{subscribers: map[int64]chan *LlmRecord{}}

// SubscribeLlmRecords opens a live feed of records as they are written. The
// caller must pass the id back to UnsubscribeLlmRecords when it is done.
func SubscribeLlmRecords() (int64, <-chan *LlmRecord) {
	llmHub.mutex.Lock()
	defer llmHub.mutex.Unlock()

	llmHub.nextId++
	id := llmHub.nextId
	feed := make(chan *LlmRecord, llmSubscriberBuffer)
	llmHub.subscribers[id] = feed
	return id, feed
}

func UnsubscribeLlmRecords(id int64) {
	llmHub.mutex.Lock()
	defer llmHub.mutex.Unlock()

	if feed, ok := llmHub.subscribers[id]; ok {
		delete(llmHub.subscribers, id)
		close(feed)
	}
}

// publishLlmRecord hands a record to every live feed, without its body: the
// feed keeps a table up to date, and a row is opened one at a time.
func publishLlmRecord(record *LlmRecord) {
	llmHub.mutex.Lock()
	defer llmHub.mutex.Unlock()

	if len(llmHub.subscribers) == 0 {
		return
	}
	summary := *record
	summary.Payload = ""
	summary.ErrorBody = ""
	for _, feed := range llmHub.subscribers {
		select {
		case feed <- &summary:
		default:
		}
	}
}
