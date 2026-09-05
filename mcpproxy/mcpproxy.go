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

// Package mcpproxy stands between an agent and the MCP server one connection
// reaches. The agent is given this executable as the server to run, so what its
// configuration file holds is the name of a connection rather than the
// credential behind it: the credential stays in Gateway and is fetched here,
// over loopback, for the life of one session.
//
// Everything the agent sends passes through, so a tool call can be asked about
// before it runs and recorded after — which is what puts connector traffic
// under the same rules as the rest of an agent's tools.
package mcpproxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sync"
)

// Subcommand is what an agent's configuration runs this executable with.
const Subcommand = "mcp-proxy"

const maxLineBytes = 8 * 1024 * 1024

// ServeIfInvoked enters proxy mode when an agent launched Gateway as the MCP
// server for one connection. Like the recorder, it exits before Gateway
// initializes its own services so stdout stays a clean JSON-RPC stream.
func ServeIfInvoked() {
	if len(os.Args) < 2 || os.Args[1] != Subcommand {
		return
	}
	if err := Run(os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "casbin-gateway %s: %v\n", Subcommand, err)
		os.Exit(1)
	}
	os.Exit(0)
}

// Options is one proxied session.
type Options struct {
	Connection string
	AgentId    string
	Owner      string
	GatewayUrl string
	Token      string
}

func Run(args []string) error {
	flags := flag.NewFlagSet(Subcommand, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := Options{}
	flags.StringVar(&options.Connection, "connection", "", "the connection to reach")
	flags.StringVar(&options.AgentId, "agent", "", "agent this server was written into")
	flags.StringVar(&options.Owner, "owner", "", "the account the connection belongs to")
	flags.StringVar(&options.GatewayUrl, "gateway-url", "", "the running Gateway")
	flags.StringVar(&options.Token, "token", "", "credential presented to Gateway")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if options.Connection == "" {
		return fmt.Errorf("--connection is required")
	}
	return Serve(os.Stdin, os.Stdout, options)
}

// Serve proxies one session until the agent closes its end.
func Serve(in *os.File, out *os.File, options Options) error {
	client := newGatewayClient(options)

	// Resolving is what needs Gateway running. Failing here with a plain
	// message is better than a half-open session: the agent shows the server as
	// failed to start, which is what actually happened.
	resolved, err := client.resolve()
	if err != nil {
		return fmt.Errorf("cannot reach connection %q: %w", options.Connection, err)
	}

	server, err := openUpstream(resolved)
	if err != nil {
		return err
	}
	defer server.Close()

	proxy := &proxy{options: options, client: client, server: server, out: out}
	if err := server.Start(proxy.emit); err != nil {
		return err
	}
	// Records are written beside the session rather than in it, so the last few
	// would be lost to the exit that follows this return.
	defer proxy.reporting.Wait()
	return proxy.pump(in)
}

type proxy struct {
	options Options
	client  *gatewayClient
	server  upstream
	out     *os.File

	// writing serializes what reaches the agent: replies the upstream produced
	// and refusals written here are both messages on one stream.
	writing sync.Mutex
	// reporting counts the records still in flight, so the session waits for
	// them instead of the process exiting from under them.
	reporting sync.WaitGroup
}

// reportCall records one forwarded call without the session waiting on it.
func (p *proxy) reportCall(tool string) {
	p.reporting.Add(1)
	go func() {
		defer p.reporting.Done()
		p.client.report(tool, "attempted", "")
	}()
}

// message is only what the proxy has to look at. Everything else about a
// JSON-RPC message is forwarded byte for byte, so a field this build has never
// heard of still reaches the other side intact.
type message struct {
	Id     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params struct {
		Name string `json:"name"`
	} `json:"params"`
}

// pump forwards everything the agent sends, asking about the calls that need
// asking about.
func (p *proxy) pump(in *os.File) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		// The scanner reuses its buffer, and the upstream write may outlive
		// this iteration.
		forwarded := append([]byte(nil), line...)

		var parsed message
		if err := json.Unmarshal(forwarded, &parsed); err == nil && parsed.Method == "tools/call" {
			allowed, reason := p.client.allowTool(parsed.Params.Name)
			if !allowed {
				// The refusal is already recorded by the endpoint that decided
				// it, so only the calls that go through are reported here.
				if err := p.refuse(parsed.Id, reason); err != nil {
					return err
				}
				continue
			}
			p.reportCall(parsed.Params.Name)
		}

		if err := p.server.Send(forwarded); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// emit writes one upstream message to the agent.
func (p *proxy) emit(line []byte) error {
	p.writing.Lock()
	defer p.writing.Unlock()
	if _, err := p.out.Write(append(bytes.TrimSpace(line), '\n')); err != nil {
		return err
	}
	return nil
}

// refuse answers a denied call as a tool error rather than a protocol error, so
// the model is told it may not use the tool and carries on, instead of the
// client treating the server as broken.
func (p *proxy) refuse(id json.RawMessage, reason string) error {
	if len(id) == 0 {
		return nil
	}
	if reason == "" {
		reason = "this tool is not allowed for this agent"
	}

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]any{
			"isError": true,
			"content": []map[string]any{{"type": "text", "text": "Refused by Casbin Gateway: " + reason}},
		},
	})
	if err != nil {
		return err
	}
	return p.emit(body)
}
