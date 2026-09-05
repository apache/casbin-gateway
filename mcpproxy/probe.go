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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/connector"
)

// ProbeTimeout is how long one probe waits. A server installed from npm is
// downloaded the first time it runs, which is slow and is not a failure.
const ProbeTimeout = 90 * time.Second

// ProbeResult is what a connection's server said about itself.
type ProbeResult struct {
	ServerName    string      `json:"serverName"`
	ServerVersion string      `json:"serverVersion"`
	Tools         []ProbeTool `json:"tools"`
}

// ProbeTool is one tool the server offers. The description is kept because it
// is what tells two similarly named tools apart when somebody is deciding which
// of them an agent may use.
type ProbeTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Probe starts one connection's server, asks what it is and what it offers, and
// stops it again. It is Gateway asking rather than an agent, so no permission is
// consulted and nothing is recorded against a session.
func Probe(rendered connector.Rendered, timeout time.Duration) (*ProbeResult, error) {
	if timeout <= 0 {
		timeout = ProbeTimeout
	}

	server, err := openUpstream(&resolvedServer{
		Name:      rendered.Name,
		Transport: rendered.Transport,
		Command:   rendered.Command,
		Args:      rendered.Args,
		Env:       rendered.Env,
		Url:       rendered.Url,
		Headers:   rendered.Headers,
	})
	if err != nil {
		return nil, err
	}
	defer server.Close()

	replies := newReplyBox()
	if err := server.Start(replies.accept); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	initialize, err := ask(server, replies, 1, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "casbin-gateway", "version": "1"},
	}, time.Until(deadline))
	if err != nil {
		return nil, err
	}

	// A server is entitled to ignore everything until it has been told the
	// handshake is done, so this notification is not optional.
	if err := send(server, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return nil, err
	}

	listed, err := ask(server, replies, 2, "tools/list", map[string]any{}, time.Until(deadline))
	if err != nil {
		return nil, err
	}

	result := &ProbeResult{Tools: []ProbeTool{}}
	var info struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if json.Unmarshal(initialize, &info) == nil {
		result.ServerName = info.ServerInfo.Name
		result.ServerVersion = info.ServerInfo.Version
	}

	var tools struct {
		Tools []ProbeTool `json:"tools"`
	}
	if err := json.Unmarshal(listed, &tools); err != nil {
		return nil, fmt.Errorf("the server's tool list could not be read: %w", err)
	}
	result.Tools = append(result.Tools, tools.Tools...)
	return result, nil
}

// ask sends one request and waits for the reply carrying its id.
func ask(server upstream, replies *replyBox, id int, method string, params any, wait time.Duration) (json.RawMessage, error) {
	waiting := replies.expect(id)
	if err := send(server, map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	}); err != nil {
		return nil, err
	}

	if wait <= 0 {
		return nil, fmt.Errorf("%s timed out", method)
	}
	select {
	case reply := <-waiting:
		if reply.Error != nil {
			return nil, fmt.Errorf("%s failed: %s", method, reply.Error.Message)
		}
		return reply.Result, nil
	case <-server.Ended():
		// The server is gone, so no reply is coming. What it printed on the way
		// out is the actual reason, and waiting out the timeout would only hide
		// it behind a slower, less useful message.
		if said := server.Diagnostics(); said != "" {
			return nil, fmt.Errorf("the server stopped without answering %s: %s", method, said)
		}
		return nil, fmt.Errorf("the server stopped without answering %s", method)
	case <-time.After(wait):
		return nil, fmt.Errorf("%s timed out after %s", method, wait.Round(time.Second))
	}
}

func send(server upstream, message map[string]any) error {
	encoded, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return server.Send(encoded)
}

type probeReply struct {
	Result json.RawMessage
	Error  *struct {
		Message string `json:"message"`
	}
}

// replyBox matches the server's answers to the questions this probe asked. A
// server may also send notifications and logs, which carry no id and are
// dropped here.
type replyBox struct {
	mutex   sync.Mutex
	waiting map[int]chan probeReply
}

func newReplyBox() *replyBox {
	return &replyBox{waiting: map[int]chan probeReply{}}
}

func (b *replyBox) expect(id int) chan probeReply {
	channel := make(chan probeReply, 1)
	b.mutex.Lock()
	b.waiting[id] = channel
	b.mutex.Unlock()
	return channel
}

func (b *replyBox) accept(line []byte) error {
	var message struct {
		Id     *int            `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(line, &message) != nil || message.Id == nil {
		return nil
	}

	b.mutex.Lock()
	channel, found := b.waiting[*message.Id]
	delete(b.waiting, *message.Id)
	b.mutex.Unlock()
	if found {
		channel <- probeReply{Result: message.Result, Error: message.Error}
	}
	return nil
}
