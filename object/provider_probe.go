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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

// A probe is the other half of the channel audit. The passive side reads what
// the agents already asked for, which cannot say anything a real request never
// happened to show; a probe asks the upstream a question chosen to have exactly
// one right answer, and costs a few cents of the user's own credit to do it.
//
// Everything here is written so a finding is reproducible: the request bodies
// are fixed, the checks compare against what the vendor's own API documents,
// and the report keeps what came back rather than a conclusion drawn from it.

// The triggers a probe can run under, kept on the row so a report says why it
// exists — and so an automatic one is told from a button press.
const (
	ProbeTriggerAdded  = "added"
	ProbeTriggerEdited = "edited"
	ProbeTriggerUnused = "unused"
	ProbeTriggerManual = "manual"
)

// The checks a probe runs.
const (
	ProbeIdentity = "identity"
	ProbeVendor   = "vendor"
	ProbeStream   = "stream"
	ProbeCache    = "cache"
	ProbeTools    = "tools"
	ProbeBilling  = "billing"
)

// probeKeepPerProvider is how many past probes are kept, which is what makes a
// provider that quietly changed backends visible later.
const probeKeepPerProvider = 20

// probeMaxParallel bounds a sweep, so adopting a machine with twenty providers
// does not open twenty upstream connections at once.
const probeMaxParallel = 3

// ProbeCheck is one measurement, in the same shape the passive audit uses: a
// level and the fact it was drawn from, with the wording left to the page.
type ProbeCheck struct {
	Key   string `json:"key"`
	Level string `json:"level"`
	// Facts are what actually came back, as data rather than as a sentence: the
	// model name that answered, the headers that were present, the events that
	// were missing, the two counts that did not agree. The page words them, so
	// a finding reads in the reader's own language and still quotes the value
	// it was drawn from.
	Facts []string `json:"facts"`
	// Value is the number the level was decided from, where there is one.
	Value float64 `json:"value"`
}

// ProviderProbe is one run of the suite against one provider.
type ProviderProbe struct {
	Id          int64  `xorm:"int notnull pk autoincr" json:"id"`
	Provider    string `xorm:"varchar(201) index" json:"provider"`
	CreatedTime string `xorm:"varchar(100) index" json:"createdTime"`

	Trigger  string `xorm:"varchar(20)" json:"trigger"`
	Protocol string `xorm:"varchar(20)" json:"protocol"`
	// Model is what the probe asked for, UpstreamModel what answered.
	Model         string `xorm:"varchar(255)" json:"model"`
	UpstreamModel string `xorm:"varchar(255)" json:"upstreamModel"`

	// Ok is whether the suite got far enough to judge anything. Error holds the
	// reason it did not, which is a finding of its own.
	Ok    bool   `xorm:"bool" json:"ok"`
	Error string `xorm:"varchar(500)" json:"error"`

	// TtftMs is the wait before the first streamed byte, which is what a chain
	// of resellers adds to and a total duration hides.
	TtftMs     int64 `xorm:"bigint" json:"ttftMs"`
	DurationMs int64 `xorm:"bigint" json:"durationMs"`

	// What the probe itself spent, so the price of knowing is on the report
	// rather than only on the bill.
	Requests         int     `xorm:"int" json:"requests"`
	PromptTokens     int     `xorm:"int" json:"promptTokens"`
	CompletionTokens int     `xorm:"int" json:"completionTokens"`
	CacheReadTokens  int     `xorm:"int" json:"cacheReadTokens"`
	CacheWriteTokens int     `xorm:"int" json:"cacheWriteTokens"`
	Cost             float64 `xorm:"double" json:"cost"`
	Priced           bool    `xorm:"bool" json:"priced"`

	// VendorHeaders are the response headers of the vendor's own API that were
	// present, by name. A relay in front of a real API usually keeps some.
	VendorHeaders []string     `xorm:"varchar(1000)" json:"vendorHeaders"`
	Checks        []ProbeCheck `xorm:"mediumtext" json:"checks"`
}

