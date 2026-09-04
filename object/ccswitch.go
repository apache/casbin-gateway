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
	"fmt"
	"net/url"
	"strings"

	"github.com/apache/casbin-gateway/agentconfig"
	"github.com/apache/casbin-gateway/ccswitch"
)

// CcSwitchImport is a CC Switch installation on this machine, read and ready to
// be brought over. It is a preview: every key in it is masked, and nothing is
// written until ImportCcSwitch is called with the keys to keep.
type CcSwitchImport struct {
	Found bool   `json:"found"`
	Path  string `json:"path"`
	// Legacy marks a store CC Switch still keeps in config.json.
	Legacy    bool                `json:"legacy"`
	Providers []*CcSwitchProvider `json:"providers"`
	Mcps      []*CcSwitchMcp      `json:"mcps"`
	Prompts   []*CcSwitchPrompt   `json:"prompts"`
	Skills    []*CcSwitchSkill    `json:"skills"`
	// Skipped are the entries there is nothing to bring over from, said out
	// loud so a shorter list than CC Switch shows is not a surprise.
	Skipped []*CcSwitchSkipped `json:"skipped"`
}

// CcSwitchProvider is one provider as Gateway would store it. Key is what a
// selection names it by.
type CcSwitchProvider struct {
	Key string `json:"key"`
	App string `json:"app"`
	// Current marks the one CC Switch has applied for its app right now.
	Current bool `json:"current"`
	// Taken marks a provider already here under the same name, which is added
	// beside it rather than over it.
	Taken    bool      `json:"taken"`
	Provider *Provider `json:"provider"`
}

// CcSwitchMcp is one MCP server of CC Switch's shared list, with the agents its
// apps resolve to here.
type CcSwitchMcp struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Config  string   `json:"config"`
	Targets []string `json:"targets"`
	Unknown []string `json:"unknown"`
}

// CcSwitchPrompt is one set of instructions. CC Switch keeps a library of them
// per app and an agent reads a single file, so only the one CC Switch has
// switched on has somewhere to go, and importing it writes over that file.
type CcSwitchPrompt struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	App     string   `json:"app"`
	Content string   `json:"content"`
	Targets []string `json:"targets"`
	Unknown []string `json:"unknown"`
}

// CcSwitchSkill is a repository skills are installed from.
type CcSwitchSkill struct {
	Key  string `json:"key"`
	Repo string `json:"repo"`
	Ref  string `json:"ref"`
}

// CcSwitchSkipped is one entry that was left out, and why.
type CcSwitchSkipped struct {
	Name   string `json:"name"`
	App    string `json:"app"`
	Reason string `json:"reason"`
}

// CcSwitchSelection is what an import was asked to bring over, by the keys the
// scan gave out. Nothing that is not named here is written.
type CcSwitchSelection struct {
	// Account is the machine account CC Switch is installed under, empty for
	// the one Gateway runs as.
	Account   string   `json:"account"`
	Providers []string `json:"providers"`
	Mcps      []string `json:"mcps"`
	Prompts   []string `json:"prompts"`
	Skills    []string `json:"skills"`
	// Overwrite replaces an MCP server the agent already has under the same
	// name, rather than leaving it alone.
	Overwrite bool `json:"overwrite"`
}

// CcSwitchOutcome is what the import did with one entry, in the vocabulary a
// configuration copy is reported in.
type CcSwitchOutcome struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
	// Agent names the agent it was written into, for the entries that go into
	// one rather than into Gateway itself.
	Agent string `json:"agent,omitempty"`
}

// CcSwitchResult is one import, entry by entry.
type CcSwitchResult struct {
	Providers []*CcSwitchOutcome `json:"providers"`
	Mcps      []*CcSwitchOutcome `json:"mcps"`
	Prompts   []*CcSwitchOutcome `json:"prompts"`
	Skills    []*CcSwitchOutcome `json:"skills"`
}

