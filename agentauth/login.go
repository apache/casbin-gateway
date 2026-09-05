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

package agentauth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/util"
)

const (
	// loginTimeout bounds one sign-in. The agent's login server sits waiting for
	// a browser that may never come back.
	loginTimeout = 10 * time.Minute
	// urlWait is how long the start holds its answer for the address the browser
	// has to open. The agent opens it itself as well, so the wait is short.
	urlWait = 10 * time.Second
	// maxLoginOutput is the tail of the agent's own output kept for the page.
	maxLoginOutput = 8 * 1024
)

// Session is one sign-in Gateway started, running or finished. A browser
// sign-in takes as long as whoever is at the machine, so the page polls this
// rather than holding a request open.
type Session struct {
	Id      string `json:"id"`
	AgentId string `json:"agentId"`
	// Url is the address the sign-in has to be finished at. The agent opens it
	// itself; this is for the machine where it could not.
	Url     string `json:"url,omitempty"`
	Running bool   `json:"running"`
	// Ok is the outcome of a finished sign-in, false while one runs.
	Ok bool `json:"ok"`
	// Account names who signed in, once one has.
	Account   string `json:"account,omitempty"`
	Error     string `json:"error,omitempty"`
	Output    string `json:"output,omitempty"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime,omitempty"`
}

type session struct {
	sync.Mutex
	Session
	url chan string
}

var sessions = struct {
	sync.Mutex
	byId map[string]*session
}{byId: map[string]*session{}}

// urlPattern picks the address out of what the agent prints. The sign-in server
// it starts is announced over plain http on this machine; the one to open is
// the https address of the provider.
var urlPattern = regexp.MustCompile(`https://[^\s"']+`)

// StartLogin runs the agent's own sign-in against a directory of its own, so
// the account it brings back is captured without disturbing the one in use.
// save stores the finished credential; it runs on the background goroutine,
// once, and its error is what the session reports.
func StartLogin(agentId string, executable string, save func(Credential) error) (Session, error) {
	if !Supports(agentId) {
		return Session{}, fmt.Errorf("gateway cannot sign %s in", agentId)
	}
	if executable == "" {
		return Session{}, errors.New("gateway found no Codex program to sign in with")
	}

	sessions.Lock()
	for _, running := range sessions.byId {
		// The agent's sign-in listens on a fixed port, so a second one would
		// fail on the first one's listener rather than sign anybody in.
		if running.snapshot().Running {
			sessions.Unlock()
			return Session{}, errors.New("a sign-in is already waiting for the browser")
		}
	}
	started := &session{
		Session: Session{
			Id:        util.GenerateToken(16),
			AgentId:   agentId,
			Running:   true,
			StartTime: util.GetCurrentTime(),
		},
		url: make(chan string, 1),
	}
	sessions.byId[started.Id] = started
	sessions.Unlock()

	go started.run(executable, save)

	// The address is what the page needs to show, so the start waits for the
	// agent to print it rather than answering with a session that says nothing.
	select {
	case url := <-started.url:
		started.Lock()
		started.Url = url
		started.Unlock()
	case <-time.After(urlWait):
	}
	return started.snapshot(), nil
}

// LoginSession is one sign-in as the page last left it.
func LoginSession(id string) (Session, bool) {
	sessions.Lock()
	found, ok := sessions.byId[id]
	sessions.Unlock()
	if !ok {
		return Session{}, false
	}
	return found.snapshot(), true
}

func (s *session) snapshot() Session {
	s.Lock()
	defer s.Unlock()
	return s.Session
}

func (s *session) run(executable string, save func(Credential) error) {
	home, err := os.MkdirTemp("", "casbin-gateway-signin-")
	if err != nil {
		s.finish(nil, err)
		return
	}
	defer os.RemoveAll(home)

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()

	command := exec.CommandContext(ctx, executable, "login")
	command.Env = append(os.Environ(), "CODEX_HOME="+home)
	command.Dir = home
	hideWindow(command)

	output, err := command.StdoutPipe()
	if err != nil {
		s.finish(nil, err)
		return
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		s.finish(nil, err)
		return
	}

	printed := s.readOutput(output)
	waitErr := command.Wait()
	if ctx.Err() != nil {
		s.finish(nil, errors.New("the sign-in was not finished in the browser in time"))
		return
	}

	credential, err := Read(s.AgentId, home)
	if err != nil {
		s.finish(nil, err)
		return
	}
	if credential == nil {
		if waitErr != nil {
			s.finish(nil, fmt.Errorf("%s: %s", waitErr, lastLine(printed)))
			return
		}
		s.finish(nil, errors.New("the sign-in left no account behind"))
		return
	}
	s.finish(credential, save(*credential))
}

// readOutput follows what the agent prints until it closes the stream, handing
// the first address it names to whoever is waiting for it.
func (s *session) readOutput(stream io.Reader) string {
	tail := &strings.Builder{}
	found := false
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		tail.WriteString(line + "\n")
		if !found {
			if url := urlPattern.FindString(line); url != "" {
				found = true
				s.url <- url
			}
		}
		s.Lock()
		s.Output = keepTail(tail.String())
		s.Unlock()
	}
	return tail.String()
}

func (s *session) finish(credential *Credential, err error) {
	s.Lock()
	defer s.Unlock()
	s.Running = false
	s.EndTime = util.GetCurrentTime()
	if err != nil {
		s.Error = err.Error()
		return
	}
	s.Ok = true
	if credential != nil {
		s.Account = credential.Label()
	}
}

func keepTail(text string) string {
	if len(text) <= maxLoginOutput {
		return text
	}
	return text[len(text)-maxLoginOutput:]
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
