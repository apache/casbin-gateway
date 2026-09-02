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

// The checks themselves. Each one asks a question the vendor's own API
// documents an answer to, so a finding can be argued with rather than believed.

package object

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
)

// ---------------------------------------------------------------------------
// One: a forced tool call, which also carries the identity and header checks
// ---------------------------------------------------------------------------

const probeToolName = "record_station_reading"

const probeToolSystem = "You are answering a capability probe. Call the tool exactly once with plausible values."

const probeToolPrompt = "Station BRAVO-7 is tagged coastal and tidal. It reported 12.5 degrees Celsius " +
	"and the sensor was working. Record that reading."

// probeToolSchema is nested and has required fields at two levels. A model that
// is not the one being sold usually flattens it or drops the array.
func probeToolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"station": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   map[string]any{"type": "string"},
					"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				},
				"required": []string{"id", "tags"},
			},
			"samples": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"celsius": map[string]any{"type": "number"},
						"ok":      map[string]any{"type": "boolean"},
					},
					"required": []string{"celsius", "ok"},
				},
			},
		},
		"required": []string{"station", "samples"},
	}
}

func probeToolBody(upstream string, model string) map[string]any {
	schema := probeToolSchema()
	if upstream == ProtocolAnthropic {
		return map[string]any{
			"model":      model,
			"max_tokens": probeToolMaxTokens,
			"system":     probeToolSystem,
			"messages":   []any{map[string]any{"role": "user", "content": probeToolPrompt}},
			"tools": []any{map[string]any{
				"name":         probeToolName,
				"description":  "Record one temperature reading for a station.",
				"input_schema": schema,
			}},
			"tool_choice": map[string]any{"type": "tool", "name": probeToolName},
		}
	}

	return map[string]any{
		"model":      model,
		"max_tokens": probeToolMaxTokens,
		"messages": []any{
			map[string]any{"role": "system", "content": probeToolSystem},
			map[string]any{"role": "user", "content": probeToolPrompt},
		},
		"tools": []any{map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        probeToolName,
				"description": "Record one temperature reading for a station.",
				"parameters":  schema,
			},
		}},
		"tool_choice": map[string]any{
			"type":     "function",
			"function": map[string]any{"name": probeToolName},
		},
	}
}

// probeToolCall is the first request. It decides whether the rest of the suite
// runs at all, and answers three questions at once: what the upstream says it
// is, what its headers look like, and whether it can hold a nested schema.
func probeToolCall(provider *Provider, model string, probe *ProviderProbe) *probeBody {
	upstream := ProviderProtocol(provider)
	answer, err := sendProbe(provider, probeToolBody(upstream, model), false)
	if err != nil {
		probe.Error = auditutil.BoundString(err.Error(), probeErrorChars)
		return nil
	}
	if !answer.ok() {
		probe.Error = answer.failure()
		return nil
	}

	parsed := probeBody{}
	if err := json.Unmarshal(answer.body, &parsed); err != nil {
		probe.Error = "the upstream did not answer with a completion this API defines"
		return nil
	}

	probe.addUsage(parsed.Usage)
	probe.UpstreamModel = parsed.Model
	probe.VendorHeaders = vendorHeadersOf(upstream, answer.header)

	probe.addCheck(probeIdentityCheck(model, parsed.Model))
	probe.addCheck(probeVendorCheck(probe.VendorHeaders))
	probe.addCheck(probeToolsCheck(upstream, &parsed))
	return &parsed
}

// probeIdentityCheck compares the model name that came back with the one that
// was asked for. Matching proves little on its own — echoing a name back is the
// easiest thing in the world to do — but not matching is decisive.
func probeIdentityCheck(asked string, answered string) ProbeCheck {
	check := ProbeCheck{Key: ProbeIdentity, Level: LlmAuditUnknown, Facts: []string{}}
	if strings.TrimSpace(answered) == "" {
		return check
	}
	check.Facts = []string{answered}
	if sameModelName(asked, answered) {
		check.Level = LlmAuditOk
		return check
	}
	check.Level = LlmAuditAlert
	return check
}

// sameModelName treats a name that only adds a version or date suffix as the
// same model, which is what a vendor answering "claude-opus-4-5-20251101" to a
// request for "claude-opus-4-5" is doing.
func sameModelName(asked string, answered string) bool {
	left := strings.ToLower(strings.TrimSpace(asked))
	right := strings.ToLower(strings.TrimSpace(answered))
	if left == right {
		return true
	}
	// An aggregator prefixes the vendor, e.g. "anthropic/claude-opus-4-5".
	if index := strings.LastIndex(left, "/"); index >= 0 {
		left = left[index+1:]
	}
	if index := strings.LastIndex(right, "/"); index >= 0 {
		right = right[index+1:]
	}
	return strings.HasPrefix(left, right) || strings.HasPrefix(right, left)
}

