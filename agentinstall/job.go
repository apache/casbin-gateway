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

package agentinstall

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/proxy"
)

const (
	// jobTimeout bounds one install: a package manager that is waiting for
	// something is never going to get it here, since nothing is attached to its
	// input.
	jobTimeout = 15 * time.Minute
	// maxOutput is the tail of the manager's own output kept for the page. The
	// end is what says why an install failed.
	maxOutput = 16 * 1024
)

// Job is one install or upgrade, running or finished. The page polls it rather
// than holding a request open for as long as a package manager takes.
type Job struct {
	AgentId string `json:"agentId"`
	Name    string `json:"name"`
	Action  string `json:"action"`
	Manager string `json:"manager"`
	Command string `json:"command"`
	// Version is the release the job pinned, empty for the current one.
	Version string `json:"version,omitempty"`
	// Interactive marks a job that put a window on screen and is waiting for
	// whoever is at the machine, rather than one working on its own.
	Interactive bool `json:"interactive,omitempty"`
	Running     bool `json:"running"`
	// Ok is the outcome of a finished job, false while one runs.
	Ok bool `json:"ok"`
	// Output is the tail of what the package manager printed, on both streams.
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime,omitempty"`
}

type job struct {
	sync.Mutex
	Job
	output tail
}

var jobs = struct {
	sync.Mutex
	byAgent map[string]*job
}{byAgent: map[string]*job{}}

// Start runs a plan in the background and reports the job it started. One agent
// runs one job at a time: a second install of the same package would fight the
// first over the same directory.
func Start(plan Plan) (Job, error) {
	if !plan.Available {
		return Job{}, errors.New(plan.Detail)
	}

	jobs.Lock()
	if running, ok := jobs.byAgent[plan.AgentId]; ok && running.snapshot().Running {
		jobs.Unlock()
		return Job{}, errors.New("this agent is already being installed or upgraded")
	}
	started := &job{Job: Job{
		AgentId:     plan.AgentId,
		Name:        agent.DisplayNameOf(plan.AgentId),
		Action:      plan.Action,
		Manager:     plan.Manager,
		Command:     plan.Command,
		Version:     plan.Version,
		Interactive: plan.Interactive,
		Running:     true,
		StartTime:   now(),
	}}
	jobs.byAgent[plan.AgentId] = started
	jobs.Unlock()

	go run(started, plan)
	return started.snapshot(), nil
}

// JobOf is the running or last finished job of one agent.
func JobOf(agentId string) (Job, bool) {
	jobs.Lock()
	defer jobs.Unlock()

	found, ok := jobs.byAgent[agentId]
	if !ok {
		return Job{}, false
	}
	return found.snapshot(), true
}

// Jobs lists every job of this process, newest first.
func Jobs() []Job {
	jobs.Lock()
	result := make([]Job, 0, len(jobs.byAgent))
	for _, item := range jobs.byAgent {
		result = append(result, item.snapshot())
	}
	jobs.Unlock()

	sort.SliceStable(result, func(left, right int) bool {
		return result[left].StartTime > result[right].StartTime
	})
	return result
}

func run(started *job, plan Plan) {
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, plan.program, plan.args...)
	cmd.Stdout, cmd.Stderr = &started.output, &started.output
	// A manager run from the agent's own tree would pick up a project's
	// configuration; the home directory is the neutral one.
	if home, err := os.UserHomeDir(); err == nil {
		cmd.Dir = home
	}
	// Nothing is attached to the input, so a manager that asks a question would
	// hang until the timeout. These say to answer none.
	cmd.Env = append(os.Environ(), "CI=1", "NO_COLOR=1", "npm_config_yes=true", "HOMEBREW_NO_AUTO_UPDATE=1")
	// A manager downloads through its own network stack, not Gateway's, so the
	// configured proxy has to be handed to it.
	cmd.Env = append(cmd.Env, proxy.Env()...)
	// An uninstaller with no silent switch, and the consent prompt Windows
	// raises for a machine-wide change, both need to be on screen: hiding them
	// would leave the job waiting on a dialog nobody can answer.
	if !plan.Interactive {
		hideWindow(cmd)
	}

	err := cmd.Run()
	if ctx.Err() != nil {
		err = errors.New("the install did not finish within " + jobTimeout.String())
	}

	started.Lock()
	started.Running = false
	started.Ok = err == nil
	if err != nil {
		started.Error = err.Error()
	}
	started.EndTime = now()
	started.Unlock()

	// Whatever the outcome, what is on disk changed: the next listing has to be
	// read from the host rather than from the scan that ran before this.
	_, _ = agent.Scan(true)
}

func (j *job) snapshot() Job {
	j.Lock()
	defer j.Unlock()

	result := j.Job
	result.Output = j.output.String()
	return result
}

// tail keeps the last maxOutput bytes of a stream, so an install that prints a
// progress bar for ten minutes cannot grow without bound.
type tail struct {
	sync.Mutex
	buffer bytes.Buffer
}

func (t *tail) Write(data []byte) (int, error) {
	t.Lock()
	defer t.Unlock()

	written := len(data)
	t.buffer.Write(data)
	if t.buffer.Len() > maxOutput {
		kept := t.buffer.Bytes()[t.buffer.Len()-maxOutput:]
		trimmed := bytes.Buffer{}
		trimmed.Write(kept)
		t.buffer = trimmed
	}
	return written, nil
}

func (t *tail) String() string {
	t.Lock()
	defer t.Unlock()

	// Carriage returns are how a progress bar overwrites its own line, and they
	// read as blank lines once the output is shown as text.
	return strings.TrimSpace(strings.ReplaceAll(t.buffer.String(), "\r", "\n"))
}

func now() string {
	return time.Now().Format(time.RFC3339)
}
