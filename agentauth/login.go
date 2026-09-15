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

var (
	// loginTimeout is the last resort for a sign-in nobody cancelled. A page
	// closed without finishing is not something the agent can see, so this is
	// what ends it - long enough that signing in slowly is not cut short.
	loginTimeout = 10 * time.Minute
	// urlWait is how long the start holds its answer for the address the browser
	// has to open. The agent opens it itself as well, so the wait is short.
	urlWait = 10 * time.Second
	// stopWait bounds the wait for a stopped agent to let go of its output. The
	// session ends either way: it must not hold the next sign-in behind it.
	stopWait = 5 * time.Second
	// finishedTtl is how long a finished sign-in stays readable, so the page
	// polling it still sees how it ended.
	finishedTtl = 10 * time.Minute
)

// maxLoginOutput is the tail of the agent's own output kept for the page.
const maxLoginOutput = 8 * 1024

// How a sign-in ended, when it was not the browser that ended it.
var (
	errLoginCancelled = errors.New("the sign-in was cancelled")
	errLoginTimedOut  = errors.New("the sign-in was not finished in the browser in time")
	errLoginEnded     = errors.New("the sign-in ended before it signed anybody in")
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
	// Cancelled marks a sign-in somebody ended rather than one that went wrong.
	Cancelled bool `json:"cancelled,omitempty"`
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
	// endedAt is when the session finished, for forgetting it later.
	endedAt time.Time
	// reason is what to report when the agent is stopped rather than left to
	// finish: killing it makes its own exit status meaningless.
	reason error
	url    chan string
	ctx    context.Context
	// cancel ends the agent and everything it started.
	cancel context.CancelFunc
	// done is closed once the session is finished and the agent is gone.
	done chan struct{}
}

var sessions = struct {
	sync.Mutex
	byId map[string]*session
	// active is the sign-in holding the agent's login port, nil when none is.
	active *session
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

	started, err := claim(agentId)
	if err != nil {
		return Session{}, err
	}

	go started.run(executable, save)

	// The address is what the page needs to show, so the start waits for the
	// agent to print it rather than answering with a session that says nothing.
	// A sign-in that is already over has none to wait for.
	select {
	case url := <-started.url:
		started.Lock()
		started.Url = url
		started.Unlock()
	case <-started.done:
	case <-time.After(urlWait):
	}
	return started.snapshot(), nil
}

// claim takes the one sign-in slot there is. The agent's sign-in listens on a
// fixed port, so a second one would fail on the first one's listener rather
// than sign anybody in - but only a sign-in that is genuinely still waiting
// holds the slot, since every other one has been ended and released.
func claim(agentId string) (*session, error) {
	sessions.Lock()
	defer sessions.Unlock()

	if active := sessions.active; active != nil {
		active.Lock()
		running := active.Running
		active.Unlock()
		if running {
			return nil, errors.New("a sign-in is already waiting for the browser")
		}
		sessions.active = nil
	}
	forgetFinishedLocked()

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	started := &session{
		Session: Session{
			Id:        util.GenerateToken(16),
			AgentId:   agentId,
			Running:   true,
			StartTime: util.GetCurrentTime(),
		},
		url:    make(chan string, 1),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
	sessions.byId[started.Id] = started
	sessions.active = started
	return started, nil
}

// release frees the slot a finished sign-in held, so the next one can start.
func release(finished *session) {
	sessions.Lock()
	defer sessions.Unlock()
	if sessions.active == finished {
		sessions.active = nil
	}
	forgetFinishedLocked()
}

// forgetFinishedLocked drops the sessions nobody is polling any more.
func forgetFinishedLocked() {
	for id, kept := range sessions.byId {
		if kept == sessions.active {
			continue
		}
		kept.Lock()
		stale := !kept.Running && time.Since(kept.endedAt) > finishedTtl
		kept.Unlock()
		if stale {
			delete(sessions.byId, id)
		}
	}
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

// CancelLogin ends a sign-in still waiting for a browser nobody is coming back
// from, freeing the port the agent holds and the slot the next one needs.
// Cancelling a finished sign-in leaves how it finished alone.
func CancelLogin(id string) (Session, error) {
	sessions.Lock()
	found, ok := sessions.byId[id]
	sessions.Unlock()
	if !ok {
		return Session{}, errors.New("no sign-in was started under this id")
	}
	return found.stop(errLoginCancelled), nil
}

// stop ends the agent and the session with it. The wait is bounded: an agent
// that does not let go of its output must not hold the next sign-in behind it.
func (s *session) stop(reason error) Session {
	s.Lock()
	running := s.Running
	if running {
		s.reason = reason
	}
	s.Unlock()
	if !running {
		return s.snapshot()
	}

	s.cancel()
	select {
	case <-s.done:
	case <-time.After(stopWait):
	}
	return s.finish(nil, reason)
}

func (s *session) snapshot() Session {
	s.Lock()
	defer s.Unlock()
	return s.Session
}

func (s *session) stopReason() error {
	s.Lock()
	defer s.Unlock()
	return s.reason
}

func (s *session) run(executable string, save func(Credential) error) {
	// The session is finished whichever way this returns, so the slot is freed
	// even down a path that did not say how it ended.
	defer close(s.done)
	defer s.finish(nil, errLoginEnded)
	defer s.cancel()

	home, err := os.MkdirTemp("", "casbin-gateway-signin-")
	if err != nil {
		s.finish(nil, err)
		return
	}
	defer os.RemoveAll(home)

	command := exec.CommandContext(s.ctx, executable, "login")
	command.Env = append(os.Environ(), "CODEX_HOME="+home)
	command.Dir = home
	// The agent goes as a tree: on Windows it is reached through a shim, and
	// killing that alone leaves the process holding the port behind.
	command.Cancel = func() error { return terminate(command) }
	prepare(command)

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

	// A stopped agent's exit status says only that it was stopped, so what
	// stopped it is reported instead.
	if reason := s.stopReason(); reason != nil {
		s.finish(nil, reason)
		return
	}
	if errors.Is(s.ctx.Err(), context.DeadlineExceeded) {
		s.finish(nil, errLoginTimedOut)
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

// finish ends the session, once: the first caller there says how it went, and
// whoever follows - the goroutine behind a cancel, the safety net behind both -
// leaves that answer alone.
func (s *session) finish(credential *Credential, err error) Session {
	s.Lock()
	if !s.Running {
		done := s.Session
		s.Unlock()
		return done
	}
	s.Running = false
	s.endedAt = time.Now()
	s.EndTime = util.GetCurrentTime()
	if err != nil {
		s.Error = err.Error()
		s.Cancelled = errors.Is(err, errLoginCancelled)
	} else {
		s.Ok = true
		if credential != nil {
			s.Account = credential.Label()
		}
	}
	done := s.Session
	s.Unlock()

	release(s)
	return done
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
