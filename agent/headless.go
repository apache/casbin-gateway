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

package agent

// Placeholders one of Headless's command lines carries. Each is replaced whole:
// a placeholder is always an argument of its own, never part of one, so nothing
// here has to be quoted or escaped.
const (
	PromptPlaceholder  = "{prompt}"
	SessionPlaceholder = "{session}"
	ModelPlaceholder   = "{model}"
)

// Headless is how one prompt is handed to an agent without a terminal, which is
// what lets Gateway drive it on somebody's behalf.
//
// Args and ResumeArgs are whole command lines rather than a base and an
// addition: the agents disagree about where a resumed prompt goes - Codex takes
// a subcommand, the Claude Code family a flag - and writing both out is what
// keeps that disagreement out of the code.
type Headless struct {
	// Args starts a new conversation. An agent that carries SessionPlaceholder
	// here is told which session it is starting, so Gateway names it rather than
	// reading the name back out of the output.
	Args []string `json:"args"`
	// ResumeArgs carries on the conversation Gateway last started. Empty for an
	// agent that cannot resume one, whose every turn then stands alone.
	ResumeArgs []string `json:"resumeArgs,omitempty"`
	// ModelArgs are appended when a model was picked for the session, and left
	// out entirely when none was, so the agent keeps its own default.
	ModelArgs []string `json:"modelArgs,omitempty"`
	// Format names the parser for what the agent writes to stdout.
	Format string `json:"format"`
	// PromptStdin hands the prompt on stdin instead of the command line. It is
	// what every agent driven from a chat window needs: a package manager's
	// Windows shim runs through cmd.exe, which ends an argument at the first
	// newline, and a typed message routinely has one.
	PromptStdin bool `json:"promptStdin,omitempty"`
}

// CanResume reports whether the agent carries a conversation across turns.
func (h *Headless) CanResume() bool {
	return len(h.ResumeArgs) > 0
}

// NamesSession reports whether the agent is told the id of the session it is
// starting. One that is not leaves the id to be read out of its output.
func (h *Headless) NamesSession() bool {
	for _, arg := range h.Args {
		if arg == SessionPlaceholder {
			return true
		}
	}
	return false
}

// HeadlessOf is how one agent is driven without a terminal, nil for an agent
// that publishes no such mode. It reads the fingerprints rather than a host
// scan, so the answer is known while the agent is not installed.
func HeadlessOf(agentId string) *Headless {
	for i := range fingerprints {
		if fingerprints[i].ID == agentId {
			return fingerprints[i].Headless
		}
	}
	return nil
}

// DrivableAgents are the ids of every agent Gateway can drive.
func DrivableAgents() []string {
	result := []string{}
	for i := range fingerprints {
		if fingerprints[i].Headless != nil {
			result = append(result, fingerprints[i].ID)
		}
	}
	return result
}
