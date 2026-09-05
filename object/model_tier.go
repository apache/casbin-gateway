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

import "strings"

// The size class a model name says it belongs to. It is what an automatic
// downgrade is decided on: an agent asks for the small model of one vendor for
// its background work — Claude Code sends claude-haiku-4-5 for that — and
// sending it whichever model the provider happens to list first bills the
// cheapest traffic at the most expensive rate.
const (
	TierSmall    = "small"
	TierStandard = "standard"
	TierLarge    = "large"
)

// The words the vendors spell their size classes with. They are matched as
// whole tokens of the model name rather than as substrings, so MiniMax-M1 is
// not read as a "mini" and gpt-4o-mini is.
var (
	smallTierWords = []string{"haiku", "mini", "flash", "lite", "small", "tiny", "nano", "micro", "instant", "air"}
	largeTierWords = []string{"opus", "ultra", "max", "pro", "plus", "large", "heavy", "reasoner", "thinking"}
)

// ModelTier is the size class a model name reads as. It is a guess made from
// the name alone, which is all a gateway has: the vendors publish no field for
// it and a relay renames their models anyway.
func ModelTier(model string) string {
	for _, token := range modelNameTokens(model) {
		if containsString(smallTierWords, token) {
			return TierSmall
		}
		if containsString(largeTierWords, token) {
			return TierLarge
		}
	}
	return TierStandard
}

// modelNameTokens splits a model name on everything that is not a letter or a
// digit, which is how every vendor separates the parts of one.
func modelNameTokens(model string) []string {
	return strings.FieldsFunc(strings.ToLower(model), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
}

// tierPreference is the order the tiers are searched in for a model to send
// instead of the one that was asked for. A request for the small model settles
// for a bigger one only after nothing small is offered, so background traffic
// stays cheap; a request for a big one steps down rather than up.
var tierPreference = map[string][]string{
	TierSmall:    {TierSmall, TierStandard, TierLarge},
	TierStandard: {TierStandard, TierLarge, TierSmall},
	TierLarge:    {TierLarge, TierStandard, TierSmall},
}

// PickProviderModel is the model to ask a provider for when the client named
// one. An exact match is always it; otherwise the closest size class the
// provider offers is taken, and the provider's own first model is the last
// resort. The provider's order is kept inside a class, since a vendor lists its
// newest model first.
func PickProviderModel(models []string, requested string) string {
	if len(models) == 0 {
		return requested
	}

	for _, candidate := range models {
		if candidate == requested {
			return requested
		}
	}
	for _, candidate := range models {
		if strings.EqualFold(candidate, requested) {
			return candidate
		}
	}

	for _, tier := range tierPreference[ModelTier(requested)] {
		for _, candidate := range models {
			if ModelTier(candidate) == tier {
				return candidate
			}
		}
	}
	return models[0]
}

// ProviderServes reports whether a provider names the model itself. A provider
// that names none serves whatever arrives, which is the client-auth case: the
// account behind the caller's own credentials decides, not this table.
func ProviderServes(provider *Provider, model string) bool {
	if len(provider.Models) == 0 {
		return UsesClientAuth(provider)
	}
	for _, candidate := range provider.Models {
		if strings.EqualFold(candidate, model) {
			return true
		}
	}
	return false
}
