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

package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/protocol"
)

// usageTailBytes is how much of the end of a response is kept to read the token
// usage out of. Both APIs report it last, so the tail is enough.
const usageTailBytes = 16 * 1024

// llmErrorBodyBytes bounds the upstream failure kept on a record, and
// llmErrorMessageBytes the one-line reason shown beside it.
const (
	llmErrorBodyBytes    = 8 * 1024
	llmErrorMessageBytes = 500
)

// finishLlmRecord queues the record of one relayed request. Every path out of
// forwardToProviders ends the client request, so this is the only place it is
// called from.
func (c *ApiController) finishLlmRecord(route *proxyRoute) {
	if route.record == nil {
		return
	}

	record := route.record
	route.record = nil
	record.DurationMs = time.Since(route.start).Milliseconds()
	object.AddLlmRecord(record, route.body)
}

func (route *proxyRoute) recordAttempt(providerId string) {
	if route.record == nil {
		return
	}
	route.record.Attempts++
	route.record.Provider = providerId
}

func (route *proxyRoute) recordOutcome(status int, message string) {
	if route.record == nil {
		return
	}
	route.record.Status = status
	route.record.Error = auditutil.BoundString(message, llmErrorMessageBytes)
}

// recordErrorBody keeps what the upstream answered a failed request with. It is
// the only place the record learns why a status code was what it was, so it
// runs whichever mode recording is in.
func (route *proxyRoute) recordErrorBody(raw []byte) {
	if route.record == nil || len(raw) == 0 {
		return
	}

	route.record.ErrorBody = auditutil.SanitizeJSON(string(raw), llmErrorBodyBytes)
	if route.record.Error == "" {
		if _, message := protocol.ReadError(raw, ""); message != "" {
			route.record.Error = auditutil.BoundString(message, llmErrorMessageBytes)
		}
	}
}

// recordFailure keeps one attempt the chain failed over from.
func (route *proxyRoute) recordFailure(providerId string, status int, message string) {
	if route.record == nil {
		return
	}
	route.record.Failures = append(route.record.Failures, object.LlmFailure{
		Provider: providerId,
		Status:   status,
		Error:    auditutil.BoundString(message, llmErrorMessageBytes),
	})
}

func (route *proxyRoute) recordUsage(tail []byte) {
	if route.record == nil {
		return
	}

	usage := readUsage(tail)
	record := route.record
	record.CompletionTokens = higher(usage.CompletionTokens, usage.OutputTokens)
	record.ReasoningTokens = usage.ReasoningTokens
	record.CacheWriteTokens = usage.CacheWriteTokens
	record.CacheReadTokens = higher(usage.CacheReadInputTokens, usage.CachedTokens)

	// OpenAI counts cached input inside its prompt total, Anthropic reports it
	// beside one. Only the OpenAI spellings fill CachedTokens, so taking it out
	// where that is set leaves the four counters disjoint, each priced at its
	// own rate.
	record.PromptTokens = higher(usage.PromptTokens, usage.InputTokens)
	if usage.CachedTokens > 0 {
		record.PromptTokens = higher(record.PromptTokens-usage.CachedTokens, 0)
	}

	record.TotalTokens = record.PromptTokens + record.CompletionTokens + record.CacheReadTokens + record.CacheWriteTokens
	if record.TotalTokens == 0 {
		record.TotalTokens = usage.TotalTokens
	}
}

// usageTap keeps the tail of a relayed response. It never changes a byte of
// what the client receives, and never holds more than usageTailBytes.
type usageTap struct {
	reader io.Reader
	tail   []byte
}

func (tap *usageTap) Read(p []byte) (int, error) {
	n, err := tap.reader.Read(p)
	if n > 0 {
		tap.keep(p[:n])
	}
	return n, err
}

func (tap *usageTap) keep(chunk []byte) {
	if len(chunk) >= usageTailBytes {
		tap.tail = append(tap.tail[:0], chunk[len(chunk)-usageTailBytes:]...)
		return
	}
	if len(tap.tail)+len(chunk) > usageTailBytes {
		kept := usageTailBytes - len(chunk)
		copy(tap.tail, tap.tail[len(tap.tail)-kept:])
		tap.tail = tap.tail[:kept]
	}
	tap.tail = append(tap.tail, chunk...)
}

// llmUsage covers both spellings of the same counters: OpenAI nests the cached
// and reasoning parts in detail objects, Anthropic reports them flat.
type llmUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`

	CacheWriteTokens     int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens int `json:"cache_read_input_tokens"`
	// CacheCreation is what cache_creation_input_tokens totals, so it is only
	// read when that field is absent.
	CacheCreation struct {
		Ephemeral5m int `json:"ephemeral_5m_input_tokens"`
		Ephemeral1h int `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	// The Responses API spells the same two details.
	InputTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`

	// Flattened out of the detail objects above by normalize.
	CachedTokens    int `json:"-"`
	ReasoningTokens int `json:"-"`
}

// normalize lifts the nested counters up onto one flat set of fields.
func (usage *llmUsage) normalize() {
	usage.CachedTokens = higher(usage.PromptTokensDetails.CachedTokens, usage.InputTokensDetails.CachedTokens)
	usage.ReasoningTokens = higher(usage.CompletionTokensDetails.ReasoningTokens, usage.OutputTokensDetails.ReasoningTokens)
	if usage.CacheWriteTokens == 0 {
		usage.CacheWriteTokens = usage.CacheCreation.Ephemeral5m + usage.CacheCreation.Ephemeral1h
	}
}

func (usage *llmUsage) merge(other llmUsage) {
	usage.PromptTokens = higher(usage.PromptTokens, other.PromptTokens)
	usage.CompletionTokens = higher(usage.CompletionTokens, other.CompletionTokens)
	usage.TotalTokens = higher(usage.TotalTokens, other.TotalTokens)
	usage.InputTokens = higher(usage.InputTokens, other.InputTokens)
	usage.OutputTokens = higher(usage.OutputTokens, other.OutputTokens)
	usage.CacheWriteTokens = higher(usage.CacheWriteTokens, other.CacheWriteTokens)
	usage.CacheReadInputTokens = higher(usage.CacheReadInputTokens, other.CacheReadInputTokens)
	usage.CachedTokens = higher(usage.CachedTokens, other.CachedTokens)
	usage.ReasoningTokens = higher(usage.ReasoningTokens, other.ReasoningTokens)
}

// readUsage merges every usage object in the tail: a stream splits the counters
// across several events, and a plain response carries them once.
func readUsage(tail []byte) llmUsage {
	usage := llmUsage{}
	marker := []byte(`"usage"`)
	for rest := tail; ; {
		index := bytes.Index(rest, marker)
		if index < 0 {
			return usage
		}
		rest = rest[index+len(marker):]

		var parsed llmUsage
		if value := balancedObject(rest); value != nil && json.Unmarshal(value, &parsed) == nil {
			parsed.normalize()
			usage.merge(parsed)
		}
	}
}

// balancedObject returns the JSON object that directly follows a key, or nil
// when the value is not one. Usage objects hold only numbers and objects of
// numbers, so counting braces cannot be thrown off by one inside a string.
func balancedObject(data []byte) []byte {
	start := 0
	for start < len(data) && (data[start] == ':' || data[start] == ' ' || data[start] == '\t' || data[start] == '\r' || data[start] == '\n') {
		start++
	}
	if start >= len(data) || data[start] != '{' {
		return nil
	}

	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	return nil
}

func higher(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
