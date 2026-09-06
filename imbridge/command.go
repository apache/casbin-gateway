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

package imbridge

import (
	"fmt"
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentsession"
)

const helpText = `/new — start a fresh conversation
/status — which agent, where, how many turns
/agents — the agents on this machine that can be driven
/agent <id> — talk to a different one, from the next message
/dir <path> — work somewhere else, from the next message
/model <name> — pick a model, empty to use the agent's own
/stop — stop what the agent is doing now
/help — this`

// command runs one slash command and reports what to say back. Changing where a
// conversation happens - the agent, the directory, the model - ends it and opens
// another: an agent is told its working directory when it starts, and carrying a
// half-finished conversation into a different project would be a lie.
func (r *router) command(message Message, text string) string {
	name, argument, _ := strings.Cut(strings.TrimPrefix(text, "/"), " ")
	argument = strings.TrimSpace(argument)

	switch strings.ToLower(name) {
	case "help", "start":
		return helpText

	case "new":
		r.closeSession(message)
		return "Started fresh. The next message opens a new conversation."

	case "status":
		return r.status(message)

	case "agents":
		return r.agents()

	case "agent":
		return r.rebind(message, func(spec *agentsession.Spec) string {
			installation, err := findDrivable(argument)
			if err != nil {
				return err.Error()
			}
			spec.AgentId = installation.AgentId
			spec.AgentPath = installation.Path
			spec.Owner = installation.Owner
			return ""
		}, "Talking to "+argument+" from the next message.")

	case "dir", "cd":
		return r.rebind(message, func(spec *agentsession.Spec) string {
			spec.WorkDir = argument
			return ""
		}, "Working in "+displayDir(argument)+" from the next message.")

	case "model":
		return r.rebind(message, func(spec *agentsession.Spec) string {
			spec.Model = argument
			return ""
		}, modelReply(argument))

	case "stop":
		session, found := r.current(message)
		if !found {
			return "Nothing is running."
		}
		agentsession.Interrupt(session.Id)
		return "Stopped."
	}

	return "No such command. /help lists them."
}

func (r *router) status(message Message) string {
	session, found := r.current(message)
	if !found {
		return fmt.Sprintf("No conversation yet. The next message starts one with %s in %s.",
			nameOr(r.channel.AgentId, "no agent"), displayDir(r.channel.WorkDir))
	}

	lines := []string{
		"Agent: " + nameOr(session.AgentId, "none"),
		"Directory: " + displayDir(session.WorkDir),
		fmt.Sprintf("Turns: %d", session.Turns),
		"State: " + string(session.State),
	}
	if session.Model != "" {
		lines = append(lines, "Model: "+session.Model)
	}
	if !session.Resumable {
		lines = append(lines, "This agent keeps nothing between messages.")
	}
	if session.LastError != "" {
		lines = append(lines, "Last error: "+session.LastError)
	}
	return strings.Join(lines, "\n")
}

func (r *router) agents() string {
	installations, err := agent.Scan(false)
	if err != nil {
		return err.Error()
	}

	lines := []string{}
	for _, installation := range installations {
		if agent.HeadlessOf(installation.AgentId) == nil {
			continue
		}
		lines = append(lines, "/agent "+installation.AgentId+" — "+installation.Name)
	}
	if len(lines) == 0 {
		return "No agent on this machine publishes a mode Gateway can drive."
	}
	return strings.Join(lines, "\n")
}

// rebind ends the conversation and opens another with one thing about it
// changed. Returns what to say back.
func (r *router) rebind(message Message, change func(*agentsession.Spec) string, done string) string {
	spec := agentsession.Spec{
		AgentId:   r.channel.AgentId,
		AgentPath: r.channel.AgentPath,
		Owner:     r.channel.AgentUser,
		WorkDir:   r.channel.WorkDir,
		Model:     r.channel.Model,
		Source:    sourceOf(message),
	}
	if session, found := r.current(message); found {
		spec.AgentId = session.AgentId
		spec.AgentPath = session.AgentPath
		spec.Owner = session.Owner
		spec.WorkDir = session.WorkDir
		spec.Model = session.Model
	}

	if failure := change(&spec); failure != "" {
		return failure
	}

	r.closeSession(message)
	if _, err := agentsession.Open(spec); err != nil {
		return err.Error()
	}
	return done
}

func (r *router) current(message Message) (agentsession.Session, bool) {
	source := sourceOf(message)
	for _, candidate := range agentsession.List() {
		if candidate.Source == source {
			return candidate, true
		}
	}
	return agentsession.Session{}, false
}

func (r *router) closeSession(message Message) {
	if session, found := r.current(message); found {
		agentsession.Close(session.Id)
		forgetSession(session.Id)
	}
}

// findDrivable resolves an agent id to an installation that can be driven.
func findDrivable(agentId string) (agent.Installation, error) {
	if agentId == "" {
		return agent.Installation{}, fmt.Errorf("name an agent: /agents lists them")
	}
	if agent.HeadlessOf(agentId) == nil {
		return agent.Installation{}, fmt.Errorf("%s publishes no mode Gateway can drive", agentId)
	}

	installations, err := agent.Scan(false)
	if err != nil {
		return agent.Installation{}, err
	}
	for _, installation := range installations {
		if installation.AgentId == agentId {
			return installation, nil
		}
	}
	return agent.Installation{}, fmt.Errorf("%s is not installed here", agentId)
}

func displayDir(workDir string) string {
	if workDir == "" {
		return "the home directory"
	}
	return workDir
}

func nameOr(agentId, fallback string) string {
	if name := agent.DisplayNameOf(agentId); name != "" {
		return name
	}
	if agentId != "" {
		return agentId
	}
	return fallback
}

func modelReply(model string) string {
	if model == "" {
		return "Using the agent's own model from the next message."
	}
	return "Using " + model + " from the next message."
}
