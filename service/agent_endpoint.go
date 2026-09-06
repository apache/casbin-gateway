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

package service

import (
	"fmt"
	"net"
	"strconv"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/agentsession"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
)

// AgentEndpoint resolves where one agent should be pointed. In gateway mode
// that is the local proxy, which is what makes a later switch take effect
// without rewriting a file; in direct mode it is the provider's own upstream.
func AgentEndpoint(agentId string) (agentprovider.Endpoint, error) {
	endpoint := agentprovider.Endpoint{Mode: object.ModeGateway}

	stored, err := object.GetAgent(agentId)
	if err != nil {
		return endpoint, err
	}
	if stored != nil && stored.Mode != "" {
		endpoint.Mode = stored.Mode
	}

	provider, err := object.GetProviderByAgent(agentId)
	if err != nil {
		return endpoint, err
	}

	endpoint.Provider = provider.GetId()
	endpoint.Protocol = object.ProviderApiFamily(provider)
	endpoint.ClientAuth = object.UsesClientAuth(provider)
	if len(provider.Models) > 0 {
		endpoint.Model = provider.Models[0]
		endpoint.Models = provider.Models
	}

	if endpoint.Mode == object.ModeDirect {
		endpoint.BaseUrl = provider.BaseUrl
		endpoint.ApiKey = provider.ApiKey
		endpoint.ServesResponsesApi = object.ServesResponsesApi(provider)
		return endpoint, nil
	}

	baseUrl, err := GatewayAgentUrl(agentId)
	if err != nil {
		return endpoint, err
	}
	endpoint.BaseUrl = baseUrl
	endpoint.ServesResponsesApi = true
	// A client-auth provider forwards whatever the agent sends, so it must keep
	// sending its own credentials: a placeholder token written into the agent's
	// configuration would replace the sign-in it already has.
	if !endpoint.ClientAuth {
		endpoint.ApiKey = conf.GetRelayToken()
	}
	return endpoint, nil
}

// GatewayAgentUrl is the base URL an agent reaches its own provider at. One URL
// serves every wire format: an OpenAI client appends /chat/completions to it,
// Codex appends /responses, and an Anthropic one appends /v1/messages. An agent
// that runs its sessions in a sandbox is named this host's own address instead
// of loopback, which inside a sandbox is the sandbox itself.
func GatewayAgentUrl(agentId string) (string, error) {
	host := "127.0.0.1"
	if agent.RunsSandboxed(agentId) {
		lan, err := LanHost()
		if err != nil {
			return "", err
		}
		host = lan
	}
	address := net.JoinHostPort(host, strconv.Itoa(conf.GetHttpPort()))
	return fmt.Sprintf("http://%s/v1/agents/%s", address, agentId), nil
}

// InitAgentSessionEnv gives the session driver the provider bound to each agent.
func InitAgentSessionEnv() {
	agentsession.SetEnvSource(agentSessionEnv)
}

// agentSessionEnv is the environment one driven agent is started with. Gateway
// hands the session the endpoint it would have written into the agent's
// configuration, for that one process: a driven agent then answers through the
// provider bound to it whether or not the switch was ever applied, and without
// a sign-in of its own.
//
// Nothing is handed over for an agent with no provider bound, or one whose
// endpoint only lives in a file. Those keep reading their own configuration,
// which is what they do when somebody types at them.
func agentSessionEnv(session agentsession.Session) []string {
	endpoint, err := AgentEndpoint(session.AgentId)
	if err != nil || endpoint.BaseUrl == "" {
		return nil
	}

	env := []string{}
	for key, value := range agentprovider.SessionEnv(session.AgentId, endpoint) {
		env = append(env, key+"="+value)
	}
	return env
}
