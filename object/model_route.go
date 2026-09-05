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

// Model routing: what the gateway actually asks for when a client names a
// model, and what it steps down to when that cannot answer. An agent picks its
// own model names - Claude Code sends claude-haiku-4-5 for its background work
// whatever it is configured with - so without a rule the only choice the
// gateway has is the provider's first model, at the provider's first price.

package object

import (
	"fmt"
	"sort"
	"strings"

	"github.com/apache/casbin-gateway/util"
)

const (
	maxRouteTargets    = 8
	maxRouteNameChars  = 100
	maxRouteMatchChars = 200
	// maxRouteAttempts bounds one request: a rule of eight steps against a
	// dozen providers must not turn one client request into a hundred.
	maxRouteAttempts = 32
)

// RouteTarget is one rung of a route: the model to ask for, and optionally the
// only provider allowed to answer it. An empty Model keeps the name the client
// sent, which is how a rule pins a model to a provider without renaming it.
type RouteTarget struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ModelRoute is one rule read on the way in. Targets is a ladder, most
// preferred first: the rungs below the first are the automatic downgrade, taken
// when the rung above is rate-limited, out of quota or down.
//
// A route decides where a request goes, not whether it may go: nothing here
// refuses a model. What an agent is allowed to ask for is a permission, and the
// ladder is filtered through the agent's own permissions before it is walked,
// so a downgrade is never a way around them.
type ModelRoute struct {
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`

	DisplayName string `xorm:"varchar(200)" json:"displayName"`
	// Match is the model the client asks for, with "*" standing for any run of
	// characters: "*haiku*" is one rule for every spelling of a vendor's small
	// model, across the versions it will have next year.
	Match string `xorm:"varchar(200)" json:"match"`
	// Agent limits the rule to the requests of one agent, by its agent id.
	// Empty applies it to every caller.
	Agent   string        `xorm:"varchar(100) index" json:"agent"`
	Targets []RouteTarget `xorm:"mediumtext json" json:"targets"`
	// Sort orders the rules that could all match. Equal values fall back to how
	// specific the pattern is, so an exact model name beats a wildcard without
	// anyone having to number the rules.
	Sort    int  `xorm:"int" json:"sort"`
	Enabled bool `xorm:"bool" json:"enabled"`
}

// RouteAttempt is one thing the proxy tries: a provider, and the model name to
// ask it for. A request is a list of these, walked until one answers.
type RouteAttempt struct {
	Provider *Provider `json:"-"`
	Model    string    `json:"model"`
	// Route names the rule that put this attempt in the list, empty for the
	// attempts the gateway would have made without any rule.
	Route string `json:"route"`
}

func (attempt RouteAttempt) providerId() string {
	return attempt.Provider.GetId()
}

// GetModelRoutes is every rule, disabled ones included, in the order they are
// read in.
func GetModelRoutes() ([]*ModelRoute, error) {
	routes := []*ModelRoute{}
	if ormer == nil || ormer.Engine == nil {
		return routes, nil
	}
	if err := ormer.Engine.Find(&routes); err != nil {
		return nil, err
	}

	sortModelRoutes(routes)
	for _, route := range routes {
		if route.Targets == nil {
			route.Targets = []RouteTarget{}
		}
	}
	return routes, nil
}

func GetModelRoute(name string) (*ModelRoute, error) {
	route := &ModelRoute{Name: name}
	existed, err := ormer.Engine.Get(route)
	if err != nil || !existed {
		return nil, err
	}
	return route, nil
}

func AddModelRoute(route *ModelRoute) error {
	if err := normalizeModelRoute(route); err != nil {
		return err
	}
	existing, err := GetModelRoute(route.Name)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("a routing rule named %s already exists", route.Name)
	}

	route.CreatedTime = util.GetCurrentTime()
	route.UpdatedTime = route.CreatedTime
	_, err = ormer.Engine.Insert(route)
	return err
}

// UpdateModelRoute writes an edited rule back. The name identifies the row and
// is taken from the caller rather than the body, so an edit cannot rename it.
func UpdateModelRoute(name string, route *ModelRoute) error {
	route.Name = name
	if err := normalizeModelRoute(route); err != nil {
		return err
	}
	stored, err := GetModelRoute(name)
	if err != nil {
		return err
	}
	if stored == nil {
		return fmt.Errorf("the routing rule %s does not exist", name)
	}

	route.CreatedTime = stored.CreatedTime
	route.UpdatedTime = util.GetCurrentTime()
	_, err = ormer.Engine.ID(name).AllCols().Update(route)
	return err
}

func DeleteModelRoute(name string) error {
	_, err := ormer.Engine.ID(name).Delete(&ModelRoute{})
	return err
}

// normalizeModelRoute fills in what a rule may leave out and refuses what it
// cannot mean. A named provider is resolved here so a typo fails at the form
// rather than on the next relayed request.
func normalizeModelRoute(route *ModelRoute) error {
	route.Name = strings.TrimSpace(route.Name)
	route.Match = strings.TrimSpace(route.Match)
	route.Agent = strings.TrimSpace(route.Agent)

	if route.Name == "" || len(route.Name) > maxRouteNameChars {
		return fmt.Errorf("the routing rule needs a name of at most %d characters", maxRouteNameChars)
	}
	if route.Match == "" || len(route.Match) > maxRouteMatchChars {
		return fmt.Errorf("the routing rule needs a model pattern of at most %d characters", maxRouteMatchChars)
	}
	if route.DisplayName == "" {
		route.DisplayName = route.Name
	}

	targets := []RouteTarget{}
	for _, target := range route.Targets {
		target.Provider = strings.TrimSpace(target.Provider)
		target.Model = strings.TrimSpace(target.Model)
		if target.Provider == "" && target.Model == "" {
			continue
		}
		if len(target.Model) > maxProviderModelChars {
			return fmt.Errorf("model name is too long: %s", target.Model)
		}
		if target.Provider != "" {
			provider, err := getProviderById(target.Provider)
			if err != nil {
				return err
			}
			if provider == nil {
				return fmt.Errorf("the provider does not exist: %s", target.Provider)
			}
		}
		targets = append(targets, target)
	}
	if len(targets) == 0 {
		return fmt.Errorf("the routing rule %s names nothing to route to", route.Name)
	}
	if len(targets) > maxRouteTargets {
		return fmt.Errorf("too many steps: %d, at most %d are allowed", len(targets), maxRouteTargets)
	}
	route.Targets = targets
	return nil
}

// sortModelRoutes puts the rules in the order they are tried: the operator's
// own numbering first, then the more specific pattern, then the rule tied to an
// agent, then the name so the list never reshuffles on its own.
func sortModelRoutes(routes []*ModelRoute) {
	sort.SliceStable(routes, func(i, j int) bool {
		left, right := routes[i], routes[j]
		if left.Sort != right.Sort {
			return left.Sort < right.Sort
		}
		leftScore, rightScore := patternSpecificity(left.Match), patternSpecificity(right.Match)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if (left.Agent != "") != (right.Agent != "") {
			return left.Agent != ""
		}
		return left.Name < right.Name
	})
}

// patternSpecificity is how much of a pattern is not a wildcard. An exact model
// name is the most specific thing a rule can say, so it beats "claude-*"
// without the operator having to order the two by hand.
func patternSpecificity(pattern string) int {
	if strings.Contains(pattern, "*") {
		return len(pattern) - strings.Count(pattern, "*") - maxRouteMatchChars
	}
	return len(pattern)
}

// MatchModelPattern reports whether a model name is covered by a pattern, in
// which "*" stands for any run of characters. The comparison ignores case: the
// same model reaches the gateway spelled both ways depending on the client.
func MatchModelPattern(pattern string, model string) bool {
	pattern, model = strings.ToLower(pattern), strings.ToLower(model)
	if !strings.Contains(pattern, "*") {
		return pattern == model
	}

	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(model, parts[0]) {
		return false
	}
	model = model[len(parts[0]):]

	last := parts[len(parts)-1]
	for _, part := range parts[1 : len(parts)-1] {
		index := strings.Index(model, part)
		if index < 0 {
			return false
		}
		model = model[index+len(part):]
	}
	return strings.HasSuffix(model, last)
}

// MatchModelRoute is the rule a request is routed by, or nil when none covers
// it. A rule tied to this agent is preferred over a general one of the same
// specificity, see sortModelRoutes.
func MatchModelRoute(agentId string, model string) (*ModelRoute, error) {
	routes, err := GetModelRoutes()
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		if route.Agent != "" && route.Agent != agentId {
			continue
		}
		if MatchModelPattern(route.Match, model) {
			return route, nil
		}
	}
	return nil, nil
}

// PlanModelRoute is the list of attempts for a request that named a model and
// no agent. The matching rule's ladder comes first; what the gateway would have
// done without any rule is appended below it, so a rule whose providers are all
// gone leaves the request working rather than dead.
func PlanModelRoute(model string) ([]RouteAttempt, error) {
	route, err := MatchModelRoute("", model)
	if err != nil {
		return nil, err
	}

	attempts := []RouteAttempt{}
	if route != nil {
		// A rung that names no provider is resolved against everything in
		// rotation, which is what a request with no agent behind it may use.
		pool, poolErr := listEnabledProviders()
		if poolErr != nil {
			return nil, poolErr
		}
		decryptProviders(pool)
		attempts = planRouteTargets(route, model, pool)
	}

	providers, err := GetProvidersByModel(model)
	if err != nil {
		if len(attempts) > 0 {
			// The rule already says where this goes, which is the whole point
			// of writing one for a model no provider lists.
			return attempts, nil
		}
		return nil, err
	}
	return appendAttempts(attempts, defaultAttempts(providers, model)), nil
}

// PlanAgentRoute is the same for a request that arrived on an agent's own
// endpoint. The chain the agent is bound to is the pool an open rung is
// resolved against; a rung naming a provider outright is an explicit
// instruction, taken as written and still held to the agent's permissions by
// the caller.
func PlanAgentRoute(agentId string, model string, chain []*Provider) ([]RouteAttempt, error) {
	route, err := MatchModelRoute(agentId, model)
	if err != nil {
		return nil, err
	}

	attempts := []RouteAttempt{}
	if route != nil {
		attempts = planRouteTargets(route, model, chain)
	}
	return appendAttempts(attempts, defaultAttempts(chain, model)), nil
}

// defaultAttempts is the plan with no rule in it: every provider in turn, each
// asked for the closest model it serves to the one the client named.
func defaultAttempts(providers []*Provider, model string) []RouteAttempt {
	attempts := make([]RouteAttempt, 0, len(providers))
	for _, provider := range providers {
		attempts = append(attempts, RouteAttempt{Provider: provider, Model: ProviderModel(provider, model)})
	}
	return attempts
}

// planRouteTargets walks a rule's ladder into attempts. A rung nothing can
// serve is skipped rather than failing the request: the rungs below it are what
// it is there for.
func planRouteTargets(route *ModelRoute, requested string, pool []*Provider) []RouteAttempt {
	attempts := []RouteAttempt{}
	for _, target := range route.Targets {
		model := target.Model
		if model == "" {
			model = requested
		}

		for _, provider := range routeTargetProviders(target, model, pool) {
			attempts = appendAttempts(attempts, []RouteAttempt{{
				Provider: provider,
				Model:    ProviderModel(provider, model),
				Route:    route.Name,
			}})
		}
	}
	return attempts
}

// routeTargetProviders is who may answer one rung. A named provider is taken as
// written, wherever it sits; an unnamed one has to name the model itself, and
// is looked for in the pool the plan was built against.
func routeTargetProviders(target RouteTarget, model string, pool []*Provider) []*Provider {
	if target.Provider != "" {
		provider, err := getProviderById(target.Provider)
		if err != nil || provider == nil || provider.Status != "enabled" {
			// A rule outliving the provider it names is why the ladder has
			// rungs below this one.
			return nil
		}
		return []*Provider{provider}
	}

	serving := []*Provider{}
	for _, provider := range pool {
		if ProviderServes(provider, model) {
			serving = append(serving, provider)
		}
	}
	return serving
}

// appendAttempts adds the attempts that are not in the list yet. The same
// provider asked for the same model twice is one attempt, however many rules
// and chains put it there.
func appendAttempts(attempts []RouteAttempt, more []RouteAttempt) []RouteAttempt {
	seen := map[string]bool{}
	for _, attempt := range attempts {
		seen[attempt.providerId()+"|"+attempt.Model] = true
	}

	for _, attempt := range more {
		key := attempt.providerId() + "|" + attempt.Model
		if seen[key] || len(attempts) >= maxRouteAttempts {
			continue
		}
		seen[key] = true
		attempts = append(attempts, attempt)
	}
	return attempts
}

// SortAttemptsByHealth puts the attempts whose provider is inside its failure
// cooldown last, keeping the ladder's order among the rest. A dead upstream
// stops costing every request the time it takes to time out, and a downgrade is
// still reached only after what is above it has been tried.
func SortAttemptsByHealth(attempts []RouteAttempt) []RouteAttempt {
	ready := make([]RouteAttempt, 0, len(attempts))
	suspended := []RouteAttempt{}
	for _, attempt := range attempts {
		if IsProviderSuspended(attempt.providerId()) {
			suspended = append(suspended, attempt)
			continue
		}
		ready = append(ready, attempt)
	}
	return append(ready, suspended...)
}

// ModelsWithRoutes adds the exact model names the rules answer for to the ones
// the providers list. They belong on the list a client fills its picker from: a
// name the gateway would happily route is one the client can ask for, whether
// or not any provider lists it. A wildcard rule adds nothing, since it names no
// model of its own.
func ModelsWithRoutes(models []string, agentId string) []string {
	routes, err := GetModelRoutes()
	if err != nil {
		return models
	}

	seen := map[string]bool{}
	for _, model := range models {
		seen[strings.ToLower(model)] = true
	}
	for _, route := range routes {
		if !route.Enabled || strings.Contains(route.Match, "*") {
			continue
		}
		if route.Agent != "" && route.Agent != agentId {
			continue
		}
		if !seen[strings.ToLower(route.Match)] {
			seen[strings.ToLower(route.Match)] = true
			models = append(models, route.Match)
		}
	}
	return models
}
