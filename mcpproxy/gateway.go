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

package mcpproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
)

const (
	resolveTimeout = 10 * time.Second
	// A tool call waits on the verdict, so a Gateway that is slow to answer is
	// treated like one that is not running: the call goes ahead. Holding every
	// session on this machine would be the worse failure.
	decideTimeout = 2 * time.Second
)

// resolvedServer is one connection's MCP server with its credentials filled in,
// as Gateway rendered it. It exists only for the life of this process and is
// never written anywhere.
type resolvedServer struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Url       string            `json:"url"`
	Headers   map[string]string `json:"headers"`
}

func (r *resolvedServer) envPairs() []string {
	pairs := make([]string, 0, len(r.Env))
	for name, value := range r.Env {
		pairs = append(pairs, name+"="+value)
	}
	return pairs
}

// gatewayClient is this process's line to the Gateway that launched it.
type gatewayClient struct {
	options Options
	client  *http.Client
}

func newGatewayClient(options Options) *gatewayClient {
	return &gatewayClient{options: options, client: &http.Client{Timeout: resolveTimeout}}
}

type apiResponse struct {
	Status string          `json:"status"`
	Msg    string          `json:"msg"`
	Data   json.RawMessage `json:"data"`
}

func (g *gatewayClient) post(path string, body any, timeout time.Duration) (json.RawMessage, error) {
	if g.options.GatewayUrl == "" {
		return nil, fmt.Errorf("no Gateway address was given")
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(http.MethodPost, g.options.GatewayUrl+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(agentmonitor.IngestTokenHeader, g.options.Token)

	client := g.client
	if timeout != 0 && timeout != client.Timeout {
		client = &http.Client{Timeout: timeout}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var decoded apiResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, err
	}
	if decoded.Status != "ok" {
		return nil, fmt.Errorf("%s", decoded.Msg)
	}
	return decoded.Data, nil
}

// resolve asks Gateway for the server behind this connection. This is the only
// place the credential exists outside Gateway, and it exists in memory of a
// process the agent started and will end.
func (g *gatewayClient) resolve() (*resolvedServer, error) {
	data, err := g.post("/api/resolve-connection", map[string]string{
		"owner": g.options.Owner,
		"name":  g.options.Connection,
		"agent": g.options.AgentId,
	}, resolveTimeout)
	if err != nil {
		return nil, err
	}

	resolved := &resolvedServer{}
	if err := json.Unmarshal(data, resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

// allowTool asks whether this agent may make one call. A verdict that cannot be
// had allows the call, matching what every hook does: Gateway being down or slow
// must not be the thing that breaks somebody's session.
func (g *gatewayClient) allowTool(tool string) (bool, string) {
	if tool == "" || g.options.AgentId == "" || g.options.Token == "" {
		return true, ""
	}

	data, err := g.post("/api/check-agent-tool", map[string]string{
		"agent": g.options.AgentId,
		"tool":  toolKey(g.options.Connection, tool),
	}, decideTimeout)
	if err != nil {
		return true, ""
	}

	var verdict struct {
		Allow  bool   `json:"allow"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(data, &verdict) != nil {
		return true, ""
	}
	return verdict.Allow, verdict.Reason
}

// toolKey names a call the way the permission switches do, so a connection's
// tools land on the switch that agent already has for this MCP server.
func toolKey(connection string, tool string) string {
	return "mcp__" + connection + "__" + tool
}

// GatewayUrl is the address a patched agent is given for the Gateway on this
// machine.
func GatewayUrl(port string) string {
	return (&url.URL{Scheme: "http", Host: "127.0.0.1:" + port}).String()
}
