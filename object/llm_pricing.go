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
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/apache/casbin-gateway/conf"
	"github.com/beego/beego"
)

// LlmPrice is what one million tokens of each kind costs, in US dollars. APIs
// that discount cached input instead of charging for the write leave
// CacheWrite at zero.
type LlmPrice struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cacheWrite"`
	CacheRead  float64 `json:"cacheRead"`
	// CacheWrite1h is what a cache written for an hour costs instead, where the
	// vendor sells the longer life at a higher rate. Zero means there is no such
	// option and every write is priced at CacheWrite.
	CacheWrite1h float64 `json:"cacheWrite1h"`
}

// builtInLlmPrices are published list prices, keyed by the part of a model name that
// identifies the model. They go stale, so llmPricingFile overrides them.
//
// Anthropic list prices are from platform.claude.com/docs/en/about-claude/pricing
// and OpenAI's from developers.openai.com/api/docs/pricing, both read on
// 2026-09-02. A key is matched as a substring, longest first, so a family entry
// covers the dated and vendor-prefixed shapes of its own name and the
// generations that kept its price.
var builtInLlmPrices = map[string]LlmPrice{
	"claude-fable-5":    {10, 50, 12.5, 1, 20},
	"claude-fable-5-1":  {10, 50, 12.5, 0.25, 20},
	"claude-mythos-5":   {10, 50, 12.5, 1, 20},
	"claude-mythos-5-1": {10, 50, 12.5, 0.25, 20},
	"claude-opus-5":     {5, 25, 6.25, 0.5, 10},
	// Opus dropped to a third of its price at 4.5; the two older ones kept theirs.
	"claude-opus-4-5":   {5, 25, 6.25, 0.5, 10},
	"claude-opus-4-6":   {5, 25, 6.25, 0.5, 10},
	"claude-opus-4-7":   {5, 25, 6.25, 0.5, 10},
	"claude-opus-4-8":   {5, 25, 6.25, 0.5, 10},
	"claude-opus-4":     {15, 75, 18.75, 1.5, 30},
	"claude-3-opus":     {15, 75, 18.75, 1.5, 30},
	"claude-sonnet-5":   {2, 10, 2.5, 0.2, 4},
	"claude-sonnet-4":   {3, 15, 3.75, 0.3, 6},
	"claude-3-7-sonnet": {3, 15, 3.75, 0.3, 6},
	"claude-3-5-sonnet": {3, 15, 3.75, 0.3, 6},
	"claude-haiku-4-5":  {1, 5, 1.25, 0.1, 2},
	"claude-3-5-haiku":  {0.8, 4, 1, 0.08, 1.6},
	"claude-3-haiku":    {0.25, 1.25, 0.3, 0.03, 0.6},

	"gpt-5":         {1.25, 10, 0, 0.125, 0},
	"gpt-5-mini":    {0.25, 2, 0, 0.025, 0},
	"gpt-5-nano":    {0.05, 0.4, 0, 0.005, 0},
	"gpt-5-pro":     {15, 120, 0, 0, 0},
	"gpt-5.1":       {1.25, 10, 0, 0.125, 0},
	"gpt-5.2":       {1.75, 14, 0, 0.175, 0},
	"gpt-5.2-pro":   {21, 168, 0, 0, 0},
	"gpt-5.4":       {2.5, 15, 0, 0.25, 0},
	"gpt-5.4-mini":  {0.75, 4.5, 0, 0.075, 0},
	"gpt-5.4-nano":  {0.2, 1.25, 0, 0.02, 0},
	"gpt-5.4-pro":   {30, 180, 0, 0, 0},
	"gpt-5.5":       {5, 30, 0, 0.5, 0},
	"gpt-5.5-pro":   {30, 180, 0, 0, 0},
	"gpt-5.6-sol":   {4, 20, 0, 0.4, 0},
	"gpt-5.6-terra": {2, 12, 0, 0.2, 0},
	"gpt-5.6-luna":  {0.2, 1.2, 0, 0.02, 0},

	"gpt-4o":       {2.5, 10, 0, 1.25, 0},
	"gpt-4o-mini":  {0.15, 0.6, 0, 0.075, 0},
	"gpt-4.1":      {2, 8, 0, 0.5, 0},
	"gpt-4.1-mini": {0.4, 1.6, 0, 0.1, 0},
	"gpt-4.1-nano": {0.1, 0.4, 0, 0.025, 0},
	"o1":           {15, 60, 0, 7.5, 0},
	"o3":           {2, 8, 0, 0.5, 0},
	"o3-pro":       {20, 80, 0, 0, 0},
	"o3-mini":      {1.1, 4.4, 0, 0.55, 0},
	"o4-mini":      {1.1, 4.4, 0, 0.275, 0},

	"deepseek-chat":     {0.27, 1.1, 0, 0.07, 0},
	"deepseek-reasoner": {0.55, 2.19, 0, 0.14, 0},
}

var (
	pricingMutex  sync.RWMutex
	pricingLoaded bool
	pricingKeys   []string
	llmPrices     map[string]LlmPrice
	// pricingOrigins says which layer each key in effect came from, which the
	// Pricing page shows and "Reset" reads to know what it would fall back to.
	pricingOrigins map[string]string
	pricingNames   map[string]string
)

// ReloadLlmPrices re-reads the override file and the stored overrides, so a
// price edited on the Pricing page, or "llmPricingFile" changed on the Settings
// page, costs the next request rather than the next restart.
func ReloadLlmPrices() {
	pricingMutex.Lock()
	defer pricingMutex.Unlock()

	loadLlmPrices()
}