// GetProviderProbeMode is how probes are started: "auto" runs them for a
// provider that was just added, one whose endpoint or key changed, and one that
// has never been probed; "manual" only ever runs one that was asked for; "off"
// disables them entirely.
func GetProviderProbeMode() string {
	switch strings.ToLower(conf.GetConfigStringUnquoted("providerProbeMode")) {
	case "off":
		return "off"
	case "manual":
		return "manual"
	default:
		return "auto"
	}
}

func isProbeAutomatic() bool {
	return GetProviderProbeMode() == "auto"
}

var probesRunning sync.Map

// ProbeProviderNow runs the suite and stores the result. It is the one entry
// point: everything else decides whether to call it.
func ProbeProviderNow(provider *Provider, trigger string) (*ProviderProbe, error) {
	if provider == nil {
		return nil, fmt.Errorf("no provider to probe")
	}
	if GetProviderProbeMode() == "off" {
		return nil, fmt.Errorf("probing is turned off")
	}

	id := provider.GetId()
	if _, busy := probesRunning.LoadOrStore(id, true); busy {
		return nil, fmt.Errorf("a probe of %s is already running", id)
	}
	defer probesRunning.Delete(id)

	probe := runProviderProbe(provider, trigger)
	if err := saveProviderProbe(probe); err != nil {
		return probe, err
	}
	return probe, nil
}

// probeProviderInBackground is the automatic path. It never blocks the caller
// and never reports to it: a probe that could not run leaves the provider
// unprobed, which the page shows as such.
func probeProviderInBackground(provider *Provider, trigger string) {
	if provider == nil || !isProbeAutomatic() {
		return
	}
	copied := *provider
	go func() {
		if _, err := ProbeProviderNow(&copied, trigger); err != nil {
			beego.Error("provider probe failed for", copied.GetId()+":", err)
		}
	}()
}

// ProbeNewProvider runs after a provider is stored. Adding a provider already
// sends one request to the upstream to check the key, so the credential is
// spent either way; this asks the same upstream the questions that say whether
// what is behind the key is what it claims to be.
func ProbeNewProvider(provider *Provider) {
	probeProviderInBackground(provider, ProbeTriggerAdded)
}

// ProbeEditedProvider runs when the part of a provider a probe measured has
// changed. Renaming one or editing its notes does not reprobe it; repointing it
// at another endpoint, or giving it another key, does.
func ProbeEditedProvider(stored *Provider, updated *Provider, keyChanged bool) {
	if stored == nil || updated == nil {
		return
	}
	if !keyChanged && stored.BaseUrl == updated.BaseUrl &&
		stored.Type == updated.Type && stored.AuthMode == updated.AuthMode {
		return
	}
	probeProviderInBackground(updated, ProbeTriggerEdited)
}

// probeSweepDelay lets the process finish starting before it spends anything,
// and probeSweepInterval catches a provider added while probing was off.
const (
	probeSweepDelay    = 45 * time.Second
	probeSweepInterval = 6 * time.Hour
)

// StartProviderProbeSweep probes the providers that were configured before
// probing existed, or before it was turned on. A provider is swept once: the
// sweep looks for the ones with no probe at all, so it goes quiet by itself.
func StartProviderProbeSweep() {
	go func() {
		time.Sleep(probeSweepDelay)
		for {
			sweepUnprobedProviders()
			time.Sleep(probeSweepInterval)
		}
	}()
}

func sweepUnprobedProviders() {
	if !isProbeAutomatic() || ormer == nil || ormer.Engine == nil {
		return
	}

	// An empty owner lists every account's providers, which is what a sweep of
	// this machine has to cover.
	providers, err := GetProviders("")
	if err != nil {
		beego.Error("provider probe sweep could not list providers:", err)
		return
	}
	probed, err := probedProviderIds()
	if err != nil {
		beego.Error("provider probe sweep could not read past probes:", err)
		return
	}

	pending := []*Provider{}
	for _, provider := range providers {
		if provider.Status == "disabled" || probed[provider.GetId()] || !isProbable(provider) {
			continue
		}
		pending = append(pending, provider)
	}
	if len(pending) == 0 {
		return
	}

	gate := make(chan struct{}, probeMaxParallel)
	group := sync.WaitGroup{}
	for _, provider := range pending {
		group.Add(1)
		gate <- struct{}{}
		go func(target *Provider) {
			defer group.Done()
			defer func() { <-gate }()
			if _, err := ProbeProviderNow(target, ProbeTriggerUnused); err != nil {
				beego.Error("provider probe failed for", target.GetId()+":", err)
			}
		}(provider)
	}
	group.Wait()
}