// ScanCcSwitch reads what CC Switch holds for one machine account and answers
// with what Gateway would make of it. owner is who the providers would belong
// to here; account is whose home the installation is in.
func ScanCcSwitch(owner string, account string) (*CcSwitchImport, error) {
	store, err := ccswitch.Read(account)
	if err != nil {
		return nil, err
	}

	found := &CcSwitchImport{
		Found:     store.Found,
		Path:      store.Path,
		Legacy:    store.Legacy,
		Providers: []*CcSwitchProvider{},
		Mcps:      []*CcSwitchMcp{},
		Prompts:   []*CcSwitchPrompt{},
		Skills:    []*CcSwitchSkill{},
		Skipped:   []*CcSwitchSkipped{},
	}
	if !store.Found {
		return found, nil
	}

	for _, entry := range store.Providers {
		provider, reason := ccSwitchProvider(owner, entry)
		if provider == nil {
			found.Skipped = append(found.Skipped, &CcSwitchSkipped{
				Name:   ccSwitchName(entry),
				App:    entry.App,
				Reason: reason,
			})
			continue
		}

		taken, err := getProvider(owner, provider.Name)
		if err != nil {
			return nil, err
		}
		found.Providers = append(found.Providers, &CcSwitchProvider{
			Key:      ccSwitchProviderKey(entry),
			App:      entry.App,
			Current:  entry.Current,
			Taken:    taken != nil,
			Provider: GetMaskedProvider(provider),
		})
	}

	for _, server := range store.Mcps {
		targets, unknown := linkTargets(server.Apps)
		found.Mcps = append(found.Mcps, &CcSwitchMcp{
			Key:     server.Id,
			Name:    ccSwitchMcpName(server),
			Config:  server.Config,
			Targets: targets,
			Unknown: unknown,
		})
	}

	for _, prompt := range store.Prompts {
		if !prompt.Enabled || strings.TrimSpace(prompt.Content) == "" {
			continue
		}
		targets, unknown := linkTargets([]string{prompt.App})
		found.Prompts = append(found.Prompts, &CcSwitchPrompt{
			Key:     ccSwitchPromptKey(prompt),
			Name:    prompt.Name,
			App:     prompt.App,
			Content: prompt.Content,
			Targets: targets,
			Unknown: unknown,
		})
	}

	for _, repo := range store.SkillRepos {
		found.Skills = append(found.Skills, &CcSwitchSkill{
			Key:  ccSwitchSkillKey(repo),
			Repo: ccSwitchSkillKey(repo),
			Ref:  repo.Branch,
		})
	}
	return found, nil
}

// ImportCcSwitch brings over what the selection names. One entry that cannot be
// written does not stop the rest: a half-finished import is reported entry by
// entry rather than rolled back, since what it writes is other tools' files.
func ImportCcSwitch(owner string, selection *CcSwitchSelection) (*CcSwitchResult, error) {
	store, err := ccswitch.Read(selection.Account)
	if err != nil {
		return nil, err
	}
	if !store.Found {
		return nil, fmt.Errorf("no CC Switch installation was found at %s", store.Path)
	}

	result := &CcSwitchResult{
		Providers: []*CcSwitchOutcome{},
		Mcps:      []*CcSwitchOutcome{},
		Prompts:   []*CcSwitchOutcome{},
		Skills:    []*CcSwitchOutcome{},
	}
	importCcSwitchProviders(owner, store, selection, result)
	importCcSwitchMcps(store, selection, result)
	importCcSwitchPrompts(store, selection, result)
	importCcSwitchSkills(store, selection, result)
	return result, nil
}

func importCcSwitchProviders(owner string, store *ccswitch.Store, selection *CcSwitchSelection, result *CcSwitchResult) {
	wanted := asSet(selection.Providers)
	for _, entry := range store.Providers {
		key := ccSwitchProviderKey(entry)
		if !wanted[key] {
			continue
		}

		outcome := &CcSwitchOutcome{Key: key, Name: ccSwitchName(entry)}
		result.Providers = append(result.Providers, outcome)

		provider, reason := ccSwitchProvider(owner, entry)
		if provider == nil {
			outcome.Action, outcome.Reason = agentconfig.ActionSkip, reason
			continue
		}
		if _, err := AddProvider(provider); err != nil {
			outcome.Action, outcome.Reason = agentconfig.ActionFailed, err.Error()
			continue
		}
		// AddProvider settles on a free name, which is the one to report: it is
		// what the provider is called in the listing afterwards.
		outcome.Action, outcome.Name = agentconfig.ActionCreate, provider.DisplayName
	}
}

