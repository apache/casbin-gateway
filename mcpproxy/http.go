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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// sessionHeader is what a Streamable HTTP server names a session with. It comes
// back from initialize and has to be sent on everything after it, or the server
// treats each request as a new session.
const sessionHeader = "Mcp-Session-Id"

const httpTimeout = 5 * time.Minute

// httpUpstream reaches a remote MCP server over Streamable HTTP. One client
// message is one POST, and the reply is either a JSON body or an event stream
// carrying several messages.
type httpUpstream struct {
	resolved *resolvedServer
	client   *http.Client
	emit     func([]byte) error

	mutex   sync.Mutex
	session string
}

func newHttpUpstream(resolved *resolvedServer) *httpUpstream {
	return &httpUpstream{resolved: resolved, client: &http.Client{Timeout: httpTimeout}}
}

func (u *httpUpstream) Start(emit func([]byte) error) error {
	u.emit = emit
	return nil
}

func (u *httpUpstream) Close() error { return nil }

func (u *httpUpstream) Send(line []byte) error {
	request, err := http.NewRequest(http.MethodPost, u.resolved.Url, bytes.NewReader(line))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	for name, value := range u.resolved.Headers {
		request.Header.Set(name, value)
	}
	if session := u.currentSession(); session != "" {
		request.Header.Set(sessionHeader, session)
	}

	response, err := u.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if session := response.Header.Get(sessionHeader); session != "" {
		u.rememberSession(session)
	}
	// A notification is answered with 202 and no body, which is not something
	// to pass on.
	if response.StatusCode == http.StatusAccepted || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if response.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024))
		return fmt.Errorf("%s answered %s: %s", u.resolved.Url, response.Status, strings.TrimSpace(string(body)))
	}

	if strings.Contains(response.Header.Get("Content-Type"), "text/event-stream") {
		return u.readEvents(response.Body)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return u.emit(body)
}

// readEvents forwards every message an event stream carries. Only the data
// field matters here: the rest of the SSE framing is transport, and what an MCP
// client is given back is the JSON-RPC message inside it.
func (u *httpUpstream) readEvents(body io.Reader) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if err := u.emit([]byte(data)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (u *httpUpstream) currentSession() string {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	return u.session
}

func (u *httpUpstream) rememberSession(session string) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	u.session = session
}
