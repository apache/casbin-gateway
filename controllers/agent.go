// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package controllers

import (
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agenthistory"
	"github.com/apache/casbin-gateway/agentinstall"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/service"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

type discoveredAgent struct {
	agent.Installation
	agentpatch.Status

	// Provider is the "owner/name" id of the bound provider, and Fallbacks are the
	// providers tried after it. Installations are discovered per host while the
	// routing is stored per agent id, so they merge here.
	Provider  string   `json:"provider"`
	Fallbacks []string `json:"fallbacks"`
	Mode      string   `json:"mode"`
	// ProviderConfig is the state of the agent's own configuration file, which
	// is what the config orchestrator writes.
	ProviderConfig agentprovider.Status `json:"providerConfig"`
	// ProxyBaseUrl is the endpoint this agent reaches its bound provider at. The
	// page cannot build it from its own address: an agent that runs in a sandbox
	// is given this host's network address rather than loopback.
	ProxyBaseUrl string `json:"proxyBaseUrl"`
	// Upgrade is the command that would update this installation in place,
	// through the package manager that installed it.
	Upgrade agentinstall.Plan `json:"upgrade"`
	// Uninstall is the command that would remove it with that same manager.
	Uninstall agentinstall.Plan `json:"uninstall"`
	// SupportsInstances says whether this agent can be run more than once at a
	// time, each copy signed in to an account of its own.
	SupportsInstances bool `json:"supportsInstances"`
}

// GetAgents scans known installation locations and returns the AI agents
// installed in the environment where Casbin Gateway is running.
func (c *ApiController) GetAgents() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(c.GetString("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	agents, err := object.GetAgents()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	// A connection's entries name this Gateway's address and program, so they go
	// stale the same way a hook does. Repairing them here means the fix lands
	// wherever the agents are listed, without a page of its own to visit.
	if err := object.EnsureConnectionsCurrent(); err != nil {
		beego.Error("some connections could not be written again:", err)
	}

	result := make([]*discoveredAgent, 0, len(installations))
	baseUrls := map[string]string{}
	for _, installation := range installations {
		target := targetOf(installation)
		// Monitoring is on by default, so an installation that is not monitored
		// yet is patched here instead of waiting for the switch to be flipped.
		// A failure is left to the status below, which says what is wrong.
		_ = agentpatch.EnsurePatched(target)
		item := &discoveredAgent{
			Installation:   installation,
			Status:         agentpatch.StatusOf(target),
			Fallbacks:      []string{},
			Mode:           object.ModeGateway,
			ProviderConfig: agentprovider.StatusOf(providerTarget(target)),
			Upgrade:        agentinstall.UpgradePlan(installation),
			Uninstall:      agentinstall.UninstallPlan(installation),

			SupportsInstances: agent.SupportsInstances(installation.AgentId),
		}
		baseUrl, ok := baseUrls[installation.AgentId]
		if !ok {
			// A host with no address of its own cannot serve a sandboxed agent
			// at all; the binding itself reports that, and the listing stays.
			baseUrl, _ = gatewayAgentUrl(installation.AgentId)
			baseUrls[installation.AgentId] = baseUrl
		}
		item.ProxyBaseUrl = baseUrl
		if stored, ok := agents[installation.AgentId]; ok {
			item.Provider = stored.Provider
			item.Mode = stored.Mode
			if stored.Fallbacks != nil {
				item.Fallbacks = stored.Fallbacks
			}
		}
		result = append(result, item)
	}
	c.ResponseOk(result, agent.InContainer())
}

// GetAgentCatalog returns every agent Gateway knows how to detect, installed or
// not, so the pages can list the ones missing from this machine and point at
// the vendor's install page.
func (c *ApiController) GetAgentCatalog() {
	if c.RequireAdmin() {
		return
	}

	catalog := agent.KnownAgents()
	result := make([]*catalogEntry, 0, len(catalog))
	for _, known := range catalog {
		result = append(result, &catalogEntry{Known: known, Install: agentinstall.InstallPlan(known.AgentId)})
	}
	c.ResponseOk(result)
}

// catalogEntry is one known agent with the command that would install it here,
// so a machine missing an agent is offered the install rather than only told
// where to read about it.
type catalogEntry struct {
	agent.Known
	Install agentinstall.Plan `json:"install"`
}

// UpdateAgentRouting binds one agent to the provider its requests are forwarded
// to, to the providers tried when that one cannot answer, and to the way it
// reaches them. The binding is per agent id.
//
// The configuration file of every installation Gateway can write is written
// here, since a binding an agent never reads is one that does nothing: it keeps
// calling the provider its own configuration names. Unbinding the agent puts
// that configuration back the way it was found.
func (c *ApiController) UpdateAgentRouting() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId   string   `json:"agentId"`
		Provider  string   `json:"provider"`
		Fallbacks []string `json:"fallbacks"`
		Mode      string   `json:"mode"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.IsKnownAgentId(form.AgentId) {
		c.ResponseError("unknown agent: " + form.AgentId)
		return
	}

	if err := saveAgentRouting(form.AgentId, form.Provider, form.Fallbacks, form.Mode); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(form.Provider)
}

// saveAgentRouting stores one agent's routing and writes it into the
// configuration of every installation Gateway can write. Both the routing form
// and the tray's one-click switch end here, so an agent reached from either
// place is left in the same state.
func saveAgentRouting(agentId string, providerId string, fallbacks []string, mode string) error {
	if err := checkAgentProtocol(agentId, mode, append([]string{providerId}, fallbacks...)); err != nil {
		return err
	}

	if err := object.SetAgentRouting(agentId, providerId, fallbacks, mode); err != nil {
		return err
	}

	// An agent that runs in a sandbox is given this host's own address, so the
	// management port has to answer there before that endpoint is written.
	if err := service.SyncLanAccess(); err != nil {
		return errors.New("the routing was saved, but the agent cannot reach Gateway from its sandbox: " + err.Error())
	}

	if providerId == "" {
		if failure := restoreAgentProvider(agentId); failure != "" {
			return errors.New("the routing was cleared, but the agent configuration was not restored: " + failure)
		}
		return nil
	}
	if failure := reapplyAgentProvider(agentId); failure != "" {
		return errors.New("the routing was saved, but the agent configuration was not rewritten: " + failure)
	}
	return nil
}

// PatchAgent enables monitoring for one discovered installation.
func (c *ApiController) PatchAgent() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentpatch.Patch(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentpatch.StatusOf(target))
}

// UnpatchAgent disables monitoring and restores any configuration it changed.
func (c *ApiController) UnpatchAgent() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentpatch.Unpatch(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentpatch.StatusOf(target))
}

// GetAgentRecords returns the stored agent activity.
func (c *ApiController) GetAgentRecords() {
	if c.RequireAdmin() {
		return
	}

	limit := 200
	if value := c.Input().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		limit = parsed
	}
	c.ResponseOk(object.GetAgentRecords(object.AgentRecordFilter{
		Agent:     c.Input().Get("agent"),
		EventType: c.Input().Get("eventType"),
		Outcome:   c.Input().Get("outcome"),
		Session:   c.Input().Get("session"),
		Limit:     limit,
	}))
}

// GetAgentSessions groups the stored records by agent session. The optional
// agent filter is what an agent's own detail page asks for.
func (c *ApiController) GetAgentSessions() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.Input().Get("agent")
	sessions := object.GetAgentRecordSessions(agentId)

	// The transcripts on disk are the sessions that already happened, so they
	// are listed next to the monitored ones rather than only after Patch.
	seen := map[string]bool{}
	for _, session := range sessions {
		seen[sessionSeenKey(session.Agent, session.SessionKey)] = true
	}
	for _, session := range object.HistoricalSessions(agentId) {
		if seen[sessionSeenKey(session.Agent, session.SessionKey)] {
			continue
		}
		sessions = append(sessions, session)
	}

	sort.SliceStable(sessions, func(left, right int) bool {
		return sessions[left].LastTime > sessions[right].LastTime
	})
	c.ResponseOk(sessions)
}

// GetAgentSession reads one transcript in full, so that a session listed off
// disk can be opened and read instead of only counted. The session is looked up
// by key among the ones a scan found, which is what keeps a request from naming
// a file of its own.
func (c *ApiController) GetAgentSession() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.Input().Get("agent")
	sessionKey := c.Input().Get("session")
	if sessionKey == "" {
		c.ResponseError("session is required")
		return
	}

	for _, session := range object.HistoricalSessions(agentId) {
		if session.SessionKey != sessionKey {
			continue
		}

		transcript, err := agenthistory.ReadTranscript(session)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(transcript)
		return
	}

	c.ResponseError("no transcript on disk for this session")
}

// GetAgentUsage totals what the agents on this machine spent, read from the
// transcripts they write themselves rather than from the requests Gateway
// relayed. LLM Records is empty until an agent is routed through the gateway,
// and stays empty for one talking to its vendor directly; the transcripts are
// what it cost either way.
//
// The optional days narrows the window to that many calendar days ending today,
// and the optional agent to one agent's own transcripts.
func (c *ApiController) GetAgentUsage() {
	if c.RequireAdmin() {
		return
	}

	since := ""
	if value := c.Input().Get("days"); value != "" {
		days, err := strconv.Atoi(value)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		if days > 0 {
			since = time.Now().AddDate(0, 0, -(days - 1)).Format(time.DateOnly)
		}
	}
	c.ResponseOk(object.GetAgentUsage(object.HistoricalSessions(c.Input().Get("agent")), since))
}

// sessionSeenKey identifies one session across the two sources, so a session
// that monitoring already reported is not listed twice.
func sessionSeenKey(agentId string, sessionKey string) string {
	return agentId + "/" + sessionKey
}

// CheckAgentTool decides one tool call for the hook an agent runs before
// executing it. This is the enforcement point for the traffic that never comes
// through the proxy: whatever provider an agent talks to, the tool still runs
// on this machine, and the hook asks here first.
//
// It authenticates the way record ingestion does, with the per-installation
// credential issued at Patch time, and it answers "allowed" rather than an
// error whenever the decision cannot be made - a hook that cannot get a verdict
// must not wedge every session on the machine.
func (c *ApiController) CheckAgentTool() {
	if _, ok := c.directLoopbackClient(); !ok {
		c.ResponseError("agent tool checks are limited to direct loopback requests")
		return
	}
	tokenAgentId, ok := agentpatch.ValidateIngestToken(c.Ctx.Input.Header(agentmonitor.IngestTokenHeader))
	if !ok {
		c.ResponseError("an agent tool check requires a valid installation token")
		return
	}

	var form struct {
		Agent      string `json:"agent"`
		Tool       string `json:"tool"`
		SessionKey string `json:"sessionKey"`
		ToolUseId  string `json:"toolUseId"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if form.Agent == "" {
		c.ResponseError("agent is required")
		return
	}
	// The token decides which agent a caller may ask for, so a compromised hook
	// cannot have another installation's rules applied to its calls.
	if tokenAgentId != "" && !strings.EqualFold(form.Agent, tokenAgentId) {
		c.ResponseError("agent does not match the installation this token was issued for")
		return
	}

	agentIds := agentmonitor.SharedAgentIds(form.Agent)
	allowed, reason, err := object.CheckAgentTool(agentIds, form.Tool)
	if err != nil {
		beego.Error("agent tool check failed, the call was allowed:", err)
	}
	if !allowed {
		// A refusal is worth a record of its own: the agent tells the model it
		// was blocked, and nothing else would show it on the records page.
		agentmonitor.AddRecord(&agentmonitor.Record{
			Agent:       agentmonitor.MonitorAgentId(form.Agent),
			CreatedTime: util.GetCurrentTime(),
			EventType:   "permission",
			Action:      "denied",
			Outcome:     "denied",
			SessionKey:  form.SessionKey,
			ToolUseId:   form.ToolUseId,
			ToolName:    form.Tool,
			Detail:      reason,
		})
	}
	c.ResponseOk(map[string]any{"allow": allowed, "reason": reason})
}

// AddAgentRecord accepts reports from a hook or MCP process launched locally by
// Gateway. Those processes have no browser session, so they authenticate with
// the per-installation credential issued at Patch time. Loopback alone is not a
// trust boundary: behind a reverse proxy every caller looks local, and any web
// page the operator visits can reach 127.0.0.1.
func (c *ApiController) AddAgentRecord() {
	ip, ok := c.directLoopbackClient()
	if !ok {
		c.ResponseError("agent record ingestion is limited to direct loopback requests")
		return
	}
	agentId, ok := agentpatch.ValidateIngestToken(c.Ctx.Input.Header(agentmonitor.IngestTokenHeader))
	if !ok {
		c.ResponseError("agent record ingestion requires a valid installation token")
		return
	}

	var record agentmonitor.Record
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &record); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if record.Agent == "" {
		c.ResponseError("agent is required")
		return
	}
	// The token decides which agent a reporter may speak for, so a compromised
	// hook cannot attribute its activity to a different installation.
	if agentId != "" && !strings.EqualFold(record.Agent, agentId) {
		c.ResponseError("agent does not match the installation this token was issued for")
		return
	}
	record.ClientIp = ip.String()
	agentmonitor.AddRecord(&record)
	c.ResponseOk()
}

