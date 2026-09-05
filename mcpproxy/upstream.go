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
	"os"
	"os/exec"
	"strings"
	"sync"
)

// upstream is the MCP server one connection actually reaches, whether that is a
// process started here or a service reached over HTTP.
//
// Start is given the function that delivers a server message to the agent,
// because the two transports produce those at different times: a stdio server
// writes whenever it likes, an HTTP one answers a request.
type upstream interface {
	Start(emit func([]byte) error) error
	Send(line []byte) error
	Close() error
	// Ended is closed when the server has gone, so a caller waiting for a reply
	// can stop waiting instead of sitting out its whole timeout. A transport
	// that has nothing to end returns nil, which blocks forever, which is right.
	Ended() <-chan struct{}
	// Diagnostics is whatever the server said on its way out, for an error
	// message that names the cause rather than only the symptom.
	Diagnostics() string
}

func openUpstream(resolved *resolvedServer) (upstream, error) {
	switch resolved.Transport {
	case "stdio":
		return &stdioUpstream{resolved: resolved}, nil
	case "http":
		return newHttpUpstream(resolved), nil
	default:
		return nil, fmt.Errorf("unknown transport %q for this connection", resolved.Transport)
	}
}

// stdioUpstream runs the server as a child process. The credential reaches it
// through the environment and arguments Gateway rendered, and never touches the
// agent's own configuration file.
type stdioUpstream struct {
	resolved *resolvedServer
	command  *exec.Cmd
	stdin    interface {
		Write([]byte) (int, error)
		Close() error
	}
	writing sync.Mutex
	ended   chan struct{}
	stderr  *tailWriter
}

func (u *stdioUpstream) Ended() <-chan struct{} { return u.ended }

func (u *stdioUpstream) Diagnostics() string {
	if u.stderr == nil {
		return ""
	}
	return u.stderr.tail()
}

func (u *stdioUpstream) Start(emit func([]byte) error) error {
	u.ended = make(chan struct{})
	u.stderr = &tailWriter{}

	command := exec.Command(u.resolved.Command, u.resolved.Args...)
	command.Env = append(os.Environ(), u.resolved.envPairs()...)
	// The child's own diagnostics belong on this process's stderr, where the
	// agent shows them, rather than mixed into the JSON-RPC stream. A copy is
	// kept so a failure can be reported with what the server actually said.
	command.Stderr = io.MultiWriter(os.Stderr, u.stderr)

	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("cannot start %s: %w", u.resolved.Command, err)
	}
	u.command, u.stdin = command, stdin

	go func() {
		// Closing this is what tells a caller the server has gone: stdout ends
		// when the process does, whether it exited or was killed.
		defer close(u.ended)
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())
			if len(line) == 0 {
				continue
			}
			if emit(append([]byte(nil), line...)) != nil {
				return
			}
		}
	}()
	return nil
}

func (u *stdioUpstream) Send(line []byte) error {
	u.writing.Lock()
	defer u.writing.Unlock()
	_, err := u.stdin.Write(append(line, '\n'))
	return err
}

func (u *stdioUpstream) Close() error {
	if u.command == nil {
		return nil
	}
	if u.stdin != nil {
		// Closing stdin is how an MCP server is asked to stop; killing it
		// outright would leave whatever it was writing half-written.
		_ = u.stdin.Close()
	}
	return u.command.Wait()
}

// tailWriter keeps the last few lines written through it. A server that fails
// usually says why on its way out, and that is the one thing worth quoting back.
type tailWriter struct {
	mutex sync.Mutex
	lines []string
}

const tailLines = 5

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	for _, line := range strings.Split(string(p), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			w.lines = append(w.lines, line)
		}
	}
	if len(w.lines) > tailLines {
		w.lines = w.lines[len(w.lines)-tailLines:]
	}
	return len(p), nil
}

func (w *tailWriter) tail() string {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return strings.Join(w.lines, "; ")
}
