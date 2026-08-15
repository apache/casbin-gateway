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

// Package mcpserver implements the stdio MCP recorder registered with Claude
// Desktop. It observes MCP traffic and posts it to the running Gateway; it
// never evaluates policy or blocks an agent action.
package mcpserver

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/auditutil"
)

const (
	// Subcommand is used in the Claude Desktop MCP server registration.
	Subcommand = "mcp-server"

	serverName              = "casbin-gateway-agent-monitor"
	fallbackProtocolVersion = "2025-06-18"
	maxLineBytes            = 8 * 1024 * 1024
	reportQueueDepth        = 256
	reportTimeout           = 5 * time.Second
	reportActionTool        = "casbin_gateway_report_action"
)

// Version is reported in the MCP initialize response.
var Version = "0.1.0"

var tools = []map[string]any{{
	"name":        reportActionTool,
	"description": "Record an action in Casbin Gateway's local agent-monitoring timeline.",
	"inputSchema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{"type": "string", "description": "Short action description."},
			"detail": map[string]any{"type": "string", "description": "Relevant action detail."},
		},
		"required": []string{"action"},
	},
}}

// Server serves one MCP stdio session.
type Server struct {
	in         io.Reader
	out        io.Writer
	agentID    string
	agentPath  string
	user       string
	recordsURL string
	client     *http.Client

	queue     chan *agentmonitor.Record
	reporting sync.WaitGroup
}

func newServer(in io.Reader, out io.Writer, agentID, agentPath, user, recordsURL string) *Server {
	return &Server{
		in:         in,
		out:        out,
		agentID:    agentID,
		agentPath:  agentPath,
		user:       user,
		recordsURL: recordsURL,
		client:     &http.Client{Timeout: reportTimeout},
		queue:      make(chan *agentmonitor.Record, reportQueueDepth),
	}
}

// ServeIfInvoked enters MCP server mode when Gateway was launched by Claude
// Desktop. It exits before normal Gateway initialization so stdout remains a
// clean JSON-RPC stream.
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

// Run serves an MCP connection on stdin/stdout.
func Run(args []string) error {
	flags := flag.NewFlagSet(Subcommand, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	recordsURL := flags.String("records-url", "", "Gateway agent record endpoint")
	agentID := flags.String("agent", "claude-desktop", "agent that launched this server")
	agentPath := flags.String("agent-path", "", "agent installation path")
	user := flags.String("user", "", "agent installation owner")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return newServer(os.Stdin, os.Stdout, *agentID, *agentPath, *user, *recordsURL).Serve()
}

// Serve processes line-delimited JSON-RPC messages until the client closes
// stdin. Event reporting is asynchronous so the monitoring endpoint cannot
// delay MCP replies.
func (s *Server) Serve() error {
	s.startReporter()
	s.report("session", "start", "", "success", nil)
	defer func() {
		s.report("session", "end", "", "success", nil)
		close(s.queue)
		s.reporting.Wait()
	}()

	scanner := bufio.NewScanner(s.in)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var message request
		if err := json.Unmarshal(line, &message); err != nil {
			s.report("mcp", "malformed", "", "failure", map[string]any{"error": err.Error()})
			continue
		}

		s.report("mcp", message.Method, "", "attempted", message.paramsMap())
		if len(message.Id) == 0 {
			continue
		}
		if err := s.write(s.dispatch(message)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(message request) response {
	switch message.Method {
	case "initialize":
		return message.reply(s.initializeResult(message))
	case "ping":
		return message.reply(map[string]any{})
	case "tools/list":
		return message.reply(map[string]any{"tools": tools})
	case "tools/call":
		return s.callTool(message)
	default:
		return message.fail(methodNotFound, fmt.Sprintf("unsupported method %q", message.Method))
	}
}

func (s *Server) initializeResult(message request) map[string]any {
	version := fallbackProtocolVersion
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if json.Unmarshal(message.Params, &params) == nil && params.ProtocolVersion != "" {
		version = params.ProtocolVersion
	}
	return map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": serverName, "version": Version},
	}
}

func (s *Server) callTool(message request) response {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		s.report("mcp", "tool_call", "", "failure", map[string]any{"error": err.Error()})
		return message.fail(invalidParams, err.Error())
	}
	if params.Name != reportActionTool {
		s.report("mcp", "tool_call", params.Name, "failure", params.Arguments)
		return message.fail(invalidParams, fmt.Sprintf("unknown tool %q", params.Name))
	}
	s.report("mcp", "tool_call", params.Name, "success", params.Arguments)
	return message.reply(map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": "Recorded in Casbin Gateway's local agent-monitoring timeline.",
		}},
	})
}

func (s *Server) report(eventType, action, toolName, outcome string, payload map[string]any) {
	if s.recordsURL == "" {
		return
	}
	record := &agentmonitor.Record{
		Agent:       s.agentID,
		AgentPath:   s.agentPath,
		User:        s.user,
		CreatedTime: time.Now().Format(time.RFC3339Nano),
		EventType:   eventType,
		Action:      action,
		ToolName:    toolName,
		Outcome:     outcome,
	}
	if eventType == "mcp" && toolName != "" {
		record.McpServer = serverName
		record.McpTool = toolName
	}
	if payload != nil {
		record.Object = auditutil.EncodeBoundedJSON(payload, auditutil.MaxPayloadBytes)
	}
	select {
	case s.queue <- record:
	default:
	}
}

func (s *Server) startReporter() {
	s.reporting.Add(1)
	go func() {
		defer s.reporting.Done()
		for record := range s.queue {
			body, err := json.Marshal(record)
			if err != nil {
				continue
			}
			response, err := s.client.Post(s.recordsURL, "application/json", bytes.NewReader(body))
			if err == nil {
				_ = response.Body.Close()
			}
		}
	}()
}

func (s *Server) write(message response) error {
	message.JSONRPC = "2.0"
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = s.out.Write(append(encoded, '\n'))
	return err
}