func importCcSwitchMcps(store *ccswitch.Store, selection *CcSwitchSelection, result *CcSwitchResult) {
	wanted := asSet(selection.Mcps)
	for _, server := range store.Mcps {
		if !wanted[server.Id] {
			continue
		}

		name := ccSwitchMcpName(server)
		targets, _ := linkTargets(server.Apps)
		if len(targets) == 0 {
			result.Mcps = append(result.Mcps, &CcSwitchOutcome{
				Key: server.Id, Name: name,
				Action: agentconfig.ActionSkip,
				Reason: "none of the apps this server was switched on for is managed here",
			})
			continue
		}

		request, err := ccSwitchMcpRequest(selection.Account, name, targets, server.Config)
		if err != nil {
			result.Mcps = append(result.Mcps, &CcSwitchOutcome{
				Key: server.Id, Name: name,
				Action: agentconfig.ActionFailed, Reason: err.Error(),
			})
			continue
		}
		request.Overwrite = selection.Overwrite

		written, err := agentconfig.AddMcp(*request)
		if err != nil {
			result.Mcps = append(result.Mcps, &CcSwitchOutcome{
				Key: server.Id, Name: name,
				Action: agentconfig.ActionFailed, Reason: err.Error(),
			})
			continue
		}
		for _, item := range written {
			result.Mcps = append(result.Mcps, &CcSwitchOutcome{
				Key: server.Id, Name: item.Name,
				Action: item.Action, Reason: item.Reason, Agent: item.AgentId,
			})
		}
	}
}

func importCcSwitchPrompts(store *ccswitch.Store, selection *CcSwitchSelection, result *CcSwitchResult) {
	wanted := asSet(selection.Prompts)
	for _, prompt := range store.Prompts {
		key := ccSwitchPromptKey(prompt)
		if !wanted[key] {
			continue
		}

		targets, _ := linkTargets([]string{prompt.App})
		for _, agentId := range targets {
			outcome := &CcSwitchOutcome{Key: key, Name: prompt.Name, Agent: agentId}
			result.Prompts = append(result.Prompts, outcome)

			if _, err := agentconfig.SavePrompt(agentId, selection.Account, prompt.Content); err != nil {
				outcome.Action, outcome.Reason = agentconfig.ActionFailed, err.Error()
				continue
			}
			outcome.Action = agentconfig.ActionOverwrite
		}
	}
}

func importCcSwitchSkills(store *ccswitch.Store, selection *CcSwitchSelection, result *CcSwitchResult) {
	wanted := asSet(selection.Skills)
	for _, repo := range store.SkillRepos {
		key := ccSwitchSkillKey(repo)
		if !wanted[key] {
			continue
		}

		outcome := &CcSwitchOutcome{Key: key, Name: key}
		result.Skills = append(result.Skills, outcome)

		source, err := agentconfig.AddSource(selection.Account, &agentconfig.SkillSource{
			Kind: agentconfig.SourceGithub,
			Url:  key,
			Ref:  repo.Branch,
		})
		if err != nil {
			outcome.Action, outcome.Reason = agentconfig.ActionFailed, err.Error()
			continue
		}
		outcome.Action, outcome.Name = agentconfig.ActionCreate, source.Name
	}
}

// ccSwitchProvider turns one CC Switch entry into the provider Gateway would
// store. The second value says why an entry that cannot become one was left
// out, which the page shows beside it.
func ccSwitchProvider(owner string, entry *ccswitch.Provider) (*Provider, string) {
	endpoint := readCcSwitchSettings(entry.Settings)
	if endpoint.baseUrl == "" && endpoint.apiKey == "" {
		return nil, "this entry signs in to the vendor's own account instead of carrying an endpoint and a key"
	}
	if endpoint.baseUrl != "" {
		if err := validateBaseUrl(endpoint.baseUrl); err != nil {
			return nil, err.Error()
		}
	}

	name := ccSwitchName(entry)
	return &Provider{
		Owner:       owner,
		Name:        ccSwitchSlug(name, entry.Id),
		DisplayName: name,
		Type:        ccSwitchProviderType(entry.App, endpoint),
		Status:      "enabled",
		AuthMode:    ProviderAuthProvider,
		BaseUrl:     endpoint.baseUrl,
		ApiKey:      endpoint.apiKey,
		Models:      endpoint.models,
		// CC Switch's own icon field names one of its icons rather than a site,
		// so the vendor's page is what a favicon can be read from.
		Icon:  strings.TrimSpace(entry.Website),
		Notes: strings.TrimSpace(entry.Notes),
	}, ""
}

