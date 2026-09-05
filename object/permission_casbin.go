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

// This file turns the switches of one agent's Permissions card into the casbin
// policy that decides its requests. The switches are what is stored; the policy
// is what is enforced, and the advanced view shows it as it is compiled here.

package object

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

// PermissionModelText is the casbin model every agent is enforced with. The
// effect is casbin's priority effect: the first rule that matches decides, and
// a request no rule matches is denied. Order is what lets one exception stand
// in front of the "everything else" rule behind it.
const PermissionModelText = `[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft

[policy_effect]
e = priority(p.eft) || deny

[matchers]
m = keyMatch(r.sub, p.sub) && keyMatch(r.obj, p.obj) && keyMatch(r.act, p.act)
`

// The objects a request is checked against. A model and a provider are named
// as they are elsewhere in Gateway; a tool is named by the group it falls in,
// see permission_tools.go.
const (
	objectModel    = "model:"
	objectProvider = "provider:"
	objectTool     = "tool:"
)

// actUse is the only action there is: everything a request asks for, it asks to
// use. The column is kept because a hand-written rule may want another one.
const actUse = "use"

// The two casbin effects, as they are written in a policy line.
const (
	effectAllow = "allow"
	effectDeny  = "deny"
)

// AgentGuard decides one agent's requests. It is nil for an agent whose
// permissions are off, which is the case until someone turns them on.
type AgentGuard struct {
	agentId  string
	enforcer *casbin.Enforcer
	// switched is every item this permission actually has a switch for. It is
	// what lets one tool of an MCP server be decided by its own switch rather
	// than by the server's, without a rule for every tool nobody has set.
	switched map[string]bool
}

var (
	guardLock  sync.Mutex
	guardCache = map[string]*AgentGuard{}
)

// LoadAgentGuard builds, or reuses, the enforcer of one agent. It answers nil
// when the agent is unrestricted, so a caller only has to check for that.
func LoadAgentGuard(agentId string) (*AgentGuard, error) {
	permission, err := GetAgentPermission(agentId)
	if err != nil {
		return nil, err
	}
	if !permission.Enabled {
		return nil, nil
	}

	guardLock.Lock()
	defer guardLock.Unlock()

	if guard := guardCache[agentId]; guard != nil {
		return guard, nil
	}

	enforcer, err := newPermissionEnforcer(permission)
	if err != nil {
		return nil, err
	}

	guard := &AgentGuard{agentId: agentId, enforcer: enforcer, switched: permission.switchedItems()}
	guardCache[agentId] = guard
	return guard, nil
}

// dropAgentEnforcer forgets one agent's enforcer, so the next request is
// decided by the permissions that were just saved.
func dropAgentEnforcer(agentId string) {
	guardLock.Lock()
	defer guardLock.Unlock()
	delete(guardCache, agentId)
}

// newPermissionEnforcer builds the casbin enforcer of one agent's policy. The
// policy is held in memory: the rows of AgentPermission are what is persisted,
// and every line below is compiled from them.
func newPermissionEnforcer(permission *AgentPermission) (*casbin.Enforcer, error) {
	permissionModel, err := model.NewModelFromString(PermissionModelText)
	if err != nil {
		return nil, err
	}

	enforcer, err := casbin.NewEnforcer(permissionModel)
	if err != nil {
		return nil, err
	}

	if _, err = enforcer.AddPolicies(permission.PolicyRules()); err != nil {
		return nil, err
	}
	return enforcer, nil
}

