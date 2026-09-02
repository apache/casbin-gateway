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
	"errors"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/util"
)

// Where a price in effect came from. A stored entry carries one of the last
// two; the first two are computed when the table is merged.
const (
	LlmPriceSourceBuiltIn   = "built-in"
	LlmPriceSourceFile      = "file"
	LlmPriceSourceManual    = "manual"
	LlmPriceSourceModelsDev = "models.dev"
)

// LlmPriceEntry is a stored override of what a model costs, edited on the
// Pricing page or written by a models.dev sync. Model is matched the way the
// built-in table is: as a substring of the name a request arrives under, so one
// entry covers the dated and vendor-prefixed shapes of the same model.
type LlmPriceEntry struct {
	Model       string `xorm:"varchar(255) notnull pk" json:"model"`
	DisplayName string `xorm:"varchar(255)" json:"displayName"`

	Input        float64 `xorm:"double" json:"input"`
	Output       float64 `xorm:"double" json:"output"`
	CacheWrite   float64 `xorm:"double" json:"cacheWrite"`
	CacheRead    float64 `xorm:"double" json:"cacheRead"`
	CacheWrite1h float64 `xorm:"double" json:"cacheWrite1h"`

	// Source is what wrote this row: a sync overwrites its own rows and leaves
	// a hand-edited one alone, so a correction survives the next sync.
	Source      string `xorm:"varchar(20)" json:"source"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`
}

// LlmPriceView is one model's price as it is actually in effect, with the layer
// it came from. The Pricing page lists these rather than the stored rows: a
// built-in price nothing overrides is still a price, and hiding it would make
// the table look emptier than the costing is.
type LlmPriceView struct {
	Model       string `json:"model"`
	DisplayName string `json:"displayName"`
	LlmPrice
	Source string `json:"source"`
	// Overridden is true where a stored row is standing in front of a built-in
	// entry, which is what "Reset" on the page has to offer.
	Overridden bool `json:"overridden"`
}

func (entry *LlmPriceEntry) price() LlmPrice {
	return LlmPrice{
		Input:        entry.Input,
		Output:       entry.Output,
		CacheWrite:   entry.CacheWrite,
		CacheRead:    entry.CacheRead,
		CacheWrite1h: entry.CacheWrite1h,
	}
}

// normalizeLlmPriceModel spells a key the way the matcher reads it: lowercase,
// trimmed, and without the provider prefix or the tag some catalogues append,
// so "anthropic/claude-opus-5:beta" and "Claude-Opus-5" are one entry.
func normalizeLlmPriceModel(model string) string {
	name := strings.ToLower(strings.TrimSpace(model))
	if slash := strings.LastIndex(name, "/"); slash != -1 {
		name = name[slash+1:]
	}
	if colon := strings.Index(name, ":"); colon != -1 {
		name = name[:colon]
	}
	return strings.TrimSpace(strings.ReplaceAll(name, "@", "-"))
}

// GetLlmPriceEntries lists the stored overrides, newest key order aside: the
// page sorts them, this only has to be stable.
func GetLlmPriceEntries() ([]*LlmPriceEntry, error) {
	entries := []*LlmPriceEntry{}
	if ormer == nil {
		return entries, nil
	}
	err := ormer.Engine.Asc("model").Find(&entries)
	return entries, err
}

// SetLlmPriceEntry writes one override and reloads the table, so a price
// changed on the page costs the next request rather than the next restart.
func SetLlmPriceEntry(entry *LlmPriceEntry) error {
	entry.Model = normalizeLlmPriceModel(entry.Model)
	if entry.Model == "" {
		return errors.New("a price needs a model name")
	}
	if entry.Input < 0 || entry.Output < 0 || entry.CacheWrite < 0 ||
		entry.CacheRead < 0 || entry.CacheWrite1h < 0 {
		return errors.New("a price cannot be negative")
	}
	if entry.Source == "" {
		entry.Source = LlmPriceSourceManual
	}
	entry.UpdatedTime = util.GetCurrentTime()

	if err := upsertLlmPriceEntry(entry); err != nil {
		return err
	}
	ReloadLlmPrices()
	return nil
}

func upsertLlmPriceEntry(entry *LlmPriceEntry) error {
	affected, err := ormer.Engine.ID(entry.Model).AllCols().Update(entry)
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	_, err = ormer.Engine.Insert(entry)
	return err
}

// DeleteLlmPriceEntry drops an override, which puts the built-in price for that
// model back in effect where there is one.
func DeleteLlmPriceEntry(model string) error {
	if _, err := ormer.Engine.ID(normalizeLlmPriceModel(model)).Delete(&LlmPriceEntry{}); err != nil {
		return err
	}
	ReloadLlmPrices()
	return nil
}

// PutLlmPriceEntries writes a batch under one source, leaving alone any model
// whose stored row was written by hand: a sync refreshes list prices, it does
// not undo a correction somebody made on purpose. It answers with the models it
// wrote and the ones it left.
func PutLlmPriceEntries(entries []*LlmPriceEntry, source string) (written []string, skipped []string, err error) {
	stored, err := GetLlmPriceEntries()
	if err != nil {
		return nil, nil, err
	}
	bySource := map[string]string{}
	for _, entry := range stored {
		bySource[entry.Model] = entry.Source
	}

	written, skipped = []string{}, []string{}
	for _, entry := range entries {
		entry.Model = normalizeLlmPriceModel(entry.Model)
		if entry.Model == "" {
			continue
		}
		if existing, found := bySource[entry.Model]; found && existing == LlmPriceSourceManual {
			skipped = append(skipped, entry.Model)
			continue
		}
		entry.Source = source
		entry.UpdatedTime = util.GetCurrentTime()
		if err := upsertLlmPriceEntry(entry); err != nil {
			return nil, nil, err
		}
		written = append(written, entry.Model)
	}

	sort.Strings(written)
	sort.Strings(skipped)
	ReloadLlmPrices()
	return written, skipped, nil
}