// The response headers each vendor's own API sets. A reseller in front of the
// real thing usually passes some of them through; a backend that never talked
// to the vendor has none to pass.
var probeVendorHeaders = map[string][]string{
	ProtocolAnthropic: {
		"Request-Id",
		"Anthropic-Organization-Id",
		"Anthropic-Ratelimit-Requests-Limit",
		"Anthropic-Ratelimit-Input-Tokens-Limit",
		"X-Should-Retry",
	},
	ProtocolOpenAi: {
		"X-Request-Id",
		"Openai-Organization",
		"Openai-Processing-Ms",
		"Openai-Version",
		"X-Ratelimit-Limit-Requests",
	},
}

func vendorHeadersOf(upstream string, header http.Header) []string {
	found := []string{}
	for _, name := range probeVendorHeaders[upstream] {
		if header.Get(name) != "" {
			found = append(found, name)
		}
	}
	return found
}

// probeVendorCheck never reaches "alert": a relay that strips headers is being
// tidy, not necessarily dishonest. It is a corroborating signal, and the page
// words it as one.
func probeVendorCheck(found []string) ProbeCheck {
	check := ProbeCheck{
		Key:   ProbeVendor,
		Level: LlmAuditWarn,
		Facts: found,
		Value: float64(len(found)),
	}
	if len(found) >= 2 {
		check.Level = LlmAuditOk
	}
	return check
}

// probeToolsCheck reads the forced call back. A model that cannot fill a
// two-level schema is not the frontier model it was sold as, whatever the
// response says its name is.
func probeToolsCheck(upstream string, parsed *probeBody) ProbeCheck {
	check := ProbeCheck{Key: ProbeTools, Level: LlmAuditAlert, Facts: []string{}}

	arguments := ""
	if upstream == ProtocolAnthropic {
		for _, block := range parsed.Content {
			if block.Type == "tool_use" && block.Name == probeToolName {
				arguments = string(block.Input)
			}
		}
	} else if len(parsed.Choices) > 0 {
		for _, call := range parsed.Choices[0].Message.ToolCalls {
			if call.Function.Name == probeToolName {
				arguments = call.Function.Arguments
			}
		}
	}

	if arguments == "" {
		return check
	}

	var filled struct {
		Station struct {
			Id   string   `json:"id"`
			Tags []string `json:"tags"`
		} `json:"station"`
		Samples []struct {
			Celsius *float64 `json:"celsius"`
			Ok      *bool    `json:"ok"`
		} `json:"samples"`
	}
	// Arguments that are not valid JSON and a half-filled object are the same
	// finding for whoever is paying: the model was made to call the tool and
	// could not hold the shape it was given.
	if err := json.Unmarshal([]byte(arguments), &filled); err != nil {
		check.Level = LlmAuditWarn
		return check
	}
	if filled.Station.Id == "" || len(filled.Samples) == 0 || filled.Samples[0].Celsius == nil {
		check.Level = LlmAuditWarn
		return check
	}

	check.Level = LlmAuditOk
	return check
}

// ---------------------------------------------------------------------------
// Two: the shape of the event stream
// ---------------------------------------------------------------------------

// probeAnthropicEvents is the sequence the Anthropic streaming API documents. A
// backend translating some other API into this one is where these go missing.
var probeAnthropicEvents = []string{
	"message_start", "content_block_start", "content_block_delta",
	"content_block_stop", "message_delta", "message_stop",
}

func probeStreamBody(upstream string, model string) map[string]any {
	body := map[string]any{
		"model":      model,
		"max_tokens": probeStreamMaxTokens,
		"messages":   []any{map[string]any{"role": "user", "content": "Reply with the single word: ready."}},
		"stream":     true,
	}
	if upstream != ProtocolAnthropic {
		body["stream_options"] = map[string]any{"include_usage": true}
	}
	return body
}

func probeStreamShape(provider *Provider, model string, probe *ProviderProbe) {
	upstream := ProviderProtocol(provider)
	answer, err := sendProbe(provider, probeStreamBody(upstream, model), true)
	if err != nil {
		probe.addCheck(ProbeCheck{Key: ProbeStream, Level: LlmAuditUnknown, Facts: []string{}})
		return
	}
	if !answer.ok() {
		probe.addCheck(ProbeCheck{Key: ProbeStream, Level: LlmAuditUnknown, Facts: []string{}})
		return
	}

	probe.TtftMs = answer.ttftMs
	probe.Requests++
	if upstream == ProtocolAnthropic {
		probe.addCheck(probeAnthropicStreamCheck(answer))
		return
	}
	probe.addCheck(probeOpenAiStreamCheck(answer))
}