func ensureLlmPrices() {
	pricingMutex.RLock()
	loaded := pricingLoaded
	pricingMutex.RUnlock()
	if loaded {
		return
	}

	pricingMutex.Lock()
	defer pricingMutex.Unlock()
	if !pricingLoaded {
		loadLlmPrices()
	}
}

// loadLlmPrices merges the layers over the built-in table, longest key first so
// a specific model wins over the family it belongs to. It rebuilds the table
// from the built-in one, so a price dropped from a layer is dropped here too.
// The caller holds the write lock.
//
// The layers, in the order each overrides the one before it: the built-in list
// prices, the JSON file named by "llmPricingFile", and the rows stored by the
// Pricing page and by a models.dev sync. The stored rows are last because they
// are the only layer somebody edited from inside Gateway, and an edit there has
// to be the one that takes effect.
func loadLlmPrices() {
	prices := make(map[string]LlmPrice, len(builtInLlmPrices))
	origins := make(map[string]string, len(builtInLlmPrices))
	names := map[string]string{}
	for model, price := range builtInLlmPrices {
		prices[model] = price
		origins[model] = LlmPriceSourceBuiltIn
	}

	path := conf.GetLlmPricingFile()
	if path != "" {
		if data, err := os.ReadFile(path); err != nil {
			if !os.IsNotExist(err) {
				beego.Error("LLM pricing file could not be read:", err)
			}
		} else {
			overrides := map[string]LlmPrice{}
			if err := json.Unmarshal(data, &overrides); err != nil {
				beego.Error("LLM pricing file is not valid JSON:", err)
			} else {
				for model, price := range overrides {
					key := strings.ToLower(model)
					prices[key] = price
					origins[key] = LlmPriceSourceFile
				}
			}
		}
	}

	// A database that is not open yet leaves the stored layer out rather than
	// failing the lookup: the built-in prices are still better than none.
	entries, err := GetLlmPriceEntries()
	if err != nil {
		beego.Error("stored LLM prices could not be read:", err)
	}
	for _, entry := range entries {
		prices[entry.Model] = entry.price()
		origins[entry.Model] = entry.Source
		if entry.DisplayName != "" {
			names[entry.Model] = entry.DisplayName
		}
	}

	keys := make([]string, 0, len(prices))
	for key := range prices {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	llmPrices, pricingKeys, pricingLoaded = prices, keys, true
	pricingOrigins, pricingNames = origins, names
}

// GetLlmPriceViews is every price in effect, in the order the page lists them:
// by model name, because a price table is looked up by name rather than read
// top to bottom.
func GetLlmPriceViews() []LlmPriceView {
	ensureLlmPrices()

	pricingMutex.RLock()
	defer pricingMutex.RUnlock()

	views := make([]LlmPriceView, 0, len(llmPrices))
	for model, price := range llmPrices {
		_, builtIn := builtInLlmPrices[model]
		source := pricingOrigins[model]
		views = append(views, LlmPriceView{
			Model:       model,
			DisplayName: pricingNames[model],
			LlmPrice:    price,
			Source:      source,
			Overridden:  builtIn && source != LlmPriceSourceBuiltIn,
		})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Model < views[j].Model })
	return views
}

// GetLlmPrice matches a model name in any of the shapes it arrives in, e.g.
// "claude-sonnet-4-20250514" or "us.anthropic.claude-sonnet-4-v1:0". It reports
// false when nothing matches, so an unpriced model is not costed at zero.
func GetLlmPrice(model string) (LlmPrice, bool) {
	ensureLlmPrices()

	pricingMutex.RLock()
	defer pricingMutex.RUnlock()

	name := strings.ToLower(strings.TrimSpace(model))
	for _, key := range pricingKeys {
		if strings.Contains(name, key) {
			return llmPrices[key], true
		}
	}
	return LlmPrice{}, false
}

// GetLlmCost is what one recorded request cost, in US dollars.
func GetLlmCost(model string, promptTokens int, completionTokens int, cacheWriteTokens int, cacheReadTokens int) (float64, bool) {
	return GetLlmLongCacheCost(model, promptTokens, completionTokens, cacheWriteTokens, 0, cacheReadTokens)
}

// GetLlmLongCacheCost is GetLlmCost where part of the cache was written to live
// an hour rather than five minutes, which Anthropic charges 2x base input for
// against 1.25x. longCacheTokens is the share of cacheWriteTokens written that
// way, not an amount on top of it; a transcript says which was used, while a
// relayed request does not, so that path passes zero and pays the shorter rate.
func GetLlmLongCacheCost(model string, promptTokens int, completionTokens int, cacheWriteTokens int, longCacheTokens int, cacheReadTokens int) (float64, bool) {
	price, ok := GetLlmPrice(model)
	if !ok {
		return 0, false
	}

	longCacheRate := price.CacheWrite1h
	if longCacheRate == 0 {
		longCacheRate = price.CacheWrite
	}
	if longCacheTokens > cacheWriteTokens {
		longCacheTokens = cacheWriteTokens
	}

	cost := float64(promptTokens)*price.Input +
		float64(completionTokens)*price.Output +
		float64(cacheWriteTokens-longCacheTokens)*price.CacheWrite +
		float64(longCacheTokens)*longCacheRate +
		float64(cacheReadTokens)*price.CacheRead
	return cost / 1e6, true
}
