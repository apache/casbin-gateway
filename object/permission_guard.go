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

// Guards judge a tool call's arguments at the hook: its commands, paths and hosts.

package object

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
)

const (
	GuardCommand = "command"
	GuardPath    = "path"
	GuardHost    = "host"
)

var guardKinds = []string{GuardCommand, GuardPath, GuardHost}

// GuardModelText: the first matching rule decides; each kind ends in an allow-all.
const GuardModelText = `[request_definition]
r = sub, kind, value

[policy_definition]
p = sub, kind, pattern, eft

[policy_effect]
e = priority(p.eft) || deny

[matchers]
m = keyMatch(r.sub, p.sub) && r.kind == p.kind && guardMatch(r.kind, r.value, p.pattern)
`

// parseGuardRule reads "kind, pattern, eft"; the pattern may contain commas.
func parseGuardRule(rule string) ([]string, error) {
	rule = strings.TrimSpace(rule)
	first := strings.Index(rule, ",")
	last := strings.LastIndex(rule, ",")
	if first < 0 || first == last {
		return nil, fmt.Errorf("a guard takes \"kind, pattern, eft\": %s", rule)
	}

	kind := strings.TrimSpace(rule[:first])
	pattern := strings.TrimSpace(rule[first+1 : last])
	effect := strings.TrimSpace(rule[last+1:])
	if !containsString(guardKinds, kind) {
		return nil, fmt.Errorf("the kind of a guard is command, path or host: %s", rule)
	}
	if pattern == "" {
		return nil, fmt.Errorf("a guard has an empty pattern: %s", rule)
	}
	if effect != effectAllow && effect != effectDeny {
		return nil, fmt.Errorf("the effect of a guard is \"allow\" or \"deny\": %s", rule)
	}
	if kind == GuardCommand {
		if _, err := regexp.Compile(pattern); err != nil {
			return nil, fmt.Errorf("a command guard is a regular expression: %s", err.Error())
		}
	}
	return []string{kind, pattern, effect}, nil
}

type guardSource struct {
	rule []string
	from string
}

// guardLines: hand-written guards, then packs, then an allow-all per kind.
func (permission *AgentPermission) guardLines() []guardSource {
	lines := []guardSource{}
	seen := map[string]bool{}
	add := func(parsed []string, from string) {
		rule := []string{permission.Name, parsed[0], parsed[1], parsed[2]}
		key := strings.Join(rule, "\x00")
		if seen[key] {
			return
		}
		seen[key] = true
		lines = append(lines, guardSource{rule: rule, from: from})
	}

	for _, guard := range permission.Guards {
		if parsed, err := parseGuardRule(guard); err == nil {
			add(parsed, "")
		}
	}
	for _, pack := range PermissionPacks() {
		if !containsString(permission.Packs, pack.Name) {
			continue
		}
		for _, guard := range pack.Guards {
			if parsed, err := parseGuardRule(guard); err == nil {
				add(parsed, pack.Name)
			}
		}
	}
	for _, kind := range guardKinds {
		add([]string{kind, "*", effectAllow}, "")
	}
	return lines
}

func (permission *AgentPermission) GuardText() []string {
	lines := []string{}
	for _, line := range permission.guardLines() {
		lines = append(lines, "p, "+strings.Join(line.rule, ", "))
	}
	return lines
}

type callGuard struct {
	enforcer *casbin.Enforcer
	sources  map[string]string
}

var (
	callGuardLock  sync.Mutex
	callGuardCache = map[string]*callGuard{}
)