// PolicyRules compiles the switches into casbin policy lines, as
// "sub, obj, act, eft". The order is the order they are decided in: the rules
// written by hand come first, then the exceptions, then the rule each group
// falls back on.
func (permission *AgentPermission) PolicyRules() [][]string {
	rules := [][]string{}
	seen := map[string]bool{}
	add := func(object string, effect string) {
		rule := []string{permission.Name, object, actUse, effect}
		key := strings.Join(rule, ",")
		if seen[key] {
			return
		}
		seen[key] = true
		rules = append(rules, rule)
	}

	// A hand-written rule is checked before everything the switches wrote, which
	// is what makes the advanced view able to carve out what they cannot say.
	for _, rule := range permission.Rules {
		parsed, err := parsePolicyRule(rule)
		if err != nil {
			continue
		}
		key := strings.Join(parsed, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		rules = append(rules, parsed)
	}

	appendList(add, objectModel, permission.ModelMode, permission.Models)
	appendList(add, objectProvider, permission.ProviderMode, permission.Providers)
	permission.appendToolRules(add)
	return rules
}

// appendToolRules writes the lines of the switches. A group whose catch-all is
// off is closed with one rule for the whole group, with the items left on
// standing in front of it; any other group only names what it took away.
func (permission *AgentPermission) appendToolRules(add func(object string, effect string)) {
	for _, group := range permission.toolItemsByGroup() {
		denied := 0
		for _, name := range group.items {
			if permission.Tools[name] == false && permission.listed(name) {
				denied++
			}
		}

		closed := permission.Tools[group.name+"/"+otherItem] == false &&
			permission.listed(group.name+"/"+otherItem)
		if !closed && denied < len(group.items) {
			for _, name := range group.items {
				if permission.listed(name) && !permission.Tools[name] {
					add(objectTool+name, effectDeny)
				}
			}
			continue
		}

		for _, name := range group.items {
			if !permission.listed(name) || permission.Tools[name] {
				add(objectTool+name, effectAllow)
			}
		}
		add(objectTool+group.name+"/*", effectDeny)
	}

	// Whatever no group took away, including the tools that fall in none of
	// them, is allowed.
	add(objectTool+"*", effectAllow)
}

// listed reports whether a switch was ever set. An item nobody has touched is
// allowed, so a switch added in a later version does not silently take
// something away from an agent configured before it existed.
// switchedItems is every item somebody has set a switch for.
func (permission *AgentPermission) switchedItems() map[string]bool {
	switched := map[string]bool{}
	for name := range permission.Tools {
		switched[name] = true
	}
	return switched
}

func (permission *AgentPermission) listed(name string) bool {
	_, found := permission.Tools[name]
	return found
}

type toolItemGroup struct {
	name  string
	items []string
}

// toolItemsByGroup is every switch this permission is read against: the
// built-in catalogue, plus whatever was stored beside it, such as the MCP
// servers of one agent.
func (permission *AgentPermission) toolItemsByGroup() []toolItemGroup {
	order := []string{}
	items := map[string][]string{}
	seen := map[string]bool{}

	appendItem := func(name string) {
		group, _, found := strings.Cut(name, "/")
		if !found || seen[name] {
			return
		}
		seen[name] = true
		if _, known := items[group]; !known {
			order = append(order, group)
		}
		items[group] = append(items[group], name)
	}

	for _, name := range CatalogItemNames() {
		appendItem(name)
	}
	stored := []string{}
	for name := range permission.Tools {
		stored = append(stored, name)
	}
	sort.Strings(stored)
	for _, name := range stored {
		appendItem(name)
	}

	groups := []toolItemGroup{}
	for _, name := range order {
		groups = append(groups, toolItemGroup{name: name, items: items[name]})
	}
	return groups
}

// appendList writes the lines of one "all / only these / all but these" choice.
// The exceptions come first: the first rule that matches is the one that
// decides.
func appendList(add func(object string, effect string), prefix string, mode string, values []string) {
	switch mode {
	case ListAllow:
		// An empty allow-list allows nothing, which is what picking that mode
		// and listing nothing asks for.
		for _, value := range values {
			add(prefix+value, effectAllow)
		}
	case ListDeny:
		for _, value := range values {
			add(prefix+value, effectDeny)
		}
		add(prefix+"*", effectAllow)
	default:
		add(prefix+"*", effectAllow)
	}
}

// parsePolicyRule reads a hand-written policy line. The leading "p," of a line
// copied out of a policy file is optional, and so is the effect, which is
// "allow" when it is left out.
func parsePolicyRule(rule string) ([]string, error) {
	fields := []string{}
	for _, field := range strings.Split(rule, ",") {
		fields = append(fields, strings.TrimSpace(field))
	}
	if len(fields) > 0 && fields[0] == "p" {
		fields = fields[1:]
	}

	if len(fields) == 3 {
		fields = append(fields, effectAllow)
	}
	if len(fields) != 4 {
		return nil, fmt.Errorf("a rule takes \"sub, obj, act\" or \"sub, obj, act, eft\": %s", rule)
	}
	for _, field := range fields {
		if field == "" {
			return nil, fmt.Errorf("a rule has an empty field: %s", rule)
		}
	}
	if fields[3] != effectAllow && fields[3] != effectDeny {
		return nil, fmt.Errorf("the effect of a rule is \"allow\" or \"deny\": %s", rule)
	}
	return fields, nil
}

// PolicyText is the policy as it would be written in a casbin policy file. It
// is what the advanced view shows, so nothing is enforced that cannot be read
// there.
func (permission *AgentPermission) PolicyText() []string {
	lines := []string{}
	for _, rule := range permission.PolicyRules() {
		lines = append(lines, "p, "+strings.Join(rule, ", "))
	}
	return lines
}

// allow asks casbin. An enforcer that errors denies: a rule that cannot be
// evaluated is not one to relay a request on.
func (guard *AgentGuard) allow(object string) bool {
	allowed, err := guard.enforcer.Enforce(guard.agentId, object, actUse)
	if err != nil {
		return false
	}
	return allowed
}

// AllowModel reports whether this agent may ask for the model it named.
func (guard *AgentGuard) AllowModel(model string) bool {
	return guard.allow(objectModel + model)
}

// AllowProvider reports whether this agent's requests may be sent to one
// provider, named by its "owner/name" id.
func (guard *AgentGuard) AllowProvider(providerId string) bool {
	return guard.allow(objectProvider + providerId)
}

// AllowTool reports whether this agent may be offered one tool, by name.
func (guard *AgentGuard) AllowTool(name string) bool {
	// A tool of an MCP server may have a switch of its own, set from the tools
	// a connection was found to offer. That switch decides it; the server's own
	// switch is what every other tool of that server falls back to.
	if specific, ok := McpToolItemOf(name); ok && guard.switched[specific] {
		return guard.allow(objectTool + specific)
	}

	entry := ToolItemOf(name)
	if entry == "" {
		return true
	}
	return guard.allow(objectTool + entry)
}

// FilterProviders drops the providers this agent may not be sent to, and says
// which was the last one dropped so that an empty chain can report why.
func (guard *AgentGuard) FilterProviders(providers []*Provider) ([]*Provider, string) {
	allowed := []*Provider{}
	denied := ""
	for _, provider := range providers {
		if !guard.AllowProvider(provider.GetId()) {
			denied = provider.GetId()
			continue
		}
		allowed = append(allowed, provider)
	}

	reason := ""
	if len(allowed) == 0 && denied != "" {
		reason = fmt.Sprintf("the permissions of agent %s do not allow the provider %s", guard.agentId, denied)
	}
	return allowed, reason
}
