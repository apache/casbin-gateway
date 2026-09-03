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

// The test cases a probe runs. They are rows rather than constants so that the
// suite can be read, argued with and changed by whoever is paying the bill: a
// case says what it asks, how it decides, and what it is worth to the score.

package object

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

// ProbeCaseParams are the knobs of the check engine a case runs on. Every
// engine ignores the fields that are not its own, so one shape covers all six
// and the page can edit a case without knowing which fields matter.
type ProbeCaseParams struct {
	// The request. Empty fields fall back to what the engine ships with.
	System    string `json:"system"`
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"maxTokens"`

	// tools: the forced call and the schema it has to fill. Schema is JSON.
	ToolName string `json:"toolName"`
	Schema   string `json:"schema"`

	// stream: the event names this API documents, in the order they are due.
	Events []string `json:"events"`

	// cache: how long the cached prefix is, and how long to wait before asking
	// for it back.
	FillerChars int `json:"fillerChars"`
	GapMs       int `json:"gapMs"`

	// vendor: the response headers of the vendor's own API, and how many have
	// to be present for the case to pass.
	Headers    []string `json:"headers"`
	MinHeaders int      `json:"minHeaders"`

	// billing: how far two byte-identical requests may drift apart, and how far
	// the billed input may sit from what was actually sent.
	DriftTolerance float64 `json:"driftTolerance"`
	WarnHigh       float64 `json:"warnHigh"`
	AlertHigh      float64 `json:"alertHigh"`
	WarnLow        float64 `json:"warnLow"`
	AlertLow       float64 `json:"alertLow"`
}

// ProbeCase is one test in the suite. It is the unit the score is built out of
// and the unit the page publishes: Question is what it asks the upstream and
// Method is how the answer is judged, both in the words of whoever wrote it.
type ProbeCase struct {
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	DisplayName string `xorm:"varchar(200)" json:"displayName"`
	// Check is the engine that runs it: identity, vendor, stream, cache, tools
	// or billing. It decides which request is sent and how the answer is read.
	Check string `xorm:"varchar(20)" json:"check"`
	// Protocol limits a case to one upstream API. Empty runs it against both.
	Protocol string `xorm:"varchar(20)" json:"protocol"`
	Enabled  bool   `xorm:"bool" json:"enabled"`
	// Weight is what this case is worth in the authenticity score, relative to
	// the other cases that could be measured on the same run.
	Weight int `xorm:"int" json:"weight"`
	// Sort orders the suite, on the report as well as in the run.
	Sort int `xorm:"int" json:"sort"`
	// BuiltIn marks a case Gateway ships, which is what restoring the defaults
	// puts back. An edited built-in case stays built-in.
	BuiltIn bool `xorm:"bool" json:"builtIn"`
	// Shipped fingerprints the built-in case this row was written from. It is
	// what tells a case nobody has touched from one someone rewrote, which is
	// what lets a later release improve a shipped case without overwriting
	// anyone's words.
	Shipped string `xorm:"varchar(64)" json:"-"`

	Question string `xorm:"varchar(1000)" json:"question"`
	Method   string `xorm:"varchar(2000)" json:"method"`

	Params ProbeCaseParams `xorm:"mediumtext json" json:"params"`

	// Edited is whether a built-in case still reads the way it shipped. It is
	// computed rather than stored, and it is what lets the page show a shipped
	// case in the reader's own language while showing a rewritten one in the
	// words whoever rewrote it actually used.
	Edited bool `xorm:"-" json:"edited"`
}

// probeCaseChecks are the engines a case may name.
var probeCaseChecks = []string{ProbeIdentity, ProbeVendor, ProbeStream, ProbeCache, ProbeTools, ProbeBilling}

func isProbeCaseCheck(check string) bool {
	for _, known := range probeCaseChecks {
		if check == known {
			return true
		}
	}
	return false
}

// GetProbeCases is the whole suite in run order, disabled cases included: the
// page publishes what is not being asked as much as what is.
func GetProbeCases() ([]*ProbeCase, error) {
	cases := []*ProbeCase{}
	if ormer == nil || ormer.Engine == nil {
		return cases, nil
	}
	if err := ormer.Engine.Find(&cases); err != nil {
		return nil, err
	}
	sortProbeCases(cases)
	markEditedProbeCases(cases)
	return cases, nil
}

// markEditedProbeCases compares each built-in case against the words it was
// written with. Only the question is compared: a case that was reweighted or
// turned off is still the shipped question, asked the shipped way.
func markEditedProbeCases(cases []*ProbeCase) {
	for _, probeCase := range cases {
		probeCase.Edited = isProbeCaseEdited(probeCase)
	}
}

func isProbeCaseEdited(probeCase *ProbeCase) bool {
	if !probeCase.BuiltIn {
		return false
	}
	// A row stored before cases were fingerprinted has only its timestamps to
	// go on, which are enough: a seeded case is written once, so one that was
	// never written again is one nobody has touched.
	if probeCase.Shipped == "" {
		return probeCase.UpdatedTime != probeCase.CreatedTime
	}
	return probeCaseFingerprint(probeCase) != probeCase.Shipped
}

// probeCaseFingerprint covers what a case asks and how it judges the answer,
// which is what a reader would call the case itself.
func probeCaseFingerprint(probeCase *ProbeCase) string {
	params, err := json.Marshal(probeCase.Params)
	if err != nil {
		params = []byte("{}")
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		probeCase.DisplayName,
		probeCase.Check,
		probeCase.Protocol,
		probeCase.Question,
		probeCase.Method,
		string(params),
	}, "|")))
	return hex.EncodeToString(sum[:])
}

func sortProbeCases(cases []*ProbeCase) {
	sort.SliceStable(cases, func(left, right int) bool {
		if cases[left].Sort != cases[right].Sort {
			return cases[left].Sort < cases[right].Sort
		}
		return cases[left].Name < cases[right].Name
	})
}

// probeCasesFor is the enabled part of the suite that applies to one upstream
// API, which is what a run actually executes. A database that could not be read
// falls back to the shipped suite: a probe measured against nothing would score
// every provider as unknown.
func probeCasesFor(protocol string) []*ProbeCase {
	all, err := GetProbeCases()
	if err != nil {
		beego.Error("probe cases could not be read:", err)
		all = nil
	}
	if len(all) == 0 {
		all = builtInProbeCases()
	}

	running := []*ProbeCase{}
	for _, probeCase := range all {
		if !probeCase.Enabled || !isProbeCaseCheck(probeCase.Check) {
			continue
		}
		if probeCase.Protocol != "" && probeCase.Protocol != protocol {
			continue
		}
		running = append(running, probeCase)
	}
	return running
}

func probeCasesOf(cases []*ProbeCase, check string) []*ProbeCase {
	matching := []*ProbeCase{}
	for _, probeCase := range cases {
		if probeCase.Check == check {
			matching = append(matching, probeCase)
		}
	}
	return matching
}

// GetProbeCase reads one case by name.
func GetProbeCase(name string) (*ProbeCase, error) {
	if ormer == nil || ormer.Engine == nil {
		return nil, nil
	}
	stored := ProbeCase{Name: name}
	existed, err := ormer.Engine.Get(&stored)
	if err != nil || !existed {
		return nil, err
	}
	return &stored, nil
}

// AddProbeCase stores a case someone wrote. A case added here is never built-in,
// so restoring the defaults leaves it alone.
func AddProbeCase(probeCase *ProbeCase) error {
	if err := normalizeProbeCase(probeCase); err != nil {
		return err
	}
	existing, err := GetProbeCase(probeCase.Name)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("a test case named %s already exists", probeCase.Name)
	}

	probeCase.BuiltIn = false
	probeCase.CreatedTime = util.GetCurrentTime()
	probeCase.UpdatedTime = probeCase.CreatedTime
	_, err = ormer.Engine.Insert(probeCase)
	return err
}

// UpdateProbeCase writes an edited case back. Whether it shipped with Gateway
// is not something an edit may change: that flag is what a reset reads.
func UpdateProbeCase(name string, probeCase *ProbeCase) error {
	if err := normalizeProbeCase(probeCase); err != nil {
		return err
	}
	stored, err := GetProbeCase(name)
	if err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("the test case %s does not exist", name)
	}

	probeCase.Name = name
	probeCase.BuiltIn = stored.BuiltIn
	probeCase.Shipped = stored.Shipped
	probeCase.CreatedTime = stored.CreatedTime
	probeCase.UpdatedTime = util.GetCurrentTime()
	_, err = ormer.Engine.ID(name).AllCols().Update(probeCase)
	return err
}

// DeleteProbeCase drops a case. A built-in one can be deleted like any other;
// restoring the defaults brings it back.
func DeleteProbeCase(name string) error {
	if ormer == nil || ormer.Engine == nil {
		return nil
	}
	_, err := ormer.Engine.ID(name).Delete(&ProbeCase{})
	return err
}

// ResetProbeCases puts the shipped suite back: every built-in case is rewritten
// as it ships and the ones that were deleted return. Cases someone wrote are
// left where they are.
func ResetProbeCases() error {
	if ormer == nil || ormer.Engine == nil {
		return nil
	}
	if _, err := ormer.Engine.Where("built_in = ?", true).Delete(&ProbeCase{}); err != nil {
		return err
	}
	return seedProbeCases()
}

func normalizeProbeCase(probeCase *ProbeCase) error {
	if probeCase == nil {
		return errors.New("no test case")
	}
	probeCase.Name = strings.TrimSpace(probeCase.Name)
	if probeCase.Name == "" {
		return errors.New("a test case needs a name")
	}
	if !isProbeCaseCheck(probeCase.Check) {
		return fmt.Errorf("%s is not a check this suite can run", probeCase.Check)
	}
	if probeCase.Protocol != "" && probeCase.Protocol != ProtocolAnthropic && probeCase.Protocol != ProtocolOpenAi {
		return fmt.Errorf("%s is not an upstream API this suite can ask", probeCase.Protocol)
	}
	if probeCase.Weight < 0 {
		return errors.New("a weight cannot be negative")
	}
	if probeCase.Params.Schema != "" && !json.Valid([]byte(probeCase.Params.Schema)) {
		return errors.New("the tool schema is not valid JSON")
	}
	if probeCase.DisplayName == "" {
		probeCase.DisplayName = probeCase.Name
	}
	return nil
}

// InitProbeCases brings the stored suite up to what this release ships: cases
// that are missing are added, a shipped case nobody has rewritten is rewritten
// as it now ships, and one this release no longer ships is dropped. A case
// someone wrote or edited is left exactly as they left it.
func InitProbeCases() {
	if err := seedProbeCases(); err != nil {
		beego.Error("probe cases could not be created:", err)
	}
}

func seedProbeCases() error {
	if ormer == nil || ormer.Engine == nil {
		return nil
	}

	stored, err := GetProbeCases()
	if err != nil {
		return err
	}
	known := map[string]*ProbeCase{}
	for _, probeCase := range stored {
		known[probeCase.Name] = probeCase
	}

	shipping := map[string]bool{}
	for _, probeCase := range builtInProbeCases() {
		shipping[probeCase.Name] = true
		probeCase.Shipped = probeCaseFingerprint(probeCase)

		existing := known[probeCase.Name]
		if existing == nil {
			probeCase.CreatedTime = util.GetCurrentTime()
			probeCase.UpdatedTime = probeCase.CreatedTime
			if _, err := ormer.Engine.Insert(probeCase); err != nil {
				return err
			}
			continue
		}
		if existing.Edited || existing.Shipped == probeCase.Shipped {
			continue
		}
		if err := refreshProbeCase(probeCase); err != nil {
			return err
		}
	}

	// A built-in case this release stopped shipping has nothing left to run:
	// the engine it named or the question it asked is gone.
	for _, probeCase := range stored {
		if !probeCase.BuiltIn || probeCase.Edited || shipping[probeCase.Name] {
			continue
		}
		if err := DeleteProbeCase(probeCase.Name); err != nil {
			return err
		}
	}
	return nil
}

// refreshProbeCase rewrites the question and leaves the settings: what a case
// asks is Gateway's, what it is worth and whether it runs at all is not.
func refreshProbeCase(probeCase *ProbeCase) error {
	probeCase.UpdatedTime = util.GetCurrentTime()
	_, err := ormer.Engine.ID(probeCase.Name).
		Cols("display_name", "check", "protocol", "question", "method", "params", "shipped", "updated_time").
		Update(probeCase)
	return err
}
