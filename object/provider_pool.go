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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// How an agent's providers - the bound one and its fallbacks - are taken in
// turn. With several accounts of one plan among them, they are a pool.
const (
	// PoolOrder tries them as listed, the bound provider first.
	PoolOrder = "order"
	// PoolSmart spends first the plan whose short window resets soonest, since
	// what it has left is lost at that reset, and an API key only after every
	// plan.
	PoolSmart = "smart"
	// PoolRotate starts each new conversation on the next provider.
	PoolRotate = "rotate"
	// PoolUsage starts on the plan with the most left.
	PoolUsage = "usage"
)

var poolStrategies = []string{"", PoolOrder, PoolSmart, PoolRotate, PoolUsage}

func validatePoolStrategy(strategy string) error {
	if !containsString(poolStrategies, strategy) {
		return fmt.Errorf("invalid pool strategy: %s", strategy)
	}
	return nil
}

var poolTurns = struct {
	sync.Mutex
	next map[string]int
}{next: map[string]int{}}

// OrderPool puts an agent's providers in the order its strategy tries them.
// advance moves a rotation on to the next provider, which only a request that
// is really sent should do.
func OrderPool(agentId string, strategy string, providers []*Provider, advance bool) []*Provider {
	if len(providers) < 2 {
		return providers
	}

	ordered := append([]*Provider{}, providers...)
	switch strategy {
	case PoolRotate:
		poolTurns.Lock()
		turn := poolTurns.next[agentId] % len(ordered)
		if advance {
			poolTurns.next[agentId] = turn + 1
		}
		poolTurns.Unlock()
		return append(ordered[turn:], ordered[:turn]...)
	case PoolSmart, PoolUsage:
		now := time.Now()
		standings := make(map[string]poolStanding, len(ordered))
		for _, provider := range ordered {
			standings[provider.GetId()] = standingOf(provider, now)
		}
		sort.SliceStable(ordered, func(i, j int) bool {
			left, right := standings[ordered[i].GetId()], standings[ordered[j].GetId()]
			if left.class != right.class {
				return left.class < right.class
			}
			if left.class != poolKnown {
				return false
			}
			if strategy == PoolSmart && !left.reset.Equal(right.reset) {
				return left.reset.Before(right.reset)
			}
			return left.remaining > right.remaining
		})
	}
	return ordered
}

const (
	poolKnown = iota
	poolUnknown
	poolKey
)

// poolStanding is what a strategy compares providers by: a plan whose limits
// are known, a plan whose limits are not, or a key with no plan behind it.
type poolStanding struct {
	class int
	// reset is when the short window resets, far off for one that has not
	// started: its allowance is not lost by waiting.
	reset time.Time
	// remaining is the least left of any window, since the tightest one is
	// what refuses the next request.
	remaining float64
}

func standingOf(provider *Provider, now time.Time) poolStanding {
	if !UsesSubscription(provider) {
		return poolStanding{class: poolKey}
	}

	windows := cachedWindows(provider.GetId())
	standing := poolStanding{class: poolUnknown, reset: now.Add(365 * 24 * time.Hour), remaining: 100}
	for _, window := range windows {
		if window.Name != WindowShort && window.Name != WindowWeek {
			continue
		}
		standing.class = poolKnown
		if window.RemainingPercent < standing.remaining {
			standing.remaining = window.RemainingPercent
		}
		if at := window.reset(); window.Name == WindowShort && at.After(now) {
			standing.reset = at
		}
	}
	return standing
}

// stickyIdle is how long a conversation keeps the provider that answered it
// after its last turn. Prompt caches outlive a short pause and not much more,
// so past this a conversation is free to go where the strategy says.
const stickyIdle = 15 * time.Minute

var stickies = struct {
	sync.Mutex
	byKey map[string]stickyEntry
}{byKey: map[string]stickyEntry{}}

type stickyEntry struct {
	provider string
	seen     time.Time
}

