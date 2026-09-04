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

package agenthistory

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

// UsageBucket totals what one session spent on one model on one day. The day is
// what a trend is drawn from, and keeping it here means the per-file cache holds
// the answer instead of the turns it was added up from.
type UsageBucket struct {
	Model            string `json:"model"`
	Day              string `json:"day"`
	Requests         int    `json:"requests"`
	PromptTokens     int    `json:"promptTokens"`
	CompletionTokens int    `json:"completionTokens"`
	CacheReadTokens  int    `json:"cacheReadTokens"`
	CacheWriteTokens int    `json:"cacheWriteTokens"`
	ReasoningTokens  int    `json:"reasoningTokens"`
	// LongCacheTokens is the share of CacheWriteTokens written to live an hour
	// rather than five minutes, which costs more. It is part of that count, not
	// an amount on top of it.
	LongCacheTokens int `json:"longCacheTokens"`
}

// turn is one model call, before it is folded into a bucket.
type turn struct {
	model      string
	day        string
	prompt     int
	completion int
	cacheRead  int
	cacheWrite int
	longCache  int
	reasoning  int
}

// usageReader accumulates the turns of one transcript while its lines are read.
type usageReader struct {
	turns map[string]turn
	// model is what the Codex turn in progress runs on: its token_count events
	// do not name a model, the turn_context ahead of them does.
	model string
	next  int
}

func newUsageReader() *usageReader {
	return &usageReader{turns: map[string]turn{}}
}

// add reads one line for what it spent. Both formats report usage, in different
// places: Claude Code hangs it off the assistant message, Codex emits a
// token_count event after every turn.
func (reader *usageReader) add(entry line) {
	switch {
	case entry.Type == "turn_context" && entry.Payload.Model != "":
		reader.model = entry.Payload.Model
	case entry.Type == "assistant":
		reader.addClaude(entry)
	case entry.Type == "event_msg" && entry.Payload.Type == "token_count":
		reader.addCodex(entry)
	case entry.Type == "gemini" && entry.Tokens != nil:
		reader.addGemini(entry)
	}
}

// addGemini reads what one Gemini CLI turn spent. The counts are on the model's
// own message, and thoughts are the reasoning it was billed for.
func (reader *usageReader) addGemini(entry line) {
	tokens := entry.Tokens
	if tokens.Input == 0 && tokens.Output == 0 && tokens.Cached == 0 && tokens.Thoughts == 0 {
		return
	}
	reader.put(entry.RequestId, turn{
		model:      entry.Model,
		day:        dayOf(firstNonEmpty(entry.Timestamp, entry.StartTime)),
		prompt:     tokens.Input,
		completion: tokens.Output,
		cacheRead:  tokens.Cached,
		reasoning:  tokens.Thoughts,
	})
}

func (reader *usageReader) addClaude(entry line) {
	var message struct {
		Id    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens    int `json:"input_tokens"`
			OutputTokens   int `json:"output_tokens"`
			CacheCreation  int `json:"cache_creation_input_tokens"`
			CacheReadInput int `json:"cache_read_input_tokens"`
			// The breakdown says how long each part of the cache was written
			// for, which is what separates the 1.25x rate from the 2x one.
			CacheCreationBy struct {
				OneHour int `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(entry.Message, &message); err != nil {
		return
	}

	usage := message.Usage
	if usage.InputTokens == 0 && usage.OutputTokens == 0 && usage.CacheCreation == 0 && usage.CacheReadInput == 0 {
		return
	}
	reader.put(firstNonEmpty(entry.RequestId, message.Id), turn{
		model:      message.Model,
		day:        dayOf(entry.Timestamp),
		prompt:     usage.InputTokens,
		completion: usage.OutputTokens,
		cacheRead:  usage.CacheReadInput,
		cacheWrite: usage.CacheCreation,
		longCache:  usage.CacheCreationBy.OneHour,
	})
}

func (reader *usageReader) addCodex(entry line) {
	var info struct {
		// Codex reports both the running total and the turn that just ended.
		// The turn is what is added up here, so a resumed session that restarts
		// its total does not count everything before it a second time.
		Last struct {
			InputTokens     int `json:"input_tokens"`
			CachedInput     int `json:"cached_input_tokens"`
			OutputTokens    int `json:"output_tokens"`
			ReasoningOutput int `json:"reasoning_output_tokens"`
		} `json:"last_token_usage"`
	}
	if err := json.Unmarshal(entry.Payload.Info, &info); err != nil {
		return
	}

	last := info.Last
	// The event fired before the first answer carries no counts at all.
	if last.InputTokens == 0 && last.OutputTokens == 0 {
		return
	}
	// Codex counts the cached part inside input_tokens, while every other
	// counter here is of what was billed fresh, so it comes back out.
	prompt := last.InputTokens - last.CachedInput
	if prompt < 0 {
		prompt = 0
	}
	reader.put("", turn{
		model:      reader.model,
		day:        dayOf(entry.Timestamp),
		prompt:     prompt,
		completion: last.OutputTokens,
		cacheRead:  last.CachedInput,
		reasoning:  last.ReasoningOutput,
	})
}

// put records one turn under the id of the request that produced it. A second
// line for the same request replaces the first rather than adding to it: Claude
// Code rewrites the assistant message as it streams, and every copy carries the
// whole turn's usage.
func (reader *usageReader) put(id string, value turn) {
	if id == "" {
		reader.next++
		id = "#" + strconv.Itoa(reader.next)
	}
	reader.turns[id] = value
}

// buckets folds the turns into one entry per model and day. A turn the
// transcript gave no timestamp is dated by the session it belongs to.
func (reader *usageReader) buckets(fallbackDay string) []UsageBucket {
	if len(reader.turns) == 0 {
		return nil
	}

	grouped := map[string]*UsageBucket{}
	for _, value := range reader.turns {
		day := firstNonEmpty(value.day, fallbackDay)
		bucket, found := grouped[value.model+"\x00"+day]
		if !found {
			bucket = &UsageBucket{Model: value.model, Day: day}
			grouped[value.model+"\x00"+day] = bucket
		}
		bucket.Requests++
		bucket.PromptTokens += value.prompt
		bucket.CompletionTokens += value.completion
		bucket.CacheReadTokens += value.cacheRead
		bucket.CacheWriteTokens += value.cacheWrite
		bucket.LongCacheTokens += value.longCache
		bucket.ReasoningTokens += value.reasoning
	}

	buckets := make([]UsageBucket, 0, len(grouped))
	for _, bucket := range grouped {
		buckets = append(buckets, *bucket)
	}
	// Sorted so that the value held in the cache does not shuffle between reads.
	sort.SliceStable(buckets, func(left, right int) bool {
		if buckets[left].Day != buckets[right].Day {
			return buckets[left].Day < buckets[right].Day
		}
		return buckets[left].Model < buckets[right].Model
	})
	return buckets
}

// dayOf is the local calendar day a turn happened on, which is what a daily
// total means to whoever reads it.
func dayOf(timestamp string) string {
	when, err := time.Parse(time.RFC3339, strings.TrimSpace(timestamp))
	if err != nil {
		return ""
	}
	return when.Local().Format(time.DateOnly)
}
