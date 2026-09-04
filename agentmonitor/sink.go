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

import "sync"

// RecordSink is where observed behaviour is kept. Storage lives outside this
// package so that tailing an agent's logs does not depend on the database.
type RecordSink func(*Record)

var recordSink struct {
	sync.RWMutex
	sink RecordSink
}

// SetRecordSink installs the store every record is written to. A record
// observed before one is installed is dropped.
func SetRecordSink(sink RecordSink) {
	recordSink.Lock()
	recordSink.sink = sink
	recordSink.Unlock()
}

// AddRecord normalizes one observed behaviour and hands it to the sink.
func AddRecord(record *Record) {
	value := *record
	normalizeRecord(&value)

	recordSink.RLock()
	sink := recordSink.sink
	recordSink.RUnlock()
	if sink != nil {
		sink(&value)
	}
}