// ConversationKey names the conversation a request belongs to, empty when it
// cannot be told. An agent that says which session it is in is taken at its
// word; otherwise a conversation is known by how it began, which every later
// turn repeats.
func ConversationKey(header http.Header, body []byte) string {
	for _, name := range []string{"X-Claude-Code-Session-Id", "Session_id", "Session-Id", "X-Session-Id", "Thread-Id", "Conversation_id"} {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return "session:" + value
		}
	}

	var fields struct {
		PromptCacheKey string `json:"prompt_cache_key"`
		Metadata       struct {
			UserId string `json:"user_id"`
		} `json:"metadata"`
		Messages     []json.RawMessage `json:"messages"`
		Input        json.RawMessage   `json:"input"`
		Contents     []json.RawMessage `json:"contents"`
		System       json.RawMessage   `json:"system"`
		Instructions string            `json:"instructions"`
	}
	if json.Unmarshal(body, &fields) != nil {
		return ""
	}
	if fields.PromptCacheKey != "" {
		return "cache:" + fields.PromptCacheKey
	}
	// Claude Code puts its session in the user id; other clients put a user
	// there, which is still one conversation stream per person.
	if fields.Metadata.UserId != "" {
		return "user:" + fields.Metadata.UserId
	}

	first := []byte(nil)
	switch {
	case len(fields.Messages) > 0:
		first = firstNonSystem(fields.Messages)
	case len(fields.Contents) > 0:
		first = fields.Contents[0]
	case len(fields.Input) > 0:
		var items []json.RawMessage
		if json.Unmarshal(fields.Input, &items) == nil && len(items) > 0 {
			first = items[0]
		} else {
			first = fields.Input
		}
	}
	if len(first) == 0 {
		return ""
	}

	sum := sha256.New()
	sum.Write(fields.System)
	sum.Write([]byte(fields.Instructions))
	sum.Write(first)
	return "start:" + hex.EncodeToString(sum.Sum(nil)[:12])
}

func firstNonSystem(messages []json.RawMessage) []byte {
	for _, message := range messages {
		var role struct {
			Role string `json:"role"`
		}
		if json.Unmarshal(message, &role) == nil && (role.Role == "system" || role.Role == "developer") {
			continue
		}
		return message
	}
	return nil
}

// PreferSticky moves the provider that answered this conversation last to the
// front of the stretch of attempts it belongs to, so its prompt cache is spent
// rather than paid for again elsewhere. A stretch is the attempts one routing
// rule put there, or the ruleless ones below them: a rule is the operator
// saying where a model goes, and a conversation that has been answered by some
// other provider before must not step over it. A provider that is cooling down
// or out of its plan is left where it is.
func PreferSticky(key string, attempts []RouteAttempt) []RouteAttempt {
	if key == "" || len(attempts) < 2 {
		return attempts
	}

	stickies.Lock()
	entry, ok := stickies.byKey[key]
	stickies.Unlock()
	if !ok || time.Since(entry.seen) > stickyIdle {
		return attempts
	}
	if IsProviderSuspended(entry.provider) || IsSubscriptionSpent(entry.provider) {
		return attempts
	}

	ordered := make([]RouteAttempt, 0, len(attempts))
	for start := 0; start < len(attempts); {
		end := start + 1
		for end < len(attempts) && attempts[end].Route == attempts[start].Route {
			end++
		}
		ordered = appendSticky(ordered, attempts[start:end], entry.provider)
		start = end
	}
	return ordered
}

// appendSticky adds one stretch with the remembered provider moved to its
// front, and the rest in the order it was given.
func appendSticky(dst []RouteAttempt, stretch []RouteAttempt, providerId string) []RouteAttempt {
	promoted := -1
	for i, attempt := range stretch {
		if attempt.providerId() == providerId {
			promoted = i
			break
		}
	}
	if promoted < 0 {
		return append(dst, stretch...)
	}

	dst = append(dst, stretch[promoted])
	for i, attempt := range stretch {
		if i != promoted {
			dst = append(dst, attempt)
		}
	}
	return dst
}

// RememberSticky records which provider answered a conversation.
func RememberSticky(key string, providerId string) {
	if key == "" {
		return
	}

	now := time.Now()
	stickies.Lock()
	defer stickies.Unlock()
	stickies.byKey[key] = stickyEntry{provider: providerId, seen: now}
	if len(stickies.byKey) > 4096 {
		for name, entry := range stickies.byKey {
			if now.Sub(entry.seen) > stickyIdle {
				delete(stickies.byKey, name)
			}
		}
	}
}
