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

// This file sends the probe requests and reads the answers. The bodies are
// fixed on purpose: a check is only worth showing if the same request can be
// sent again and produce the same finding.

package object

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/auditutil"
	"github.com/apache/casbin-gateway/proxy"
)

const (
	probeRequestTimeout = 90 * time.Second
	// probeMaxBody bounds one answer. A probe asks for a few tokens, so
	// anything past this is an error page.
	probeMaxBody = 1 << 20
	// probeErrorChars is how much of an upstream failure is kept on the report.
	probeErrorChars = 400
	// probeEvidenceChars bounds the request and the answer kept on each check.
	// A probe asks for a few tokens, so this holds a whole short answer.
	probeEvidenceChars = 1500
)

// The answer lengths asked for. They are small on purpose: what is being
// measured is the envelope, not the writing.
const (
	probeToolMaxTokens   = 256
	probeStreamMaxTokens = 32
	probeCacheMaxTokens  = 16
)

// runProviderProbe is the suite: the enabled test cases that apply to this
// upstream, in the order they are listed. It stops early where a later question
// would be meaningless — the first request decides whether the upstream is
// usable at all — and grades whatever it did measure.
func runProviderProbe(provider *Provider, trigger string) *ProviderProbe {
	probe := newProviderProbe(provider, trigger)
	start := time.Now()
	defer func() {
		probe.DurationMs = time.Since(start).Milliseconds()
		scoreProviderProbe(probe)
	}()

	if !isProbable(provider) {
		probe.Error = "this provider has no credential of its own to probe with"
		return probe
	}

	model, err := probeModelOf(provider)
	if err != nil {
		probe.Error = auditutil.BoundString(err.Error(), probeErrorChars)
		return probe
	}
	probe.Model = model

	cases := probeCasesFor(ProviderApiFamily(provider), model)
	if len(cases) == 0 {
		probe.Error = "no test case is enabled for this upstream API"
		return probe
	}

	runProbeCases(provider, model, probe, cases)
	priceProviderProbe(probe)
	return probe
}

// probeModelOf is the model a probe asks for. The first model configured on the
// provider is used rather than a cheap one: a report about a model nobody routes
// to would answer a question nobody asked. A provider naming none is asked what
// it serves.
func probeModelOf(provider *Provider) (string, error) {
	for _, model := range provider.Models {
		if strings.TrimSpace(model) != "" {
			return strings.TrimSpace(model), nil
		}
	}

	models, err := FetchProviderModels(provider)
	if err != nil {
		return "", err
	}
	return models[0], nil
}

// ---------------------------------------------------------------------------
// The requests
// ---------------------------------------------------------------------------

// probeAnswer is one upstream answer, kept whole so every check reads the same
// bytes rather than a summary of them.
type probeAnswer struct {
	status int
	header http.Header
	body   []byte
	ttftMs int64
	// events are the SSE event names in the order they arrived, empty for a
	// request that was not streamed.
	events []string
	// payloads are the SSE data payloads, in the same order.
	payloads [][]byte
}

func (answer *probeAnswer) ok() bool {
	return answer.status >= 200 && answer.status < 300
}

// probeFailure is the one-line reason a request did not answer, with the
// upstream's own words: the status code alone almost never says which of key,
// URL, model name or plan was the problem.
func (answer *probeAnswer) failure() string {
	text := strings.Join(strings.Fields(string(answer.body)), " ")
	if _, message := readProbeError(answer.body); message != "" {
		text = message
	}
	return auditutil.BoundString(fmt.Sprintf("the upstream answered %d: %s", answer.status, text), probeErrorChars)
}

func readProbeError(body []byte) (string, string) {
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", ""
	}
	return parsed.Error.Type, parsed.Error.Message
}

// sendProbe posts one probe body to the provider's completion endpoint. An
// upstream that rejects "max_tokens" is asked again the way the newer OpenAI
// models spell it, so a provider serving those is measured rather than skipped.
func sendProbe(provider *Provider, body map[string]any, stream bool) (*probeAnswer, error) {
	answer, err := postProbe(provider, body, stream)
	if err != nil || answer.ok() {
		return answer, err
	}

	if _, message := readProbeError(answer.body); strings.Contains(strings.ToLower(message), "max_tokens") {
		if limit, ok := body["max_tokens"]; ok {
			retried := map[string]any{}
			for key, value := range body {
				retried[key] = value
			}
			delete(retried, "max_tokens")
			retried["max_completion_tokens"] = limit
			return postProbe(provider, retried, stream)
		}
	}
	return answer, nil
}

