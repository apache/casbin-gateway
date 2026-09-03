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
//
// What each one asks is a stored test case, not a constant: the suite is on the
// Authenticity page, where it can be read, weighted differently, turned off or
// added to. The engines here are what a case runs on, and the defaults they
// fall back to are the ones the shipped cases carry.

package object

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
)

// ---------------------------------------------------------------------------
// The run: the enabled cases, in their own order
// ---------------------------------------------------------------------------

// runProbeCases sends what the suite asks for and records what came back. The
// first request decides whether the upstream is usable at all; everything after
// it is a question that would be meaningless if it were not.
func runProbeCases(provider *Provider, model string, probe *ProviderProbe, cases []*ProbeCase) {
	toolCases := probeCasesOf(cases, ProbeTools)

	var primary *probeBody
	var headers http.Header

	if len(toolCases) > 0 {
		for index, toolCase := range toolCases {
			parsed, header := probeToolCase(provider, model, probe, toolCase, index == 0)
			if index == 0 {
				if parsed == nil {
					return
				}
				primary, headers = parsed, header
			}
		}
	} else {
		// With every tool case turned off there is still a question to settle
		// before anything else is asked: whether the upstream answers at all,
		// and as what.
		primary, headers = probeOpeningCall(provider, model, probe)
		if primary == nil {
			return
		}
	}

	probe.Ok = true
	probe.UpstreamModel = primary.Model

	for _, identityCase := range probeCasesOf(cases, ProbeIdentity) {
		probe.addCheck(probeIdentityCheck(identityCase, model, primary.Model))
	}
	for _, vendorCase := range probeCasesOf(cases, ProbeVendor) {
		wanted := vendorHeadersOf(vendorCase, provider)
		if len(wanted) == 0 {
			// Nothing to hold this endpoint to: the vendor it belongs to is
			// not one whose headers this build documents, or it belongs to no
			// vendor at all. That is a gap here, not a finding about it.
			check := checkOf(vendorCase, LlmAuditUnknown)
			check.Facts = []string{probeVendorUndocumented}
			probe.addCheck(check)
			continue
		}
		found := headersPresent(wanted, headers)
		probe.VendorHeaders = mergeStrings(probe.VendorHeaders, found)
		probe.addCheck(probeVendorCheck(vendorCase, found))
	}
	for _, streamCase := range probeCasesOf(cases, ProbeStream) {
		probeStreamShape(provider, model, probe, streamCase)
	}

	probeCacheAndBilling(provider, model, probe, cases)
	sortProbeChecks(probe, cases)
}

// sortProbeChecks puts the report in the order the suite is listed in, so the
// page and the case table read the same way down.
func sortProbeChecks(probe *ProviderProbe, cases []*ProbeCase) {
	order := map[string]int{}
	for index, probeCase := range cases {
		order[probeCase.Name] = index
	}
	sort.SliceStable(probe.Checks, func(left, right int) bool {
		return order[probe.Checks[left].Case] < order[probe.Checks[right].Case]
	})
}

// checkOf is the row a case reports through, so every finding says which case
// produced it and what that case was worth.
func checkOf(probeCase *ProbeCase, level string) ProbeCheck {
	return ProbeCheck{
		Key:    probeCase.Check,
		Case:   probeCase.Name,
		Title:  probeCase.DisplayName,
		Weight: probeCase.Weight,
		Level:  level,
		Facts:  []string{},
	}
}