func loadCallGuard(permission *AgentPermission) (*callGuard, error) {
	if !permission.Enabled || (len(permission.Guards) == 0 && len(permission.Packs) == 0) {
		return nil, nil
	}

	callGuardLock.Lock()
	defer callGuardLock.Unlock()
	if guard := callGuardCache[permission.Name]; guard != nil {
		return guard, nil
	}

	guardModel, err := model.NewModelFromString(GuardModelText)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer(guardModel)
	if err != nil {
		return nil, err
	}
	enforcer.AddFunction("guardMatch", func(args ...interface{}) (interface{}, error) {
		kind, _ := args[0].(string)
		value, _ := args[1].(string)
		pattern, _ := args[2].(string)
		return guardMatch(kind, value, pattern), nil
	})

	lines := permission.guardLines()
	rules := [][]string{}
	sources := map[string]string{}
	for _, line := range lines {
		rules = append(rules, line.rule)
		sources[strings.Join(line.rule, "\x00")] = line.from
	}
	if _, err = enforcer.AddPolicies(rules); err != nil {
		return nil, err
	}

	guard := &callGuard{enforcer: enforcer, sources: sources}
	callGuardCache[permission.Name] = guard
	return guard, nil
}

func dropCallGuard(agentId string) {
	callGuardLock.Lock()
	defer callGuardLock.Unlock()
	delete(callGuardCache, agentId)
}

// check returns why a call is refused, or "" if it may run.
func (guard *callGuard) check(agentId string, facts ToolCallFacts) string {
	values := [][2]string{}
	for _, command := range facts.Commands {
		values = append(values, [2]string{GuardCommand, command})
	}
	for _, path := range facts.Paths {
		values = append(values, [2]string{GuardPath, path})
	}
	for _, host := range facts.Hosts {
		values = append(values, [2]string{GuardHost, host})
	}

	for _, value := range values {
		allowed, explain, err := guard.enforcer.EnforceEx(agentId, value[0], value[1])
		if err != nil || allowed {
			continue
		}
		reason := fmt.Sprintf("the %s %q is not allowed", value[0], shorten(value[1], 200))
		if len(explain) == 4 {
			if from := guard.sources[strings.Join(explain, "\x00")]; from != "" {
				reason += fmt.Sprintf(" by the %q pack", from)
			} else {
				reason += fmt.Sprintf(" by the guard %q", explain[2])
			}
		}
		return reason
	}
	return ""
}

func shorten(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "…"
}

func guardMatch(kind string, value string, pattern string) bool {
	if pattern == "*" {
		return true
	}
	switch kind {
	case GuardCommand:
		matcher, err := cachedRegexp(pattern)
		return err == nil && matcher.MatchString(value)
	case GuardPath:
		matcher, err := cachedRegexp(globRegexp(expandHome(pattern)))
		return err == nil && matcher.MatchString(normalizeGuardPath(value))
	case GuardHost:
		return hostMatch(value, pattern)
	}
	return false
}

var (
	regexpLock  sync.Mutex
	regexpCache = map[string]*regexp.Regexp{}
)

func cachedRegexp(pattern string) (*regexp.Regexp, error) {
	regexpLock.Lock()
	defer regexpLock.Unlock()
	if compiled := regexpCache[pattern]; compiled != nil {
		return compiled, nil
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexpCache[pattern] = compiled
	return compiled, nil
}

var caseInsensitivePaths = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// globRegexp: "**/" spans directories; "*" and "?" stay in one segment.
func globRegexp(glob string) string {
	glob = normalizeGuardPath(glob)
	var builder strings.Builder
	if caseInsensitivePaths {
		builder.WriteString("(?i)")
	}
	builder.WriteString("^")
	for index := 0; index < len(glob); index++ {
		switch char := glob[index]; char {
		case '*':
			if strings.HasPrefix(glob[index:], "**/") {
				builder.WriteString("(?:.*/)?")
				index += 2
			} else if strings.HasPrefix(glob[index:], "**") {
				builder.WriteString(".*")
				index++
			} else {
				builder.WriteString("[^/]*")
			}
		case '?':
			builder.WriteString("[^/]")
		default:
			builder.WriteString(regexp.QuoteMeta(string(char)))
		}
	}
	builder.WriteString("$")
	return builder.String()
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return home + path[1:]
}

func normalizeGuardPath(path string) string {
	path = expandHome(strings.TrimSpace(path))
	if path == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(path))
}

