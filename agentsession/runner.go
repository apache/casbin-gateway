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

package agentsession

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agenthome"
)

// turnTimeout caps one exchange. An agent driven from a chat window has nobody
// watching the console it would otherwise hang in, so a turn that goes quiet is
// ended rather than left holding the session forever.
const turnTimeout = 30 * time.Minute

// stderrLimit is how much of a failed run's diagnostics is kept for the error
// message. Agents are talkative on the way down and only the tail is useful.
const stderrLimit = 4000

// run carries out one exchange and reports what went wrong, if anything.
func run(ctx context.Context, headless *agent.Headless, session Session, prompt string, emit func(Event)) error {
	launch := agent.LaunchOf(agent.Installation{
		AgentId: session.AgentId,
		Path:    session.AgentPath,
		Owner:   session.Owner,
	})
	if launch.Executable == "" {
		return errors.New("no launcher was found for this installation")
	}

	// A command line cannot carry a newline through a package manager's Windows
	// shim - cmd.exe ends the argument there - and a message typed in a chat
	// window routinely has one. An agent whose prompt goes on the command line
	// is therefore held to a single line rather than quietly losing the rest.
	if !headless.PromptStdin && strings.ContainsAny(prompt, "\r\n") {
		return errors.New("this agent takes its prompt on the command line, which cannot carry more than one line")
	}

	workDir, err := resolveWorkDir(session.WorkDir)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, turnTimeout)
	defer cancel()

	args := buildArgs(headless, session, prompt)
	cmd := newCommand(ctx, launch.Executable, args)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr := &boundedBuffer{limit: stderrLimit}
	cmd.Stderr = stderr

	// The prompt goes on stdin wherever the agent reads it there: it is the one
	// part of the command line that carries arbitrary text, and keeping it off
	// removes every quoting question with it. Stdin is closed either way, so an
	// agent that decides to ask something gets an answer instead of waiting.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		if headless.PromptStdin {
			io.WriteString(stdin, prompt)
		}
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	// Killing the process itself leaves what it started behind - every one of
	// these agents is a launcher in front of a runtime - so the tree goes.
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			killTree(cmd)
		case <-stopped:
		}
	}()

	parseErr := parse(headless.Format, stdout, emit)
	waitErr := cmd.Wait()
	close(stopped)

	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return fmt.Errorf("the agent was still running after %s and was stopped", turnTimeout)
		}
		return errors.New("stopped")
	}
	if waitErr != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return fmt.Errorf("%s: %s", waitErr, detail)
		}
		return waitErr
	}
	return parseErr
}

// buildArgs writes the command line for one turn. A session the agent can carry
// on is resumed; one it cannot starts over, which is the honest behaviour for an
// agent that publishes no way to continue.
func buildArgs(headless *agent.Headless, session Session, prompt string) []string {
	template := headless.Args
	if session.NativeId != "" && session.Turns > 0 && headless.CanResume() {
		template = headless.ResumeArgs
	}

	args := fill(template, session, prompt, headless.PromptStdin)
	if session.Model != "" && len(headless.ModelArgs) > 0 {
		args = append(args, fill(headless.ModelArgs, session, prompt, headless.PromptStdin)...)
	}
	return args
}

// fill replaces the placeholders, each of which is a whole argument. A prompt
// handed over on stdin drops out of the command line rather than being repeated
// on it.
func fill(template []string, session Session, prompt string, promptOnStdin bool) []string {
	args := make([]string, 0, len(template))
	for _, arg := range template {
		switch arg {
		case agent.PromptPlaceholder:
			if !promptOnStdin {
				args = append(args, prompt)
			}
		case agent.SessionPlaceholder:
			args = append(args, session.NativeId)
		case agent.ModelPlaceholder:
			args = append(args, session.Model)
		default:
			args = append(args, arg)
		}
	}
	return args
}

// resolveWorkDir is the directory the agent runs in. An empty one is the home of
// the account Gateway runs as, which is where an agent started by hand would
// have been.
func resolveWorkDir(workDir string) (string, error) {
	if workDir == "" {
		return agenthome.Resolve("")
	}

	info, err := os.Stat(workDir)
	if err != nil {
		return "", fmt.Errorf("the working directory cannot be opened: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", workDir)
	}
	return workDir, nil
}

// boundedBuffer keeps the tail of what was written to it.
type boundedBuffer struct {
	limit int
	buf   bytes.Buffer
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	written, err := b.buf.Write(p)
	if b.buf.Len() > b.limit {
		kept := b.buf.Bytes()[b.buf.Len()-b.limit:]
		trimmed := bytes.NewBuffer(append([]byte{}, kept...))
		b.buf = *trimmed
	}
	return written, err
}

func (b *boundedBuffer) String() string {
	return b.buf.String()
}