func mergeStrings(into []string, added []string) []string {
	seen := map[string]bool{}
	for _, value := range into {
		seen[value] = true
	}
	for _, value := range added {
		if !seen[value] {
			seen[value] = true
			into = append(into, value)
		}
	}
	return into
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// The opening call: a forced tool call, which also carries identity and headers
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

// probeCaseSchema is the schema a case asks for, or the shipped one where the
// case names none. A stored schema that will not parse is not a reason to skip
// the case: the request still goes out with the default.
func probeCaseSchema(probeCase *ProbeCase) map[string]any {
	if strings.TrimSpace(probeCase.Params.Schema) == "" {
		return probeToolSchema()
	}
	parsed := map[string]any{}
	if err := json.Unmarshal([]byte(probeCase.Params.Schema), &parsed); err != nil {
		return probeToolSchema()
	}
	return parsed
}

func probeCaseToolName(probeCase *ProbeCase) string {
	return firstString(probeCase.Params.ToolName, probeToolName)
}

func probeToolBody(upstream string, model string, probeCase *ProbeCase) map[string]any {
	schema := probeCaseSchema(probeCase)
	name := probeCaseToolName(probeCase)
	system := firstString(probeCase.Params.System, probeToolSystem)
	prompt := firstString(probeCase.Params.Prompt, probeToolPrompt)
	limit := firstInt(probeCase.Params.MaxTokens, probeToolMaxTokens)

	if upstream == ProtocolAnthropic {
		return map[string]any{
			"model":      model,
			"max_tokens": limit,
			"system":     system,
			"messages":   []any{map[string]any{"role": "user", "content": prompt}},
			"tools": []any{map[string]any{
				"name":         name,
				"description":  "Record one temperature reading for a station.",
				"input_schema": schema,
			}},
			"tool_choice": map[string]any{"type": "tool", "name": name},
		}
	}

	return map[string]any{
		"model":      model,
		"max_tokens": limit,
		"messages": []any{
			map[string]any{"role": "system", "content": system},
			map[string]any{"role": "user", "content": prompt},
		},
		"tools": []any{map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": "Record one temperature reading for a station.",
				"parameters":  schema,
			},
		}},
		"tool_choice": map[string]any{
			"type":     "function",
			"function": map[string]any{"name": name},
		},
	}
}

// probeToolCase sends one tool case. The first one is also the opening call: it
// is what says whether the upstream is usable, so its failure is the report.
func probeToolCase(
	provider *Provider,
	model string,
	probe *ProviderProbe,
	probeCase *ProbeCase,
	opening bool,
) (*probeBody, http.Header) {
	upstream := ProviderProtocol(provider)
	answer, err := sendProbe(provider, probeToolBody(upstream, model, probeCase), false)
	if err != nil || !answer.ok() {
		reason := ""
		if err != nil {
			reason = auditutil.BoundString(err.Error(), probeErrorChars)
		} else {
			reason = answer.failure()
		}
		if opening {
			probe.Error = reason
			return nil, nil
		}
		probe.addCheck(checkOf(probeCase, LlmAuditUnknown))
		return nil, nil
	}

	parsed := probeBody{}
	if err := json.Unmarshal(answer.body, &parsed); err != nil {
		if opening {
			probe.Error = "the upstream did not answer with a completion this API defines"
			return nil, nil
		}
		probe.addCheck(checkOf(probeCase, LlmAuditUnknown))
		return nil, nil
	}

	probe.addUsage(parsed.Usage)
	probe.addCheck(probeToolsCheck(probeCase, upstream, &parsed))
	return &parsed, answer.header
}

// probeOpeningCall is the request that stands in for the tool call when every
// tool case is turned off: the shortest completion this API defines, sent so
// that identity and headers still have an answer to be read out of.
func probeOpeningCall(provider *Provider, model string, probe *ProviderProbe) (*probeBody, http.Header) {
	body := map[string]any{
		"model":      model,
		"max_tokens": probeStreamMaxTokens,
		"messages":   []any{map[string]any{"role": "user", "content": probeStreamPrompt}},
	}

	answer, err := sendProbe(provider, body, false)
	if err != nil {
		probe.Error = auditutil.BoundString(err.Error(), probeErrorChars)
		return nil, nil
	}
	if !answer.ok() {
		probe.Error = answer.failure()
		return nil, nil
	}

	parsed := probeBody{}
	if err := json.Unmarshal(answer.body, &parsed); err != nil {
		probe.Error = "the upstream did not answer with a completion this API defines"
		return nil, nil
	}

	probe.addUsage(parsed.Usage)
	return &parsed, answer.header
}

// probeIdentityAlias marks an answer that matched because the name asked for is
// one the vendor documents as moving, so that the report can say which of the
// two things it found.
const probeIdentityAlias = "alias"