func postProbe(provider *Provider, body map[string]any, stream bool) (*probeAnswer, error) {
	upstream := ProviderApiFamily(provider)
	endpoint := "/chat/completions"
	if upstream == ProtocolAnthropic {
		endpoint = "/v1/messages"
	}

	target, err := BuildProviderUrl(provider.BaseUrl, upstream, endpoint)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if upstream == ProtocolAnthropic {
		request.Header.Set("Anthropic-Version", AnthropicVersion)
	}
	SetProviderAuth(request.Header, provider)

	client := &http.Client{Timeout: probeRequestTimeout, Transport: proxy.Transport()}
	start := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	answer := &probeAnswer{status: response.StatusCode, header: response.Header}
	if !stream || !answer.ok() {
		answer.body, err = io.ReadAll(io.LimitReader(response.Body, probeMaxBody))
		answer.ttftMs = time.Since(start).Milliseconds()
		return answer, err
	}

	readProbeStream(answer, response.Body, start)
	return answer, nil
}

// readProbeStream keeps the shape of an event stream: the event names in order,
// their payloads, and how long the first one took. The names are the point —
// a backend pretending to be another API usually gets the envelope wrong long
// before it gets the text wrong.
func readProbeStream(answer *probeAnswer, body io.Reader, start time.Time) {
	reader := bufio.NewReaderSize(body, 64*1024)
	name, data := "", []byte{}
	first := true

	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(trimmed, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
		case strings.HasPrefix(trimmed, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(trimmed, "data:"), " ")...)
		case trimmed == "" && len(data) > 0:
			if first {
				answer.ttftMs = time.Since(start).Milliseconds()
				first = false
			}
			answer.events = append(answer.events, name)
			answer.payloads = append(answer.payloads, append([]byte{}, data...))
			name, data = "", data[:0]
		}
		if err != nil {
			if len(data) > 0 {
				answer.events = append(answer.events, name)
				answer.payloads = append(answer.payloads, append([]byte{}, data...))
			}
			return
		}
	}
}

// ---------------------------------------------------------------------------
// The usage counters, in both spellings
// ---------------------------------------------------------------------------

type probeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`

	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	PromptCacheHitTokens int `json:"prompt_cache_hit_tokens"`
}

// fresh, cached, written and answered are the four counters the rest of the
// gateway prices, read out of whichever spelling arrived.
func (usage probeUsage) fresh() int {
	if usage.PromptTokens > 0 {
		return maxInt(usage.PromptTokens-usage.cached(), 0)
	}
	return usage.InputTokens
}

func (usage probeUsage) cached() int {
	return maxInt(maxInt(usage.CacheReadInputTokens, usage.PromptTokensDetails.CachedTokens), usage.PromptCacheHitTokens)
}

func (usage probeUsage) written() int {
	return usage.CacheCreationInputTokens
}

func (usage probeUsage) answered() int {
	return maxInt(usage.OutputTokens, usage.CompletionTokens)
}

// billedInput is everything the upstream counted as input, however it split it.
// Two identical requests have to be billed the same amount of it.
func (usage probeUsage) billedInput() int {
	return usage.fresh() + usage.cached() + usage.written()
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

// probeBody is the outer shape both APIs answer a completion in, as far as a
// probe needs to read it.
type probeBody struct {
	Model   string     `json:"model"`
	Usage   probeUsage `json:"usage"`
	Content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Choices []struct {
		Message struct {
			ToolCalls []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (probe *ProviderProbe) addUsage(usage probeUsage) {
	probe.Requests++
	probe.PromptTokens += usage.fresh()
	probe.CompletionTokens += usage.answered()
	probe.CacheReadTokens += usage.cached()
	probe.CacheWriteTokens += usage.written()
}

func (probe *ProviderProbe) addCheck(check ProbeCheck) {
	probe.Checks = append(probe.Checks, check)
}

// priceProviderProbe costs what the probe itself spent, so the report can say
// what it took to produce rather than leaving it to the next invoice.
func priceProviderProbe(probe *ProviderProbe) {
	cost, priced := GetLlmCost(probe.Model, probe.PromptTokens, probe.CompletionTokens,
		probe.CacheWriteTokens, probe.CacheReadTokens)
	probe.Cost, probe.Priced = cost, priced
}