// directLoopbackClient reports the peer address, rejecting anything that was
// relayed by a proxy. A forwarding header means the real client is remote even
// though the socket is local.
func (c *ApiController) directLoopbackClient() (net.IP, bool) {
	for _, header := range []string{"X-Forwarded-For", "X-Real-Ip", "Forwarded"} {
		if c.Ctx.Input.Header(header) != "" {
			return nil, false
		}
	}
	remoteAddr := c.Ctx.Request.RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, false
	}
	return ip, true
}

// readAgentInstallation resolves the request body against the installations that
// were actually discovered. Patching writes into the owner's home directory and
// starting runs a program, so an unverified body would let a caller name any
// account, and any file, on the host.
func (c *ApiController) readAgentInstallation() (agent.Installation, bool) {
	var requested agentpatch.Target
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &requested); err != nil {
		c.ResponseError(err.Error())
		return agent.Installation{}, false
	}

	installation, err := findInstallation(requested.AgentId, requested.Path, requested.Owner)
	if err != nil {
		c.ResponseError(err.Error())
		return agent.Installation{}, false
	}
	return installation, true
}

func (c *ApiController) readAgentPatchTarget() (agentpatch.Target, bool) {
	installation, ok := c.readAgentInstallation()
	if !ok {
		return agentpatch.Target{}, false
	}
	return targetOf(installation), true
}

func matchesTarget(discovered, requested agentpatch.Target) bool {
	return discovered.AgentId == requested.AgentId &&
		strings.EqualFold(filepath.Clean(discovered.Path), filepath.Clean(requested.Path)) &&
		strings.EqualFold(discovered.Owner, requested.Owner)
}

func targetOf(installation agent.Installation) agentpatch.Target {
	return agentpatch.Target{
		AgentId: installation.AgentId,
		Path:    installation.Path,
		Owner:   installation.Owner,
	}
}