// hostMatch reads "*.example.com" as example.com and every host under it.
func hostMatch(host string, pattern string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if suffix, ok := strings.CutPrefix(pattern, "*."); ok {
		return host == suffix || strings.HasSuffix(host, "."+suffix)
	}
	return host == pattern
}

type ToolCallFacts struct {
	Commands []string
	Paths    []string
	Hosts    []string
}

func (facts ToolCallFacts) empty() bool {
	return len(facts.Commands) == 0 && len(facts.Paths) == 0 && len(facts.Hosts) == 0
}

// Argument names agents use for a command, a path and a URL.
var (
	commandKeys = map[string]bool{"command": true, "cmd": true, "command_line": true, "commandline": true, "script": true}
	pathKeys    = map[string]bool{
		"file_path": true, "filepath": true, "path": true, "absolute_path": true, "notebook_path": true,
		"target_file": true, "target_directory": true, "dir_path": true, "directory": true,
		"file": true, "filename": true, "source": true, "destination": true, "old_path": true,
		"new_path": true, "paths": true, "file_paths": true, "files": true,
	}
	urlKeys = map[string]bool{"url": true, "uri": true, "urls": true, "href": true}
)

var (
	urlInText     = regexp.MustCompile(`(?i)\b(?:https?|ftp|wss?)://[^\s'"<>()\x60]+`)
	downloadWords = map[string]bool{"curl": true, "wget": true, "iwr": true, "irm": true,
		"invoke-webrequest": true, "invoke-restmethod": true, "http": true, "https": true}
	bareHost = regexp.MustCompile(`^(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}(?::\d+)?(?:/\S*)?$`)
)

// ReadToolCallFacts pulls commands, paths and hosts out of a call's arguments.
func ReadToolCallFacts(input map[string]any, cwd string) ToolCallFacts {
	facts := ToolCallFacts{}
	seen := map[string]bool{}
	add := func(list *[]string, kind string, value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[kind+value] {
			return
		}
		seen[kind+value] = true
		*list = append(*list, value)
	}
	addHost := func(raw string) {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			return
		}
		add(&facts.Hosts, GuardHost, parsed.Hostname())
	}

	var walk func(key string, value any, depth int)
	walk = func(key string, value any, depth int) {
		if depth > 4 {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			for child, inner := range typed {
				walk(strings.ToLower(child), inner, depth+1)
			}
		case []any:
			for _, inner := range typed {
				walk(key, inner, depth+1)
			}
		case string:
			switch {
			case commandKeys[key]:
				add(&facts.Commands, GuardCommand, typed)
			case pathKeys[key]:
				add(&facts.Paths, GuardPath, resolveGuardPath(typed, cwd))
			case urlKeys[key]:
				addHost(typed)
			}
		}
	}
	walk("", input, 0)

	for _, command := range facts.Commands {
		for _, raw := range urlInText.FindAllString(command, -1) {
			addHost(raw)
		}
		for _, host := range downloadHosts(command) {
			add(&facts.Hosts, GuardHost, host)
		}
	}
	return facts
}

// downloadHosts finds scheme-less hosts after curl or wget.
func downloadHosts(command string) []string {
	hosts := []string{}
	fields := strings.Fields(command)
	for index, field := range fields {
		if !downloadWords[strings.ToLower(filepath.Base(field))] {
			continue
		}
		for _, argument := range fields[index+1:] {
			argument = strings.Trim(argument, `'"`)
			if strings.ContainsAny(argument, ";|&") {
				break
			}
			if strings.HasPrefix(argument, "-") || strings.Contains(argument, "://") || !bareHost.MatchString(argument) {
				continue
			}
			host := strings.SplitN(argument, "/", 2)[0]
			if name, _, err := net.SplitHostPort(host); err == nil {
				host = name
			}
			hosts = append(hosts, host)
		}
	}
	return hosts
}

func resolveGuardPath(path string, cwd string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(path, "~") || strings.HasPrefix(path, "/") || filepath.IsAbs(path) || cwd == "" {
		return path
	}
	return filepath.Join(cwd, path)
}