func probeAnthropicStreamCheck(answer *probeAnswer) ProbeCheck {
	seen := map[string]bool{}
	for _, name := range answer.events {
		seen[name] = true
	}

	missing := []string{}
	for _, name := range probeAnthropicEvents {
		if !seen[name] {
			missing = append(missing, name)
		}
	}

	check := ProbeCheck{Key: ProbeStream, Level: LlmAuditOk, Facts: missing, Value: float64(len(missing))}
	switch {
	case !seen["message_start"] || !seen["message_delta"]:
		check.Level = LlmAuditAlert
		return check
	case len(missing) > 0:
		check.Level = LlmAuditWarn
		return check
	}

	// message_start carries the input count before a token has been generated.
	// A backend translating some other API has nothing to put there yet and
	// sends a zero, which is as good as a missing event.
	if probeStreamOpeningInput(answer) == 0 {
		check.Level, check.Facts = LlmAuditWarn, []string{"message_start.usage"}
	}
	return check
}

func probeStreamOpeningInput(answer *probeAnswer) int {
	for index, name := range answer.events {
		if name != "message_start" {
			continue
		}
		var parsed struct {
			Message struct {
				Usage probeUsage `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(answer.payloads[index], &parsed) == nil {
			return parsed.Message.Usage.billedInput()
		}
	}
	return 0
}

func probeOpenAiStreamCheck(answer *probeAnswer) ProbeCheck {
	chunks, finished, done, usage := 0, false, false, false
	for _, payload := range answer.payloads {
		if strings.TrimSpace(string(payload)) == "[DONE]" {
			done = true
			continue
		}
		var parsed struct {
			Choices []struct {
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *probeUsage `json:"usage"`
		}
		if json.Unmarshal(payload, &parsed) != nil {
			continue
		}
		chunks++
		if len(parsed.Choices) > 0 && parsed.Choices[0].FinishReason != nil {
			finished = true
		}
		if parsed.Usage != nil && parsed.Usage.billedInput() > 0 {
			usage = true
		}
	}

	missing := []string{}
	if chunks == 0 {
		missing = append(missing, "chat.completion.chunk")
	}
	if !finished {
		missing = append(missing, "finish_reason")
	}
	if !done {
		missing = append(missing, "[DONE]")
	}

	check := ProbeCheck{Key: ProbeStream, Level: LlmAuditOk, Facts: missing, Value: float64(len(missing))}
	if len(missing) > 0 {
		check.Level = LlmAuditAlert
		return check
	}
	if !usage {
		// Asking for it is standard; not answering it is common among the
		// OpenAI-compatible vendors, so this is a note rather than a finding.
		check.Level, check.Facts = LlmAuditWarn, []string{"usage"}
	}
	return check
}

// ---------------------------------------------------------------------------
// Three: the cache, sent twice, which also settles the billing question
// ---------------------------------------------------------------------------

// probeFillerChars is long enough to clear the largest minimum any of these
// vendors caches at, and no longer: every character of it is paid for twice.
const probeFillerChars = 9000

const probeFillerParagraph = "This paragraph is a fixed cache probe sent by Casbin Gateway on behalf of " +
	"the account that owns this key. It carries no user data and asks for no work. It exists only so that the " +
	"same request can be sent twice and the two answers compared against each other, which is the one way to " +
	"tell whether an upstream is really accounting for a prompt cache."

var (
	probeFillerOnce sync.Once
	probeFillerText string
)

// probeFiller is byte-identical on every call, which is what a cache needs: two
// requests that differ anywhere in the cached prefix are two different prompts.
func probeFiller() string {
	probeFillerOnce.Do(func() {
		builder := strings.Builder{}
		for index := 0; builder.Len() < probeFillerChars; index++ {
			fmt.Fprintf(&builder, "Section %d. %s\n", index, probeFillerParagraph)
		}
		probeFillerText = builder.String()
	})
	return probeFillerText
}

const probeCachePrompt = "Answer with the single word: ok."

func probeCacheBody(upstream string, model string) map[string]any {
	if upstream == ProtocolAnthropic {
		return map[string]any{
			"model":      model,
			"max_tokens": probeCacheMaxTokens,
			"system": []any{map[string]any{
				"type":          "text",
				"text":          probeFiller(),
				"cache_control": map[string]any{"type": "ephemeral"},
			}},
			"messages": []any{map[string]any{"role": "user", "content": probeCachePrompt}},
		}
	}

	return map[string]any{
		"model":      model,
		"max_tokens": probeCacheMaxTokens,
		"messages": []any{
			map[string]any{"role": "system", "content": probeFiller()},
			map[string]any{"role": "user", "content": probeCachePrompt},
		},
	}
}

// probeCacheGap lets a written cache become readable before it is asked for.
const probeCacheGap = 1500 * time.Millisecond

// probeCachePair sends the same long request twice. The pair answers two
// questions no single request can: whether the cache is real, and whether the
// input counter is stable — two identical requests that are billed different
// amounts of input were not counted, they were invented.
func probeCachePair(provider *Provider, model string, probe *ProviderProbe) {
	upstream := ProviderProtocol(provider)
	body := probeCacheBody(upstream, model)

	first, err := sendProbeUsage(provider, body, probe)
	if err != nil {
		probe.addCacheUnknown()
		return
	}
	time.Sleep(probeCacheGap)
	second, err := sendProbeUsage(provider, body, probe)
	if err != nil {
		probe.addCacheUnknown()
		return
	}

	probe.addCheck(probeCacheCheck(upstream, first, second))
	probe.addCheck(probeBillingCheck(first, second))
}

// sendProbeUsage sends one of the pair and returns what it reported.
func sendProbeUsage(provider *Provider, body map[string]any, probe *ProviderProbe) (probeUsage, error) {
	answer, err := sendProbe(provider, body, false)
	if err != nil {
		return probeUsage{}, err
	}
	if !answer.ok() {
		return probeUsage{}, fmt.Errorf("%s", answer.failure())
	}

	parsed := probeBody{}
	if err := json.Unmarshal(answer.body, &parsed); err != nil {
		return probeUsage{}, fmt.Errorf("the answer could not be read")
	}
	probe.addUsage(parsed.Usage)
	return parsed.Usage, nil
}

// addCacheUnknown records that the pair could not be sent, so the two checks it
// answers are shown as unmeasured rather than left off the report.
func (probe *ProviderProbe) addCacheUnknown() {
	probe.addCheck(ProbeCheck{Key: ProbeCache, Level: LlmAuditUnknown, Facts: []string{}})
	probe.addCheck(ProbeCheck{Key: ProbeBilling, Level: LlmAuditUnknown, Facts: []string{}})
}

func probeCacheCheck(upstream string, first probeUsage, second probeUsage) ProbeCheck {
	check := ProbeCheck{
		Key:   ProbeCache,
		Value: float64(second.cached()),
		Facts: []string{strconv.Itoa(first.written()), strconv.Itoa(second.cached())},
	}

	switch {
	case second.cached() > 0:
		check.Level = LlmAuditOk
	case first.written() > 0:
		// Written but never read back: the write may simply not have landed in
		// time, which is why this is not the harder finding.
		check.Level = LlmAuditWarn
	case upstream == ProtocolAnthropic:
		// This request carried cache_control, and this API documents a counter
		// for it. Nothing came back in either field.
		check.Level = LlmAuditAlert
	default:
		// The OpenAI-compatible vendors are not all expected to cache at all.
		check.Level = LlmAuditWarn
	}
	return check
}

// probeBillingEstimate is what the cache request should count as input, from
// the bytes that were actually sent. Four characters to the token is rough, so
// only a large gap is treated as a finding.
func probeBillingEstimate() int {
	return (len(probeFiller()) + len(probeCachePrompt)) / 4
}

func probeBillingCheck(first probeUsage, second probeUsage) ProbeCheck {
	billed, again := first.billedInput(), second.billedInput()
	check := ProbeCheck{Key: ProbeBilling, Level: LlmAuditUnknown, Facts: []string{}}
	if billed == 0 {
		return check
	}

	// The two requests were byte-identical, so the counts have to be too. This
	// part needs no tokenizer and no assumption.
	drift := math.Abs(float64(billed-again)) / float64(billed)
	if drift > 0.02 {
		check.Level = LlmAuditAlert
		check.Value = drift
		// The leading token says which of the two comparisons the numbers are.
		check.Facts = []string{"drift", strconv.Itoa(billed), strconv.Itoa(again)}
		return check
	}

	estimate := probeBillingEstimate()
	check.Value = float64(billed) / float64(estimate)
	check.Facts = []string{"estimate", strconv.Itoa(billed), strconv.Itoa(estimate)}
	switch {
	case check.Value >= 2.5 || check.Value <= 0.4:
		check.Level = LlmAuditAlert
	case check.Value >= 1.5 || check.Value <= 0.7:
		check.Level = LlmAuditWarn
	default:
		check.Level = LlmAuditOk
	}
	return check
}
