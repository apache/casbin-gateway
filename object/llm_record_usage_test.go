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
	"testing"
	"time"

	"github.com/apache/casbin-gateway/util"
)

func insertUsageRecord(t *testing.T, model, channel, agent string, total int) {
	t.Helper()
	record := &LlmRecord{
		CreatedTime:      util.GetCurrentTime(),
		Model:            model,
		Channel:          channel,
		Agent:            agent,
		Status:           200,
		PromptTokens:     total / 2,
		CompletionTokens: total - total/2,
		TotalTokens:      total,
	}
	if _, err := ormer.Engine.Insert(record); err != nil {
		t.Fatal(err)
	}
}

func TestLlmUsageAggregation(t *testing.T) {
	initSqliteOrmer(t)

	insertUsageRecord(t, "gpt-5", "admin/openai", "codex", 100)
	insertUsageRecord(t, "gpt-5", "admin/openai", "codex", 50)
	insertUsageRecord(t, "claude", "admin/anthropic", "", 30) // model-routed: no agent

	since := time.Now().Add(-24 * time.Hour)
	filter := LlmRecordFilter{}

	totals, err := GetLlmUsageTotals(filter, since)
	if err != nil {
		t.Fatal(err)
	}
	if totals.Requests != 3 {
		t.Errorf("requests = %d, want 3", totals.Requests)
	}
	if totals.TotalTokens != 180 {
		t.Errorf("totalTokens = %d, want 180", totals.TotalTokens)
	}
	if totals.PromptTokens+totals.CompletionTokens != totals.TotalTokens {
		t.Errorf("prompt+completion (%d+%d) != total (%d)", totals.PromptTokens, totals.CompletionTokens, totals.TotalTokens)
	}

	byModel, err := GetLlmTokensByDimension(filter, since, "model", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(byModel) != 2 || byModel[0].Data != "gpt-5" || byModel[0].Count != 150 {
		t.Errorf("byModel = %+v, want gpt-5=150 first", byModel)
	}

	// The model-routed record has no agent, so it must not form an empty bucket.
	byAgent, err := GetLlmTokensByDimension(filter, since, "agent", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(byAgent) != 1 || byAgent[0].Data != "codex" || byAgent[0].Count != 150 {
		t.Errorf("byAgent = %+v, want only codex=150", byAgent)
	}

	overTime, err := GetLlmTokensOverTime(filter, since, "day")
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, point := range overTime {
		sum += point.Count
	}
	if sum != 180 {
		t.Errorf("tokens over time sum = %d, want 180", sum)
	}

	if _, err := GetLlmTokensByDimension(filter, since, "clientIp", 8); err == nil {
		t.Error("expected an error for a non-whitelisted dimension")
	}
}