// isProbable reports whether there is anything to probe with. A provider that
// forwards the caller's own login has no credential of its own here, so the
// suite would only ever measure a 401.
func isProbable(provider *Provider) bool {
	if !IsProviderTypeSupported(provider) || provider.BaseUrl == "" {
		return false
	}
	return !UsesClientAuth(provider) && provider.ApiKey != ""
}

func probedProviderIds() (map[string]bool, error) {
	rows := []ProviderProbe{}
	err := ormer.Engine.Table("provider_probe").Distinct("provider").Find(&rows)
	if err != nil {
		return nil, err
	}
	probed := map[string]bool{}
	for _, row := range rows {
		probed[row.Provider] = true
	}
	return probed, nil
}

func saveProviderProbe(probe *ProviderProbe) error {
	if ormer == nil || ormer.Engine == nil {
		return fmt.Errorf("no database")
	}
	if _, err := ormer.Engine.Insert(probe); err != nil {
		return err
	}
	prunePastProbes(probe.Provider)
	return nil
}

// prunePastProbes keeps the recent history of one provider and drops the rest.
func prunePastProbes(providerId string) {
	ids := []int64{}
	err := ormer.Engine.Table("provider_probe").Cols("id").
		Where("provider = ?", providerId).Desc("id").Limit(1, probeKeepPerProvider).
		Find(&ids)
	if err != nil || len(ids) == 0 {
		return
	}
	if _, err := ormer.Engine.In("id", ids).Delete(&ProviderProbe{}); err != nil {
		beego.Error("provider probe cleanup failed:", err)
	}
}

// GetProviderProbes is the newest probe of every provider that has one, which
// is what the audit page shows beside the traffic.
func GetProviderProbes() ([]*ProviderProbe, error) {
	if ormer == nil || ormer.Engine == nil {
		return []*ProviderProbe{}, nil
	}

	probes := []*ProviderProbe{}
	// Every kept probe is read and the newest per provider taken here: the
	// history is bounded per provider, and the three drivers Gateway supports
	// spell a per-group maximum differently.
	err := ormer.Engine.Desc("id").Find(&probes)
	if err != nil {
		return nil, err
	}

	latest := []*ProviderProbe{}
	seen := map[string]bool{}
	for _, probe := range probes {
		if seen[probe.Provider] {
			continue
		}
		seen[probe.Provider] = true
		latest = append(latest, probe)
	}
	return latest, nil
}

// GetProviderProbeHistory is every kept run for one provider, newest first. Two
// runs of the same suite months apart are what says a backend was swapped.
func GetProviderProbeHistory(providerId string) ([]*ProviderProbe, error) {
	probes := []*ProviderProbe{}
	if ormer == nil || ormer.Engine == nil {
		return probes, nil
	}
	err := ormer.Engine.Where("provider = ?", providerId).Desc("id").Find(&probes)
	return probes, err
}

// DeleteProviderProbes drops the history of a provider that no longer exists.
func DeleteProviderProbes(providerId string) error {
	if ormer == nil || ormer.Engine == nil {
		return nil
	}
	_, err := ormer.Engine.Where("provider = ?", providerId).Delete(&ProviderProbe{})
	return err
}

// newProviderProbe is the row a run starts from, so a failure before the first
// request still produces a report saying so.
func newProviderProbe(provider *Provider, trigger string) *ProviderProbe {
	return &ProviderProbe{
		Provider:      provider.GetId(),
		CreatedTime:   util.GetCurrentTime(),
		Trigger:       trigger,
		Protocol:      ProviderProtocol(provider),
		VendorHeaders: []string{},
		Checks:        []ProbeCheck{},
	}
}