// probeIdentityCheck compares the model name that came back with the one that
// was asked for. Matching proves little on its own — echoing a name back is the
// easiest thing in the world to do — but not matching is decisive, except where
// the vendor documents the name asked for as an alias.
func probeIdentityCheck(probeCase *ProbeCase, asked string, answered string) ProbeCheck {
	check := checkOf(probeCase, LlmAuditUnknown)
	if strings.TrimSpace(answered) == "" {
		return check
	}
	check.Facts = []string{answered}
	if sameModelName(asked, answered) {
		check.Level = LlmAuditOk
		return check
	}

	// "deepseek-chat" is DeepSeek's own name for whichever chat model is
	// current, and it answers with the name of the model that ran. A different
	// name there is the documented behaviour; what is left to check is that it
	// is still a model of the vendor whose alias was asked for.
	if isProbeModelAlias(asked) && sameModelVendor(asked, answered) {
		check.Level = LlmAuditOk
		check.Facts = []string{answered, probeIdentityAlias}
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

// probeVendorUndocumented is what the header case reports when there is no
// documented list to hold the endpoint to.
const probeVendorUndocumented = "undocumented"

// vendorHeadersOf is the list this endpoint is held to: the one the case names,
// or failing that the response headers the vendor whose own host it is
// documents. An endpoint that is not a vendor's own — a reseller, an
// aggregator, a vendor this build carries no header list for — has no list, and
// the case is then not asked. Answering an OpenAI-compatible API is not a claim
// to be OpenAI, so OpenAI's headers are not owed by everything that speaks it.
func vendorHeadersOf(probeCase *ProbeCase, provider *Provider) []string {
	if len(probeCase.Params.Headers) > 0 {
		return probeCase.Params.Headers
	}
	if vendor := probeVendorOfProvider(provider); vendor != nil {
		return vendor.headers
	}
	return nil
}

func headersPresent(wanted []string, header http.Header) []string {
	found := []string{}
	for _, name := range wanted {
		if header.Get(name) != "" {
			found = append(found, name)
		}
	}
	return found
}

// probeVendorCheck never reaches "alert": a relay that strips headers is being
// tidy, not necessarily dishonest. It is a corroborating signal, and the page
// words it as one.
func probeVendorCheck(probeCase *ProbeCase, found []string) ProbeCheck {
	check := checkOf(probeCase, LlmAuditWarn)
	check.Facts = found
	check.Value = float64(len(found))

	minimum := firstInt(probeCase.Params.MinHeaders, 2)
	if len(found) >= minimum {
		check.Level = LlmAuditOk
	}
	return check
}

// probeToolsCheck reads the forced call back. A model that cannot fill the
// schema it was given is not the frontier model it was sold as, whatever the
// response says its name is.
func probeToolsCheck(probeCase *ProbeCase, upstream string, parsed *probeBody) ProbeCheck {
	check := checkOf(probeCase, LlmAuditAlert)
	name := probeCaseToolName(probeCase)

	arguments := ""
	if upstream == ProtocolAnthropic {
		for _, block := range parsed.Content {
			if block.Type == "tool_use" && block.Name == name {
				arguments = string(block.Input)
			}
		}
	} else if len(parsed.Choices) > 0 {
		for _, call := range parsed.Choices[0].Message.ToolCalls {
			if call.Function.Name == name {
				arguments = call.Function.Arguments
			}
		}
	}

	if arguments == "" {
		return check
	}

	// Arguments that are not valid JSON and a half-filled object are the same
	// finding for whoever is paying: the model was made to call the tool and
	// could not hold the shape it was given.
	filled := any(nil)
	if err := json.Unmarshal([]byte(arguments), &filled); err != nil {
		check.Level = LlmAuditWarn
		return check
	}
	if !schemaFilled(probeCaseSchema(probeCase), filled) {
		check.Level = LlmAuditWarn
		return check
	}

	check.Level = LlmAuditOk
	return check
}

// schemaFilled reports whether a value carries everything the schema requires,
// down through the objects and arrays it nests. It is deliberately lenient
// about what it does not understand: the finding here is a shape the model
// could not hold, not a schema violation a validator would care about.
func schemaFilled(schema map[string]any, value any) bool {
	switch schemaType(schema) {
	case "object":
		filled, ok := value.(map[string]any)
		if !ok {
			return false
		}
		properties, _ := schema["properties"].(map[string]any)
		for _, name := range schemaRequired(schema) {
			nested, present := filled[name]
			if !present || nested == nil {
				return false
			}
			if property, ok := properties[name].(map[string]any); ok {
				if !schemaFilled(property, nested) {
					return false
				}
			}
		}
		return true
	case "array":
		items, ok := value.([]any)
		if !ok || len(items) == 0 {
			return false
		}
		if item, ok := schema["items"].(map[string]any); ok {
			return schemaFilled(item, items[0])
		}
		return true
	case "string":
		text, ok := value.(string)
		return ok && strings.TrimSpace(text) != ""
	default:
		return value != nil
	}
}

func schemaType(schema map[string]any) string {
	if name, ok := schema["type"].(string); ok {
		return name
	}
	return ""
}

func schemaRequired(schema map[string]any) []string {
	names := []string{}
	switch required := schema["required"].(type) {
	case []string:
		names = append(names, required...)
	case []any:
		for _, entry := range required {
			if name, ok := entry.(string); ok {
				names = append(names, name)
			}
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// The shape of the event stream
// ---------------------------------------------------------------------------

// probeAnthropicEvents is the sequence the Anthropic streaming API documents. A
// backend translating some other API into this one is where these go missing.
var probeAnthropicEvents = []string{
	"message_start", "content_block_start", "content_block_delta",
	"content_block_stop", "message_delta", "message_stop",
}

const probeStreamPrompt = "Reply with the single word: ready."

func probeStreamBody(upstream string, model string, probeCase *ProbeCase) map[string]any {
	body := map[string]any{
		"model":      model,
		"max_tokens": firstInt(probeCase.Params.MaxTokens, probeStreamMaxTokens),
		"messages": []any{map[string]any{
			"role":    "user",
			"content": firstString(probeCase.Params.Prompt, probeStreamPrompt),
		}},
		"stream": true,
	}
	if upstream != ProtocolAnthropic {
		body["stream_options"] = map[string]any{"include_usage": true}
	}
	return body
}

func probeStreamShape(provider *Provider, model string, probe *ProviderProbe, probeCase *ProbeCase) {
	upstream := ProviderProtocol(provider)
	answer, err := sendProbe(provider, probeStreamBody(upstream, model, probeCase), true)
	if err != nil || !answer.ok() {
		probe.addCheck(checkOf(probeCase, LlmAuditUnknown))
		return
	}

	if probe.TtftMs == 0 {
		probe.TtftMs = answer.ttftMs
	}
	probe.Requests++
	if upstream == ProtocolAnthropic {
		probe.addCheck(probeAnthropicStreamCheck(probeCase, answer))
		return
	}
	probe.addCheck(probeOpenAiStreamCheck(probeCase, answer))
}

func probeAnthropicStreamCheck(probeCase *ProbeCase, answer *probeAnswer) ProbeCheck {
	seen := map[string]bool{}
	for _, name := range answer.events {
		seen[name] = true
	}

	expected := probeCase.Params.Events
	if len(expected) == 0 {
		expected = probeAnthropicEvents
	}
	missing := []string{}
	for _, name := range expected {
		if !seen[name] {
			missing = append(missing, name)
		}
	}

	check := checkOf(probeCase, LlmAuditOk)
	check.Facts = missing
	check.Value = float64(len(missing))
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

func probeOpenAiStreamCheck(probeCase *ProbeCase, answer *probeAnswer) ProbeCheck {
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

	check := checkOf(probeCase, LlmAuditOk)
	check.Facts = missing
	check.Value = float64(len(missing))
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
// The cache, sent twice, which also settles the billing question
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
func probeFiller(chars int) string {
	if chars > 0 && chars != probeFillerChars {
		return buildProbeFiller(chars)
	}
	probeFillerOnce.Do(func() { probeFillerText = buildProbeFiller(probeFillerChars) })
	return probeFillerText
}

func buildProbeFiller(chars int) string {
	builder := strings.Builder{}
	for index := 0; builder.Len() < chars; index++ {
		fmt.Fprintf(&builder, "Section %d. %s\n", index, probeFillerParagraph)
	}
	return builder.String()
}

const probeCachePrompt = "Answer with the single word: ok."

// probeCacheGap lets a written cache become readable before it is asked for.
const probeCacheGap = 1500 * time.Millisecond

func probeCacheBody(upstream string, model string, filler string, prompt string, limit int) map[string]any {
	if upstream == ProtocolAnthropic {
		return map[string]any{
			"model":      model,
			"max_tokens": limit,
			"system": []any{map[string]any{
				"type":          "text",
				"text":          filler,
				"cache_control": map[string]any{"type": "ephemeral"},
			}},
			"messages": []any{map[string]any{"role": "user", "content": prompt}},
		}
	}

	return map[string]any{
		"model":      model,
		"max_tokens": limit,
		"messages": []any{
			map[string]any{"role": "system", "content": filler},
			map[string]any{"role": "user", "content": prompt},
		},
	}
}

// probeCachePair is what one pair of identical requests reported, which is what
// both the cache case and the billing cases are read out of.
type probeCachePair struct {
	first  probeUsage
	second probeUsage
	// sentChars is how much text went out, which is what the billing case
	// compares the billed input against.
	sentChars int
	ok        bool
}

// probeCacheAndBilling sends each cache case's pair. The billing cases ride on
// the first pair rather than sending a third and fourth request: what they ask
// is about those two answers, and asking it again would cost twice for the same
// finding.
func probeCacheAndBilling(provider *Provider, model string, probe *ProviderProbe, cases []*ProbeCase) {
	cacheCases := probeCasesOf(cases, ProbeCache)
	billingCases := probeCasesOf(cases, ProbeBilling)

	pair := probeCachePair{}
	for index, cacheCase := range cacheCases {
		sent := probeCachePairOf(provider, model, probe, cacheCase)
		if index == 0 {
			pair = sent
		}
	}

	for _, billingCase := range billingCases {
		if !pair.ok {
			probe.addCheck(checkOf(billingCase, LlmAuditUnknown))
			continue
		}
		probe.addCheck(probeBillingCheck(billingCase, pair))
	}
}

// probeCachePairOf sends the same long request twice. The pair answers two
// questions no single request can: whether the cache is real, and whether the
// input counter is stable — two identical requests that are billed different
// amounts of input were not counted, they were invented.
func probeCachePairOf(
	provider *Provider,
	model string,
	probe *ProviderProbe,
	probeCase *ProbeCase,
) probeCachePair {
	upstream := ProviderProtocol(provider)
	filler := probeFiller(probeCase.Params.FillerChars)
	prompt := firstString(probeCase.Params.Prompt, probeCachePrompt)
	limit := firstInt(probeCase.Params.MaxTokens, probeCacheMaxTokens)
	gap := probeCacheGap
	if probeCase.Params.GapMs > 0 {
		gap = time.Duration(probeCase.Params.GapMs) * time.Millisecond
	}
	body := probeCacheBody(upstream, model, filler, prompt, limit)

	first, err := sendProbeUsage(provider, body, probe)
	if err != nil {
		probe.addCheck(checkOf(probeCase, LlmAuditUnknown))
		return probeCachePair{}
	}
	time.Sleep(gap)
	second, err := sendProbeUsage(provider, body, probe)
	if err != nil {
		probe.addCheck(checkOf(probeCase, LlmAuditUnknown))
		return probeCachePair{}
	}

	probe.addCheck(probeCacheCheck(probeCase, upstream, first, second))
	return probeCachePair{first: first, second: second, sentChars: len(filler) + len(prompt), ok: true}
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

func probeCacheCheck(probeCase *ProbeCase, upstream string, first probeUsage, second probeUsage) ProbeCheck {
	check := checkOf(probeCase, LlmAuditUnknown)
	check.Value = float64(second.cached())
	check.Facts = []string{strconv.Itoa(first.written()), strconv.Itoa(second.cached())}

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
func probeBillingEstimate(pair probeCachePair) int {
	return pair.sentChars / 4
}

func probeBillingCheck(probeCase *ProbeCase, pair probeCachePair) ProbeCheck {
	billed, again := pair.first.billedInput(), pair.second.billedInput()
	check := checkOf(probeCase, LlmAuditUnknown)
	if billed == 0 {
		return check
	}

	// The two requests were byte-identical, so the counts have to be too. This
	// part needs no tokenizer and no assumption.
	tolerance := firstFloat(probeCase.Params.DriftTolerance, 0.02)
	drift := math.Abs(float64(billed-again)) / float64(billed)
	if drift > tolerance {
		check.Level = LlmAuditAlert
		check.Value = drift
		// The leading token says which of the two comparisons the numbers are.
		check.Facts = []string{"drift", strconv.Itoa(billed), strconv.Itoa(again)}
		return check
	}

	estimate := probeBillingEstimate(pair)
	if estimate == 0 {
		return check
	}
	check.Value = float64(billed) / float64(estimate)
	check.Facts = []string{"estimate", strconv.Itoa(billed), strconv.Itoa(estimate)}

	warnHigh := firstFloat(probeCase.Params.WarnHigh, 1.5)
	alertHigh := firstFloat(probeCase.Params.AlertHigh, 2.5)
	warnLow := firstFloat(probeCase.Params.WarnLow, 0.7)
	alertLow := firstFloat(probeCase.Params.AlertLow, 0.4)
	switch {
	case check.Value >= alertHigh || check.Value <= alertLow:
		check.Level = LlmAuditAlert
	case check.Value >= warnHigh || check.Value <= warnLow:
		check.Level = LlmAuditWarn
	default:
		check.Level = LlmAuditOk
	}
	return check
}