// ccSwitchProviderType picks the wire format. The Claude apps speak Anthropic's;
// among the rest only OpenAI's own endpoint serves the Responses API, and a
// vendor that resells it behind an OpenAI-compatible URL does not.
func ccSwitchProviderType(app string, endpoint ccSwitchEndpoint) string {
	if ccSwitchProtocol(app) == ProtocolAnthropic {
		return "anthropic"
	}
	if endpoint.responses || ccSwitchIsOpenAi(endpoint.baseUrl) {
		return "openai"
	}
	return "custom"
}

// ccSwitchProtocol is the wire format one CC Switch app speaks. An app it grows
// support for after this was written is taken as OpenAI-compatible, which every
// one but Claude's is.
func ccSwitchProtocol(app string) string {
	if protocol, known := providerLinkApps[app]; known {
		return protocol
	}
	if strings.HasPrefix(app, "claude") {
		return ProtocolAnthropic
	}
	return ProtocolOpenAi
}

func ccSwitchIsOpenAi(baseUrl string) bool {
	parsed, err := url.Parse(baseUrl)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "openai.com" || strings.HasSuffix(host, ".openai.com")
}

// ccSwitchMcpRequest reads the server block CC Switch stores, which is the same
// JSON every host spells a server in.
func ccSwitchMcpRequest(account string, name string, targets []string, config string) (*agentconfig.McpRequest, error) {
	server, err := agentconfig.ParseMcpServer(config)
	if err != nil {
		return nil, err
	}

	return &agentconfig.McpRequest{
		Owner:     account,
		To:        targets,
		Name:      name,
		Transport: server.Transport,
		Command:   server.Command,
		Args:      server.Args,
		Env:       server.Env,
		Url:       server.Url,
		Headers:   server.Headers,
	}, nil
}

func ccSwitchName(entry *ccswitch.Provider) string {
	if name := strings.TrimSpace(entry.Name); name != "" {
		return name
	}
	return entry.Id
}

func ccSwitchMcpName(server *ccswitch.McpServer) string {
	if name := strings.TrimSpace(server.Name); name != "" {
		return name
	}
	return server.Id
}

// The keys a selection names an entry by. A provider and a prompt are both kept
// per app in CC Switch, so neither id is unique on its own.
func ccSwitchProviderKey(entry *ccswitch.Provider) string {
	return entry.App + "/" + entry.Id
}

func ccSwitchPromptKey(prompt *ccswitch.Prompt) string {
	return prompt.App + "/" + prompt.Id
}

func ccSwitchSkillKey(repo *ccswitch.SkillRepo) string {
	return repo.Owner + "/" + repo.Name
}

// ccSwitchSlug mirrors the name the add-provider form derives from what was
// typed. A name written in a script that leaves nothing behind falls back to CC
// Switch's own id, which reads better in a URL than a numbered "provider".
func ccSwitchSlug(name string, id string) string {
	slug := slugOf(name)
	if slug == "" {
		slug = slugOf(id)
	}
	if slug == "" {
		return "provider"
	}
	return slug
}

func slugOf(value string) string {
	var builder strings.Builder
	for _, letter := range strings.ToLower(value) {
		switch {
		case letter >= 'a' && letter <= 'z', letter >= '0' && letter <= '9':
			builder.WriteRune(letter)
		default:
			builder.WriteByte('-')
		}
	}

	slug := strings.Trim(builder.String(), "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if len(slug) > 60 {
		slug = strings.Trim(slug[:60], "-")
	}
	return slug
}

func asSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}
