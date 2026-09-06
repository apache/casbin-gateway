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
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentsession"
	"github.com/apache/casbin-gateway/object"
)

// sessionStreamHeartbeat keeps a stream that has gone quiet from being closed by
// whatever is between the page and Gateway.
const sessionStreamHeartbeat = 25 * time.Second

// drivableAgent is one installation the chat page can open a session against.
type drivableAgent struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Owner   string `json:"owner"`
	// Resumable is false for an agent that carries nothing from one turn to the
	// next, which the page says out loud rather than letting somebody find out.
	Resumable bool `json:"resumable"`
}

// GetDrivableAgents lists the installations Gateway can hand a prompt to. An
// agent that publishes no non-interactive mode is left out rather than shown as
// something that will work later.
func (c *ApiController) GetDrivableAgents() {
	if c.RequireAdmin() {
		return
	}

	installations, err := agent.Scan(false)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	result := []*drivableAgent{}
	for _, installation := range installations {
		headless := agent.HeadlessOf(installation.AgentId)
		if headless == nil {
			continue
		}
		result = append(result, &drivableAgent{
			AgentId:   installation.AgentId,
			Name:      installation.Name,
			Path:      installation.Path,
			Owner:     installation.Owner,
			Resumable: headless.CanResume(),
		})
	}
	c.ResponseOk(result)
}

// GetDrivenSessions lists the conversations Gateway is driving, most recently
// used first.
func (c *ApiController) GetDrivenSessions() {
	if c.RequireAdmin() {
		return
	}

	sessions := agentsession.List()
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedTime > sessions[j].UpdatedTime
	})
	c.ResponseOk(sessions)
}

// OpenDrivenSession starts a conversation against one installation. Nothing runs
// until it is asked something.
func (c *ApiController) OpenDrivenSession() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		AgentId string `json:"agentId"`
		Path    string `json:"path"`
		Owner   string `json:"owner"`
		WorkDir string `json:"workDir"`
		Model   string `json:"model"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	installation, err := findInstallation(request.AgentId, request.Path, request.Owner)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	session, err := agentsession.Open(agentsession.Spec{
		AgentId:   installation.AgentId,
		AgentPath: installation.Path,
		Owner:     installation.Owner,
		WorkDir:   strings.TrimSpace(request.WorkDir),
		Model:     strings.TrimSpace(request.Model),
		Source:    "web",
	})
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(session)
}

// SendDrivenSession asks the session's agent one thing and returns as soon as it
// has been started. What it says arrives on the stream.
func (c *ApiController) SendDrivenSession() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Id     string `json:"id"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if strings.TrimSpace(request.Prompt) == "" {
		c.ResponseError("there is nothing to ask")
		return
	}

	session, err := agentsession.Send(request.Id, request.Prompt)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(session)
}

// InterruptDrivenSession stops the turn a session is in the middle of, leaving
// the session open for the next one.
func (c *ApiController) InterruptDrivenSession() {
	if c.RequireAdmin() {
		return
	}

	id := c.readSessionId()
	if id == "" {
		return
	}
	if err := agentsession.Interrupt(id); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk()
}

// CloseDrivenSession ends a conversation and forgets it. The agent keeps its own
// transcript of what was said, which is where the record of it lives.
func (c *ApiController) CloseDrivenSession() {
	if c.RequireAdmin() {
		return
	}

	id := c.readSessionId()
	if id == "" {
		return
	}
	if err := agentsession.Close(id); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.DeleteAgentSession(id); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk()
}

// StreamDrivenSession pushes what a session says as it says it. What it has
// already said is replayed first, so a page that opens in the middle of a turn -
// or comes back to one - reads the whole conversation from here.
func (c *ApiController) StreamDrivenSession() {
	if c.RequireAdmin() {
		return
	}

	id := c.Input().Get("id")
	// The browser reconnects a dropped stream on its own and says which event it
	// had last, so only what came after that is replayed.
	seen, _ := strconv.ParseInt(firstNonEmpty(c.Ctx.Input.Header("Last-Event-ID"), c.Input().Get("seen")), 10, 64)

	subscriberId, feed, history, err := agentsession.Subscribe(id, seen)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	defer agentsession.Unsubscribe(id, subscriberId)

	c.EnableRender = false
	writer := c.Ctx.ResponseWriter
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	// An nginx in front of Gateway would otherwise hold every event back until
	// the response ends.
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	writer.Flush()

	for _, event := range history {
		if !writeSessionEvent(writer, event) {
			return
		}
	}

	ticker := time.NewTicker(sessionStreamHeartbeat)
	defer ticker.Stop()
	closed := c.Ctx.Request.Context().Done()
	for {
		select {
		case event, ok := <-feed:
			if !ok {
				return
			}
			if !writeSessionEvent(writer, event) {
				return
			}
		case <-ticker.C:
			if _, err := fmt.Fprint(writer, ": keep-alive\n\n"); err != nil {
				return
			}
			writer.Flush()
		case <-closed:
			return
		}
	}
}

func writeSessionEvent(writer interface {
	Write([]byte) (int, error)
	Flush()
}, event agentsession.Event) bool {
	data, err := json.Marshal(event)
	if err != nil {
		return true
	}
	if _, err := fmt.Fprintf(writer, "id: %d\nevent: message\ndata: %s\n\n", event.Seq, data); err != nil {
		return false
	}
	writer.Flush()
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// readSessionId takes the session out of a request that names nothing else.
func (c *ApiController) readSessionId() string {
	var request struct {
		Id string `json:"id"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return ""
	}
	if request.Id == "" {
		c.ResponseError("no session was named")
		return ""
	}
	return request.Id
}
