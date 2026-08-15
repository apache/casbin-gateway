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

package agentmonitor

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// RecordCapacity is the fixed live diagnostic window retained by Gateway.
const RecordCapacity = 1000

// RecordQuery narrows the live diagnostic window. Empty fields match all
// values and a non-positive Limit returns every matching record.
type RecordQuery struct {
	Agent     string
	EventType string
	Outcome   string
	Session   string
	Limit     int
}

// SessionSummary is the session-level view derived from the live records.
type SessionSummary struct {
	SessionKey  string `json:"sessionKey"`
	Agent       string `json:"agent"`
	Title       string `json:"title"`
	RecordCount int    `json:"recordCount"`
	FirstTime   string `json:"firstTime"`
	LastTime    string `json:"lastTime"`
}

type storedRecord struct {
	record   Record
	sequence uint64
}

// Store keeps a bounded, thread-safe window of recent monitoring records.
type Store struct {
	mutex    sync.RWMutex
	entries  []storedRecord
	next     int
	count    int
	sequence uint64
}

var recordStore = newStore(RecordCapacity)

func newStore(capacity int) *Store {
	return &Store{entries: make([]storedRecord, capacity)}
}

// AddRecord appends one event to Gateway's process-local monitoring window.
func AddRecord(record *Record) {
	recordStore.Add(record)
}

// Add normalizes and appends one record, replacing the oldest record once the
// capacity is reached.
func (store *Store) Add(record *Record) {
	value := *record
	normalizeRecord(&value)

	store.mutex.Lock()
	store.sequence++
	store.entries[store.next] = storedRecord{record: value, sequence: store.sequence}
	store.next = (store.next + 1) % len(store.entries)
	if store.count < len(store.entries) {
		store.count++
	}
	store.mutex.Unlock()
}

// ListRecords returns matching records ordered by their reported time, newest
// first. Ties preserve their arrival order.
func ListRecords(query RecordQuery) []Record {
	return recordStore.List(query)
}

// List returns matching records ordered by reported time, newest first.
func (store *Store) List(query RecordQuery) []Record {
	store.mutex.RLock()
	matches := make([]storedRecord, 0, store.count)
	for _, entry := range store.snapshotLocked() {
		if matchesRecord(entry.record, query) {
			matches = append(matches, entry)
		}
	}
	store.mutex.RUnlock()

	sort.SliceStable(matches, func(left, right int) bool {
		leftTime, rightTime := recordTime(matches[left].record), recordTime(matches[right].record)
		if leftTime.Equal(rightTime) {
			return matches[left].sequence > matches[right].sequence
		}
		return leftTime.After(rightTime)
	})
	if query.Limit > 0 && len(matches) > query.Limit {
		matches = matches[:query.Limit]
	}
	result := make([]Record, len(matches))
	for index, entry := range matches {
		result[index] = entry.record
	}
	return result
}

// ListSessions returns session summaries newest first.
func ListSessions(query RecordQuery) []SessionSummary {
	return recordStore.Sessions(query)
}

// Sessions groups matching records by their session key.
func (store *Store) Sessions(query RecordQuery) []SessionSummary {
	query.Limit = 0
	records := store.List(query)
	groups := map[string][]Record{}
	order := make([]string, 0)
	for _, record := range records {
		if record.SessionKey == "" {
			continue
		}
		key := record.Agent + "\x00" + record.SessionKey
		if _, found := groups[key]; !found {
			order = append(order, key)
		}
		groups[key] = append(groups[key], record)
	}

	result := make([]SessionSummary, 0, len(order))
	for _, key := range order {
		group := groups[key]
		newest, oldest := group[0], group[len(group)-1]
		result = append(result, SessionSummary{
			SessionKey:  newest.SessionKey,
			Agent:       newest.Agent,
			Title:       sessionTitle(group),
			RecordCount: len(group),
			FirstTime:   oldest.CreatedTime,
			LastTime:    newest.CreatedTime,
		})
	}
	return result
}

func (store *Store) snapshotLocked() []storedRecord {
	result := make([]storedRecord, 0, store.count)
	start := 0
	if store.count == len(store.entries) {
		start = store.next
	}
	for index := 0; index < store.count; index++ {
		result = append(result, store.entries[(start+index)%len(store.entries)])
	}
	return result
}

func matchesRecord(record Record, query RecordQuery) bool {
	return matchesFilter(record.Agent, query.Agent) &&
		matchesFilter(record.EventType, query.EventType) &&
		matchesFilter(record.Outcome, query.Outcome) &&
		matchesFilter(record.SessionKey, query.Session)
}

func matchesFilter(value, filter string) bool {
	return filter == "" || strings.EqualFold(value, filter)
}

func recordTime(record Record) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, record.CreatedTime)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func sessionTitle(newestFirst []Record) string {
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
