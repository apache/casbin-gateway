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
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agenthistory"
	"github.com/apache/casbin-gateway/agenthome"
)

// HistoricalSessions reads the transcripts of every account with an agent on
// this machine. A home Gateway cannot open is skipped: the caller lists what it
// can read, and says nothing about the rest.
func HistoricalSessions(agentId string) []agenthistory.Session {
	installations, err := agent.Scan(false)
	if err != nil {
		return nil
	}

	sessions := []agenthistory.Session{}
	scanned := map[string]bool{}
	for _, installation := range installations {
		home, err := agenthome.Resolve(installation.Owner)
		if err != nil || scanned[home] {
			continue
		}
		scanned[home] = true
		for _, session := range agenthistory.Scan(home) {
			if agentId == "" || strings.EqualFold(session.Agent, agentId) {
				sessions = append(sessions, session)
			}
		}
	}
	return sessions
}

// AgentUsageStat is what one agent, one model or one day spent. Which of the
// three a stat counts is decided by the list it is in; Name is that key.
type AgentUsageStat struct {
	Name             string  `json:"name"`
	Requests         int64   `json:"requests"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`
	CacheReadTokens  int64   `json:"cacheReadTokens"`
	CacheWriteTokens int64   `json:"cacheWriteTokens"`
	ReasoningTokens  int64   `json:"reasoningTokens"`
	TotalTokens      int64   `json:"totalTokens"`
	Cost             float64 `json:"cost"`
	// Unpriced counts the requests left out of Cost because no list price
	// matched their model, so a total is never quietly short.
	Unpriced int64 `json:"unpriced"`
	// LastTime and LastModel are filled for agents, which is what the page
	// listing every agent shows next to its totals.
	LastTime  string `json:"lastTime"`
	LastModel string `json:"lastModel"`
}

// AgentUsage is what the transcripts on this machine add up to.
type AgentUsage struct {
	Totals AgentUsageStat   `json:"totals"`
	Agents []AgentUsageStat `json:"agents"`
	Models []AgentUsageStat `json:"models"`
	// Days is ordered oldest first, which is the order a trend is drawn in.
	Days []AgentUsageStat `json:"days"`
	// Sessions is how many transcripts the totals were read from.
	Sessions int `json:"sessions"`
	// Since repeats the window the totals cover, so a page cannot label them
	// with a range they were not asked for.
	Since string `json:"since"`
}

// unknownModel stands in for a turn whose transcript never named a model. It is
// priced like any other unknown one, which is to say not at all.
const unknownModel = "unknown"

// GetAgentUsage totals what the agents spent, read from the transcripts they
// keep themselves. LLM Records only ever holds what went through Gateway, so
// this is the account of an agent talking to its vendor directly - and the one
// that has something to show before anything is routed at all.
//
// since bounds the window to the calendar days on or after it, formatted
// "2006-01-02". An empty since covers every transcript that was scanned.
func GetAgentUsage(sessions []agenthistory.Session, since string) *AgentUsage {
	usage := &AgentUsage{
		Agents: []AgentUsageStat{},
		Models: []AgentUsageStat{},
		Days:   []AgentUsageStat{},
		Since:  since,
	}
	agents := map[string]*AgentUsageStat{}
	models := map[string]*AgentUsageStat{}
	days := map[string]*AgentUsageStat{}

	for _, session := range sessions {
		counted := false
		for _, bucket := range session.Usage {
			if since != "" && bucket.Day < since {
				continue
			}
			counted = true

			model := bucket.Model
			if strings.TrimSpace(model) == "" {
				model = unknownModel
			}
			cost, priced := GetLlmLongCacheCost(model, bucket.PromptTokens, bucket.CompletionTokens,
				bucket.CacheWriteTokens, bucket.LongCacheTokens, bucket.CacheReadTokens)

			for _, stat := range []*AgentUsageStat{
				&usage.Totals,
				statOf(agents, session.Agent),
				statOf(models, model),
				statOf(days, bucket.Day),
			} {
				stat.add(bucket, cost, priced)
			}

			// An agent's last model is the newest one it was seen on, and the
			// buckets of a session are ordered oldest first.
			agent := statOf(agents, session.Agent)
			if session.LastTime >= agent.LastTime {
				agent.LastTime = session.LastTime
				agent.LastModel = model
			}
		}
		if counted {
			usage.Sessions++
		}
	}

	usage.Agents = sorted(agents, byCost)
	usage.Models = sorted(models, byCost)
	usage.Days = sorted(days, byName)
	return usage
}

func (stat *AgentUsageStat) add(bucket agenthistory.UsageBucket, cost float64, priced bool) {
	stat.Requests += int64(bucket.Requests)
	stat.PromptTokens += int64(bucket.PromptTokens)
	stat.CompletionTokens += int64(bucket.CompletionTokens)
	stat.CacheReadTokens += int64(bucket.CacheReadTokens)
	stat.CacheWriteTokens += int64(bucket.CacheWriteTokens)
	stat.ReasoningTokens += int64(bucket.ReasoningTokens)
	stat.TotalTokens += int64(bucket.PromptTokens + bucket.CompletionTokens +
		bucket.CacheReadTokens + bucket.CacheWriteTokens)
	if priced {
		stat.Cost += cost
	} else {
		stat.Unpriced += int64(bucket.Requests)
	}
}

func statOf(stats map[string]*AgentUsageStat, name string) *AgentUsageStat {
	stat, found := stats[name]
	if !found {
		stat = &AgentUsageStat{Name: name}
		stats[name] = stat
	}
	return stat
}

// byCost puts the expensive first, falling back to tokens where nothing carries
// a price at all, so the order does not collapse to alphabetical.
func byCost(left, right *AgentUsageStat) bool {
	if left.Cost != right.Cost {
		return left.Cost > right.Cost
	}
	if left.TotalTokens != right.TotalTokens {
		return left.TotalTokens > right.TotalTokens
	}
	return left.Name < right.Name
}

func byName(left, right *AgentUsageStat) bool {
	return left.Name < right.Name
}

func sorted(stats map[string]*AgentUsageStat, less func(left, right *AgentUsageStat) bool) []AgentUsageStat {
	ordered := make([]*AgentUsageStat, 0, len(stats))
	for _, stat := range stats {
		ordered = append(ordered, stat)
	}
	sort.SliceStable(ordered, func(left, right int) bool { return less(ordered[left], ordered[right]) })

	result := make([]AgentUsageStat, 0, len(ordered))
	for _, stat := range ordered {
		result = append(result, *stat)
	}
	return result
}
