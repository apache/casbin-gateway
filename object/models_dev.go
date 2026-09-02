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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/util"
)

// models.dev publishes one JSON document of every model it knows, with list
// prices already in US dollars per million tokens - the same unit LlmPrice is
// in, so nothing is converted on the way in.
const modelsDevUrl = "https://models.dev/api.json"

const (
	// The document is a few megabytes, so it is fetched with a timeout that
	// tolerates a slow link and cached rather than read once per search.
	modelsDevTimeout   = 60 * time.Second
	modelsDevCacheLife = time.Hour
	// A catalogue this large is never sent to the browser whole; a search
	// answers with a page of it.
	modelsDevSearchLimit = 200
	modelsDevMaxBytes    = 32 << 20
)

// ModelsDevModel is one model of the catalogue, priced at what most of the
// providers listing it agree that it costs - see fetchModelsDev.
type ModelsDevModel struct {
	Model       string `json:"model"`
	DisplayName string `json:"displayName"`
	ReleaseDate string `json:"releaseDate"`
	// Providers is how many of them listed this model, which is what says how
	// much agreement is behind the rates.
	Providers  int     `json:"providers"`
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// ModelsDevSync is what one sync did, in the terms the page reports it: which
// models were priced, which were left alone, and which the catalogue does not
// know. Missing is the useful half - it names the models still costing nothing.
type ModelsDevSync struct {
	Catalogue  int      `json:"catalogue"`
	Considered []string `json:"considered"`
	Updated    []string `json:"updated"`
	Skipped    []string `json:"skipped"`
	Missing    []string `json:"missing"`
	SyncedTime string   `json:"syncedTime"`
}

type modelsDevCost struct {
	Input      *float64 `json:"input"`
	Output     *float64 `json:"output"`
	CacheRead  *float64 `json:"cache_read"`
	CacheWrite *float64 `json:"cache_write"`
}

type modelsDevRawModel struct {
	Id          string         `json:"id"`
	Name        string         `json:"name"`
	ReleaseDate string         `json:"release_date"`
	Status      string         `json:"status"`
	Cost        *modelsDevCost `json:"cost"`
	Modalities  struct {
		Output []string `json:"output"`
	} `json:"modalities"`
}

type modelsDevRawProvider struct {
	Name   string                       `json:"name"`
	Models map[string]modelsDevRawModel `json:"models"`
}

var (
	modelsDevMutex   sync.Mutex
	modelsDevCache   []ModelsDevModel
	modelsDevFetched time.Time
)

// GetModelsDevCatalogue is the catalogue, fetched at most once an hour. force
// asks for it again regardless, which is what the Refresh button does.
func GetModelsDevCatalogue(force bool) ([]ModelsDevModel, error) {
	modelsDevMutex.Lock()
	defer modelsDevMutex.Unlock()

	if !force && modelsDevCache != nil && time.Since(modelsDevFetched) < modelsDevCacheLife {
		return modelsDevCache, nil
	}

	models, err := fetchModelsDev()
	if err != nil {
		// A stale catalogue answers better than nothing when the network is
		// down, and the caller is told neither way that it is stale: the sync
		// result carries the time it was read.
		if modelsDevCache != nil {
			return modelsDevCache, nil
		}
		return nil, err
	}

	modelsDevCache, modelsDevFetched = models, time.Now()
	return models, nil
}

func fetchModelsDev() ([]ModelsDevModel, error) {
	client := &http.Client{Timeout: modelsDevTimeout, Transport: proxy.Transport()}
	resp, err := client.Get(modelsDevUrl)
	if err != nil {
		return nil, fmt.Errorf("models.dev could not be reached: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, modelsDevMaxBytes))
	if err != nil {
		return nil, fmt.Errorf("models.dev could not be read: %s", err.Error())
	}

	catalogue := map[string]modelsDevRawProvider{}
	if err := json.Unmarshal(body, &catalogue); err != nil {
		return nil, fmt.Errorf("models.dev did not answer with the catalogue: %s", err.Error())
	}

	// One model is listed by every provider reselling it - thirty of them for a
	// current Claude - and they do not all fill the same fields in. Rather than
	// picking one listing and inheriting whatever it left out, each rate is the
	// one most of them agree on, counting a missing field as "this model has no
	// such charge". That is what reproduces the vendors' own published tables:
	// most listings of an OpenAI model carry no cache-write rate because OpenAI
	// does not charge for the write, while most listings of a Claude one do.
	groups := map[string][]modelsDevRawModel{}
	for _, provider := range catalogue {
		for modelId, raw := range provider.Models {
			if !pricesText(modelId, raw) {
				continue
			}
			name := normalizeLlmPriceModel(modelId)
			if name == "" {
				continue
			}
			if raw.Name == "" {
				raw.Name = modelId
			}
			groups[name] = append(groups[name], raw)
		}
	}

	models := make([]ModelsDevModel, 0, len(groups))
	for name, listings := range groups {
		input := agreedRate(listings, func(cost *modelsDevCost) *float64 { return cost.Input })
		output := agreedRate(listings, func(cost *modelsDevCost) *float64 { return cost.Output })
		// Both rates at zero is a listing nobody put a price on rather than a
		// model that is free, and costing requests at nothing while calling
		// them priced is worse than leaving them uncosted.
		if input <= 0 && output <= 0 {
			continue
		}
		models = append(models, ModelsDevModel{
			Model:       name,
			DisplayName: agreedText(listings, func(raw modelsDevRawModel) string { return raw.Name }),
			ReleaseDate: agreedText(listings, func(raw modelsDevRawModel) string { return raw.ReleaseDate }),
			Providers:   len(listings),
			Input:       input,
			Output:      output,
			CacheRead:   agreedRate(listings, func(cost *modelsDevCost) *float64 { return cost.CacheRead }),
			CacheWrite:  agreedRate(listings, func(cost *modelsDevCost) *float64 { return cost.CacheWrite }),
		})
	}

	// Newest first, so a search for a family shows the current generation
	// before the ones it replaced.
	sort.Slice(models, func(i, j int) bool {
		if models[i].ReleaseDate != models[j].ReleaseDate {
			return models[i].ReleaseDate > models[j].ReleaseDate
		}
		return models[i].Model < models[j].Model
	})
	return models, nil
}

// agreedRate is the rate most of the listings carry, an absent one counting as
// zero. A tie takes the lower rate, which is the half of a disagreement that
// cannot overstate what somebody is being told they spent.
func agreedRate(listings []modelsDevRawModel, read func(*modelsDevCost) *float64) float64 {
	counts := map[float64]int{}
	for _, raw := range listings {
		rate := 0.0
		if raw.Cost != nil {
			rate = amount(read(raw.Cost))
		}
		counts[rate]++
	}

	best, bestCount := 0.0, 0
	for rate, count := range counts {
		if count > bestCount || (count == bestCount && rate < best) {
			best, bestCount = rate, count
		}
	}
	return best
}

// agreedText is agreedRate for a name or a date: the spelling most of the
// listings use, ignoring the ones that left it empty.
func agreedText(listings []modelsDevRawModel, read func(modelsDevRawModel) string) string {
	counts := map[string]int{}
	for _, raw := range listings {
		if value := strings.TrimSpace(read(raw)); value != "" {
			counts[value]++
		}
	}

	best, bestCount := "", 0
	for value, count := range counts {
		if count > bestCount || (count == bestCount && value < best) {
			best, bestCount = value, count
		}
	}
	return best
}

// pricesText keeps the entries a token price means anything for: a model that
// is retired, or that answers in audio or pictures, is not one this gateway
// costs by the token.
func pricesText(modelId string, raw modelsDevRawModel) bool {
	if raw.Cost == nil || raw.Cost.Input == nil || raw.Cost.Output == nil {
		return false
	}
	if strings.EqualFold(raw.Status, "deprecated") {
		return false
	}
	if len(raw.Modalities.Output) > 0 {
		text := false
		for _, modality := range raw.Modalities.Output {
			switch strings.ToLower(modality) {
			case "text":
				text = true
			case "audio", "image", "video":
				return false
			}
		}
		if !text {
			return false
		}
	}

	searchable := strings.ToLower(modelId + " " + raw.Name)
	for _, marker := range []string{
		"embedding", "moderation", "realtime", "transcribe", "tts", "audio", "image", "video",
	} {
		if strings.Contains(searchable, marker) {
			return false
		}
	}
	return true
}

func amount(value *float64) float64 {
	if value == nil || *value < 0 {
		return 0
	}
	return *value
}

// SearchModelsDev is one page of the catalogue matching a query, for the dialog
// that adds a price for a model this machine has not run yet.
func SearchModelsDev(query string, force bool) ([]ModelsDevModel, error) {
	models, err := GetModelsDevCatalogue(force)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	found := []ModelsDevModel{}
	for _, model := range models {
		if needle != "" &&
			!strings.Contains(model.Model, needle) &&
			!strings.Contains(strings.ToLower(model.DisplayName), needle) {
			continue
		}
		found = append(found, model)
		if len(found) >= modelsDevSearchLimit {
			break
		}
	}
	return found, nil
}

// matchModelsDev finds the catalogue entry that prices a model name, by the
// same rule GetLlmPrice matches by: the longest catalogue id contained in the
// name. That is what makes one entry cover "claude-opus-5", the dated
// "claude-opus-5-20260214" and the vendor-prefixed "us.anthropic.claude-opus-5".
func matchModelsDev(models []ModelsDevModel, name string) (ModelsDevModel, bool) {
	needle := strings.ToLower(strings.TrimSpace(name))
	best, found := ModelsDevModel{}, false
	for _, model := range models {
		if !strings.Contains(needle, model.Model) {
			continue
		}
		if !found || len(model.Model) > len(best.Model) {
			best, found = model, true
		}
	}
	return best, found
}

// longCacheRate is what a cache written to live an hour costs. models.dev does
// not publish one, and a stored row replaces its built-in entry whole, so
// leaving it at zero would quietly bill every long write at the five-minute
// rate. The built-in table answers for the models it knows; for the rest the
// rate is read off the shape of the pricing itself.
//
// Anthropic is the only vendor that sells the longer life, and it prices the
// pair against base input exactly: a five-minute write at 1.25x and an hour at
// 2x. So a model charging 1.25x for its write is on that scheme and its hour
// rate is 2x, and a model that charges nothing to write - everyone else - has
// no such option to price.
func longCacheRate(model ModelsDevModel) float64 {
	if rate := builtInLlmPrices[model.Model].CacheWrite1h; rate > 0 {
		return rate
	}
	if model.Input <= 0 || model.CacheWrite <= 0 {
		return 0
	}
	if ratio := model.CacheWrite / model.Input; ratio < 1.245 || ratio > 1.255 {
		return 0
	}
	return model.Input * 2
}

// SyncModelsDevPrices prices the models this machine has actually run. The
// catalogue holds thousands of them and all but a few would be dead weight in a
// table every request is matched against, so the set to price is passed in
// rather than imported wholesale.
//
// A model somebody priced by hand is left as it is - see PutLlmPriceEntries.
func SyncModelsDevPrices(names []string, force bool) (*ModelsDevSync, error) {
	catalogue, err := GetModelsDevCatalogue(force)
	if err != nil {
		return nil, err
	}

	result := &ModelsDevSync{
		Catalogue:  len(catalogue),
		Considered: []string{},
		Updated:    []string{},
		Skipped:    []string{},
		Missing:    []string{},
		SyncedTime: util.GetCurrentTime(),
	}

	// One catalogue entry can price several of the names seen, so the entries
	// are collected by key first and each written once.
	entries := map[string]*LlmPriceEntry{}
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || key == unknownModel || seen[key] {
			continue
		}
		seen[key] = true
		result.Considered = append(result.Considered, key)

		match, ok := matchModelsDev(catalogue, key)
		if !ok {
			result.Missing = append(result.Missing, key)
			continue
		}
		entries[match.Model] = &LlmPriceEntry{
			Model:        match.Model,
			DisplayName:  match.DisplayName,
			Input:        match.Input,
			Output:       match.Output,
			CacheWrite:   match.CacheWrite,
			CacheRead:    match.CacheRead,
			CacheWrite1h: longCacheRate(match),
		}
	}

	batch := make([]*LlmPriceEntry, 0, len(entries))
	for _, entry := range entries {
		batch = append(batch, entry)
	}
	written, skipped, err := PutLlmPriceEntries(batch, LlmPriceSourceModelsDev)
	if err != nil {
		return nil, err
	}

	sort.Strings(result.Considered)
	sort.Strings(result.Missing)
	result.Updated, result.Skipped = written, skipped
	return result, nil
}
