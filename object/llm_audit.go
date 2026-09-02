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
	"sort"

	"github.com/apache/casbin-gateway/protocol"
)

// The audit reads records that were kept anyway and adds nothing to the request
// path. It measures what an upstream reported about itself, which is evidence
// about a provider rather than a verdict on one: every number here is meant to
// be quoted back together with the sample it was measured over.

// llmAuditScanLimit bounds how many records one audit reads, so the page stays
// answerable on a table configured to keep far more than the default.
const llmAuditScanLimit = 50000

// llmAuditMinSample is how many requests a check needs before its level means
// anything. Below it the check reports itself as unknown rather than guessing.
const llmAuditMinSample = 20

// llmAuditCacheMinTokens is the prompt size below which a missing cache says
// nothing: Anthropic will not cache a prefix shorter than its own minimum, so
// short requests are left out of the cache sample entirely.
const llmAuditCacheMinTokens = 2048

// llmAuditMaxModels bounds the model names carried on one report.
const llmAuditMaxModels = 24

// The levels a check ends in. "unknown" is not a failure: it is the honest
// answer when the window did not hold enough requests to measure.
const (
	LlmAuditOk      = "ok"
	LlmAuditWarn    = "warn"
	LlmAuditAlert   = "alert"
	LlmAuditUnknown = "unknown"
)

// The checks, named so that the page can word each one itself.
const (
	LlmAuditCache   = "cache"
	LlmAuditErrors  = "errors"
	LlmAuditLatency = "latency"
	LlmAuditPricing = "pricing"
)

// LlmAuditCheck is one measurement, kept as a number and a level rather than a
// sentence: what it means is worded by whatever renders it.
type LlmAuditCheck struct {
	Key   string `json:"key"`
	Level string `json:"level"`
	// Value is a share between 0 and 1 for every check but latency, where it is
	// the ratio of the slow tail to the median.
	Value float64 `json:"value"`
	// Sample is how many requests the value was measured over, which is the
	// half of a rate that says whether to believe it.
	Sample int64 `json:"sample"`
}

