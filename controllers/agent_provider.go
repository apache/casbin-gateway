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

package controllers

import (
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/service"
)

// PlanAgentProvider renders what a switch would write, without touching a file.
func (c *ApiController) PlanAgentProvider() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	endpoint, err := service.AgentEndpoint(target.AgentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	files, err := agentprovider.Plan(providerTarget(target), endpoint)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(files)
}

// ApplyAgentProvider writes the bound provider into the agent's own configuration
// file, in that agent's format.
func (c *ApiController) ApplyAgentProvider() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	endpoint, err := service.AgentEndpoint(target.AgentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := agentprovider.Apply(providerTarget(target), endpoint); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentprovider.StatusOf(providerTarget(target)))
}

// RestoreAgentProvider puts back the provider settings the agent had before
// Gateway first switched it.
func (c *ApiController) RestoreAgentProvider() {
	if c.RequireAdmin() {
		return
	}

	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	if err := agentprovider.Restore(providerTarget(target)); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentprovider.StatusOf(providerTarget(target)))
}

// GetProviderHealth reports what the proxy has seen of each provider, which is
// what says why a request went to a fallback rather than to the bound provider.
func (c *ApiController) GetProviderHealth() {
	if c.RequireAdmin() {
		return
	}
	c.ResponseOk(object.GetProviderHealth())
}

// checkAgentProtocol rejects providers the agent could never talk to. Only a
// direct binding can be wrong: it writes the provider's own URL into the agent
// configuration, and the two then have to speak the same API. Through the
// gateway any provider will do, since the proxy translates.
func checkAgentProtocol(agentId string, mode string, providerIds []string) error {
	if mode != object.ModeDirect {
		return nil
	}

	spokenByAgent := agentprovider.ProtocolOf(agentId)
	if spokenByAgent == "" {
		return nil
	}

	for _, id := range providerIds {
		if id == "" {
			continue
		}
		provider, err := object.GetProvider(id)
		if err != nil {
			return err
		}
		// A provider that does not exist is reported by the routing itself.
		if provider == nil {
			continue
		}
		if spoken := object.ProviderApiFamily(provider); spoken != spokenByAgent {
			return fmt.Errorf("%s speaks the %s API, but provider %s speaks %s: bind a provider that speaks %s, or route this agent through the gateway",
				agentId, spokenByAgent, id, spoken, spokenByAgent)
		}
	}
	return nil
}

// reapplyAgentProvider writes the bound provider into the configuration of every
// installation of one agent, which is what makes a routing change reach the
// agent at all: an installation Gateway never wrote keeps talking to the
// provider its own configuration names, and in gateway mode there is nothing
// else to point it at the proxy. It reports what it could not write rather than
// failing the routing change, which is already stored by then.
func reapplyAgentProvider(agentId string) string {
	installations, err := agent.Scan(false)
	if err != nil {
		return err.Error()
	}

	endpoint, endpointErr := service.AgentEndpoint(agentId)
	failures := []string{}
	for _, installation := range installations {
		if installation.AgentId != agentId {
			continue
		}
		target := providerTarget(targetOf(installation))
		// An agent whose configuration format Gateway cannot write is reached
		// through the environment variables the UI shows instead.
		if !agentprovider.StatusOf(target).Supported {
			continue
		}
		if endpointErr != nil {
			failures = append(failures, endpointErr.Error())
			break
		}
		if err := agentprovider.Apply(target, endpoint); err != nil {
			failures = append(failures, err.Error())
		}
	}
	return strings.Join(failures, "; ")
}

// restoreAgentProvider puts back the configuration of every installation
// Gateway wrote, which is what unbinding an agent means for the files it left
// behind: they would otherwise keep pointing at a gateway URL that no longer
// has a provider to forward to.
func restoreAgentProvider(agentId string) string {
	installations, err := agent.Scan(false)
	if err != nil {
		return err.Error()
	}

	failures := []string{}
	for _, installation := range installations {
		if installation.AgentId != agentId {
			continue
		}
		if err := agentprovider.Restore(providerTarget(targetOf(installation))); err != nil {
			failures = append(failures, err.Error())
		}
	}
	return strings.Join(failures, "; ")
}

func providerTarget(target agentpatch.Target) agentprovider.Target {
	return agentprovider.Target{AgentId: target.AgentId, Path: target.Path, Owner: target.Owner}
}