// LlmProviderAudit is what the kept records say about one provider.
type LlmProviderAudit struct {
	Provider string `json:"provider"`

	// Requests counts the records this provider answered, failures included.
	Requests int64 `json:"requests"`
	Failed   int64 `json:"failed"`
	// FailedOver counts the attempts this provider lost the request on, which
	// the chain then had another provider answer. They are not in Requests: a
	// record belongs to whoever answered it.
	FailedOver int64 `json:"failedOver"`
	Retried    int64 `json:"retried"`

	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	CacheReadTokens  int64   `json:"cacheReadTokens"`
	CacheWriteTokens int64   `json:"cacheWriteTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	Cost             float64 `json:"cost"`

	// Cacheable counts the answered requests a cache could have shown up on at
	// all: the Anthropic protocol accounts a cache separately, and the prompt
	// was long enough to be worth caching. CacheHits is how many of those came
	// back with any cache accounting on them.
	Cacheable int64 `json:"cacheable"`
	CacheHits int64 `json:"cacheHits"`

	LatencyP50Ms int64 `json:"latencyP50Ms"`
	LatencyP95Ms int64 `json:"latencyP95Ms"`

	// Unpriced counts answered requests whose model has no rate anywhere, and
	// UnpricedModels names them: a model nobody publishes a rate for is a model
	// name this upstream invented.
	Unpriced       int64    `json:"unpriced"`
	UnpricedModels []string `json:"unpricedModels"`
	Models         []string `json:"models"`

	FirstTime string `json:"firstTime"`
	LastTime  string `json:"lastTime"`

	Checks []LlmAuditCheck `json:"checks"`

	// The working sets the exported fields above are cut from.
	latencies      []int64
	models         map[string]bool
	unpricedModels map[string]bool
}

// LlmAuditReport is one window's worth of providers, with what it was measured
// over: a report that read fewer records than the window holds has to say so.
type LlmAuditReport struct {
	Scanned   int64               `json:"scanned"`
	Truncated bool                `json:"truncated"`
	Providers []*LlmProviderAudit `json:"providers"`
}

// llmAuditCols are the columns an audit reads. The bodies are left behind: the
// report is about how an upstream answered, not about what it was asked.
var llmAuditCols = []string{
	"created_time", "protocol", "model", "provider", "status", "duration_ms",
	"attempts", "failures", "prompt_tokens", "completion_tokens",
	"cache_read_tokens", "cache_write_tokens", "total_tokens", "cost", "priced",
}

// GetLlmProviderAudit reads the kept records of one window and reports what
// each provider in them looks like from the outside.
func GetLlmProviderAudit(filter LlmRecordFilter) (*LlmAuditReport, error) {
	session := llmRecordSession(filter)
	defer session.Close()

	records := []*LlmRecord{}
	err := session.Cols(llmAuditCols...).Desc("id").Limit(llmAuditScanLimit).Find(&records)
	if err != nil {
		return nil, err
	}

	report := &LlmAuditReport{
		Scanned:   int64(len(records)),
		Truncated: len(records) >= llmAuditScanLimit,
		Providers: []*LlmProviderAudit{},
	}

	audits := map[string]*LlmProviderAudit{}
	for _, record := range records {
		if record.Provider != "" {
			addLlmAuditRecord(audits, record)
		}
		for _, failure := range llmAuditFailoversOf(record) {
			if failure.Provider != "" {
				auditOf(audits, failure.Provider).FailedOver++
			}
		}
	}

	for _, audit := range audits {
		finishLlmAudit(audit)
		report.Providers = append(report.Providers, audit)
	}
	sort.Slice(report.Providers, func(i, j int) bool {
		left, right := report.Providers[i], report.Providers[j]
		if left.Requests != right.Requests {
			return left.Requests > right.Requests
		}
		return left.Provider < right.Provider
	})
	return report, nil
}

func auditOf(audits map[string]*LlmProviderAudit, provider string) *LlmProviderAudit {
	audit, ok := audits[provider]
	if !ok {
		audit = &LlmProviderAudit{
			Provider:       provider,
			UnpricedModels: []string{},
			Models:         []string{},
			Checks:         []LlmAuditCheck{},
			models:         map[string]bool{},
			unpricedModels: map[string]bool{},
		}
		audits[provider] = audit
	}
	return audit
}

// llmAuditFailoversOf is the attempts a record failed over from, without the
// one the record itself already counts. A chain whose last provider failed
// before writing anything appends that attempt to both, and counting it twice
// would make that provider look worse than it was.
func llmAuditFailoversOf(record *LlmRecord) []LlmFailure {
	failures := record.Failures
	if len(failures) == 0 {
		return nil
	}
	last := failures[len(failures)-1]
	if last.Provider == record.Provider && !llmAuditAnswered(record) {
		failures = failures[:len(failures)-1]
	}
	return failures
}

func llmAuditAnswered(record *LlmRecord) bool {
	return record.Status >= 200 && record.Status < 300
}

func addLlmAuditRecord(audits map[string]*LlmProviderAudit, record *LlmRecord) {
	audit := auditOf(audits, record.Provider)
	audit.Requests++
	if !llmAuditAnswered(record) {
		audit.Failed++
	}
	if record.Attempts > 1 {
		audit.Retried++
	}

	audit.PromptTokens += int64(record.PromptTokens)
	audit.CompletionTokens += int64(record.CompletionTokens)
	audit.CacheReadTokens += int64(record.CacheReadTokens)
	audit.CacheWriteTokens += int64(record.CacheWriteTokens)
	audit.TotalTokens += int64(record.TotalTokens)
	audit.Cost += record.Cost

	// The records arrive newest first, so the first one seen is the last one
	// there was, and every later one moves the beginning further back.
	if audit.LastTime == "" {
		audit.LastTime = record.CreatedTime
	}
	audit.FirstTime = record.CreatedTime

	if record.Model != "" && len(audit.models) < llmAuditMaxModels {
		audit.models[record.Model] = true
	}
	if !llmAuditAnswered(record) {
		return
	}

	if record.TotalTokens > 0 && !record.Priced {
		audit.Unpriced++
		if record.Model != "" && len(audit.unpricedModels) < llmAuditMaxModels {
			audit.unpricedModels[record.Model] = true
		}
	}
	if record.DurationMs > 0 {
		audit.latencies = append(audit.latencies, record.DurationMs)
	}

	if llmAuditCacheable(record) {
		audit.Cacheable++
		if record.CacheReadTokens > 0 || record.CacheWriteTokens > 0 {
			audit.CacheHits++
		}
	}
}

// llmAuditCacheable reports whether a cache could have been accounted for on
// this request at all. Only the Anthropic protocol reports a cache write, and
// only a prompt past the minimum is eligible for one, so counting anything else
// would make a zero look like a finding when it is the shape of the request.
func llmAuditCacheable(record *LlmRecord) bool {
	if record.Protocol != protocol.Anthropic {
		return false
	}
	return record.PromptTokens+record.CacheReadTokens >= llmAuditCacheMinTokens
}

func finishLlmAudit(audit *LlmProviderAudit) {
	audit.Models = sortedKeys(audit.models)
	audit.UnpricedModels = sortedKeys(audit.unpricedModels)

	sort.Slice(audit.latencies, func(i, j int) bool { return audit.latencies[i] < audit.latencies[j] })
	audit.LatencyP50Ms = llmAuditPercentile(audit.latencies, 0.5)
	audit.LatencyP95Ms = llmAuditPercentile(audit.latencies, 0.95)

	audit.Checks = []LlmAuditCheck{
		llmAuditCacheCheck(audit),
		llmAuditErrorCheck(audit),
		llmAuditLatencyCheck(audit),
		llmAuditPricingCheck(audit),
	}
	audit.models, audit.unpricedModels, audit.latencies = nil, nil, nil
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func llmAuditPercentile(sorted []int64, share float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(share*float64(len(sorted)-1))]
}

// llmAuditCacheCheck is the share of cacheable requests the upstream reported
// any cache accounting on. One that never reports any is either not passing
// prompt caching through or not implementing it, and is billing the whole
// prompt as fresh input either way.
func llmAuditCacheCheck(audit *LlmProviderAudit) LlmAuditCheck {
	check := LlmAuditCheck{Key: LlmAuditCache, Level: LlmAuditUnknown, Sample: audit.Cacheable}
	if audit.Cacheable == 0 {
		return check
	}

	check.Value = float64(audit.CacheHits) / float64(audit.Cacheable)
	if audit.Cacheable < llmAuditMinSample {
		return check
	}
	switch {
	case audit.CacheHits == 0:
		check.Level = LlmAuditAlert
	case check.Value < 0.2:
		check.Level = LlmAuditWarn
	default:
		check.Level = LlmAuditOk
	}
	return check
}

func llmAuditErrorCheck(audit *LlmProviderAudit) LlmAuditCheck {
	attempts := audit.Requests + audit.FailedOver
	check := LlmAuditCheck{Key: LlmAuditErrors, Level: LlmAuditUnknown, Sample: attempts}
	if attempts == 0 {
		return check
	}

	check.Value = float64(audit.Failed+audit.FailedOver) / float64(attempts)
	if attempts < llmAuditMinSample {
		return check
	}
	switch {
	case check.Value >= 0.2:
		check.Level = LlmAuditAlert
	case check.Value >= 0.05:
		check.Level = LlmAuditWarn
	default:
		check.Level = LlmAuditOk
	}
	return check
}

// llmAuditLatencyCheck measures the spread rather than the duration: how long a
// request takes depends on how much was asked for, but how far the slow tail
// sits from the middle is about the route it took, and a long chain of resellers
// is what makes that tail long.
func llmAuditLatencyCheck(audit *LlmProviderAudit) LlmAuditCheck {
	answered := audit.Requests - audit.Failed
	check := LlmAuditCheck{Key: LlmAuditLatency, Level: LlmAuditUnknown, Sample: answered}
	if audit.LatencyP50Ms == 0 {
		return check
	}

	check.Value = float64(audit.LatencyP95Ms) / float64(audit.LatencyP50Ms)
	if answered < llmAuditMinSample {
		return check
	}
	switch {
	case check.Value >= 10:
		check.Level = LlmAuditAlert
	case check.Value >= 5:
		check.Level = LlmAuditWarn
	default:
		check.Level = LlmAuditOk
	}
	return check
}

// llmAuditPricingCheck is the share of answered requests whose model has no
// rate anywhere. It says nothing on its own about an honest provider serving
// something new, and quite a lot about one serving a model name that exists
// nowhere else.
func llmAuditPricingCheck(audit *LlmProviderAudit) LlmAuditCheck {
	answered := audit.Requests - audit.Failed
	check := LlmAuditCheck{Key: LlmAuditPricing, Level: LlmAuditUnknown, Sample: answered}
	if answered == 0 {
		return check
	}

	check.Value = float64(audit.Unpriced) / float64(answered)
	switch {
	case check.Value >= 0.5:
		check.Level = LlmAuditAlert
	case audit.Unpriced > 0:
		check.Level = LlmAuditWarn
	default:
		check.Level = LlmAuditOk
	}
	return check
}
