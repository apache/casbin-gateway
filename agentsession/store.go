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
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/apache/casbin-gateway/agent"
	"github.com/google/uuid"
)

var (
	ErrNoSession   = errors.New("no such session")
	ErrNotDrivable = errors.New("this agent publishes no non-interactive mode, so Gateway cannot drive it")
	ErrBusy        = errors.New("this session is still answering")
)

const (
	// historyLimit caps what one session replays to a page that connects while a
	// turn is already running. A turn that says more than this is truncated at
	// the front rather than kept whole in memory.
	historyLimit = 4000
	// feedBuffer is how far one subscriber may fall behind before its events are
	// dropped. Dropping is deliberate: a page nobody is looking at must not hold
	// the agent up.
	feedBuffer = 1024
)

// live is one session and whatever is listening to it.
type live struct {
	mu          sync.Mutex
	session     Session
	events      []Event
	nextSeq     int64
	subscribers map[int]chan Event
	nextId      int
	cancel      context.CancelFunc
}

var registry = struct {
	sync.Mutex
	sessions map[string]*live
}{sessions: map[string]*live{}}

// SessionSink is where sessions are kept, so they outlive the process. Storage
// lives outside this package for the same reason it does for the monitor:
// driving an agent must not depend on the database.
type SessionSink func(*Session)

var sessionSink struct {
	sync.RWMutex
	sink SessionSink
}

// SetSessionSink installs the store every change to a session is written to.
func SetSessionSink(sink SessionSink) {
	sessionSink.Lock()
	sessionSink.sink = sink
	sessionSink.Unlock()
}

func persist(session Session) {
	sessionSink.RLock()
	sink := sessionSink.sink
	sessionSink.RUnlock()
	if sink != nil {
		sink(&session)
	}
}

// Open starts a session against one installation. Nothing runs yet: an agent is
// only started when there is something to ask it.
func Open(spec Spec) (Session, error) {
	headless := agent.HeadlessOf(spec.AgentId)
	if headless == nil {
		return Session{}, ErrNotDrivable
	}

	entry := &live{
		session: Session{
			Id:          uuid.NewString(),
			Spec:        spec,
			Resumable:   headless.CanResume(),
			State:       StateIdle,
			CreatedTime: now(),
			UpdatedTime: now(),
		},
		subscribers: map[int]chan Event{},
	}
	// An agent that takes the id of the session it is starting is told ours, so
	// the next turn resumes it without anything having to be read back.
	if headless.NamesSession() {
		entry.session.NativeId = entry.session.Id
	}

	registry.Lock()
	registry.sessions[entry.session.Id] = entry
	registry.Unlock()

	persist(entry.session)
	return entry.session, nil
}

// Restore puts a session kept from an earlier run back in the registry, idle.
func Restore(session Session) {
	session.State = StateIdle
	entry := &live{session: session, subscribers: map[int]chan Event{}}

	registry.Lock()
	registry.sessions[session.Id] = entry
	registry.Unlock()
}

func lookup(id string) (*live, error) {
	registry.Lock()
	entry, ok := registry.sessions[id]
	registry.Unlock()
	if !ok {
		return nil, ErrNoSession
	}
	return entry, nil
}

// Get is one session as it stands now.
func Get(id string) (Session, error) {
	entry, err := lookup(id)
	if err != nil {
		return Session{}, err
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.session, nil
}

// List is every session, newest first by the caller's own ordering.
func List() []Session {
	registry.Lock()
	entries := make([]*live, 0, len(registry.sessions))
	for _, entry := range registry.sessions {
		entries = append(entries, entry)
	}
	registry.Unlock()

	result := make([]Session, 0, len(entries))
	for _, entry := range entries {
		entry.mu.Lock()
		result = append(result, entry.session)
		entry.mu.Unlock()
	}
	return result
}

// Close ends a session: whatever it is running is stopped, and it is dropped
// from the registry. What was stored of it is the caller's to remove.
func Close(id string) error {
	entry, err := lookup(id)
	if err != nil {
		return err
	}

	entry.mu.Lock()
	if entry.cancel != nil {
		entry.cancel()
	}
	for _, feed := range entry.subscribers {
		close(feed)
	}
	entry.subscribers = map[int]chan Event{}
	entry.mu.Unlock()

	registry.Lock()
	delete(registry.sessions, id)
	registry.Unlock()
	return nil
}

// Interrupt stops the turn a session is in the middle of, leaving the session
// itself open for the next one.
func Interrupt(id string) error {
	entry, err := lookup(id)
	if err != nil {
		return err
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.cancel == nil {
		return nil
	}
	entry.cancel()
	return nil
}

// Subscribe hands back what the session has said so far and a feed of what it
// says next, so a page that connects in the middle of a turn misses nothing.
// Only what came after "seen" is replayed, which is how a page that reconnects
// picks up where it left off instead of reading the conversation twice.
func Subscribe(id string, seen int64) (int, <-chan Event, []Event, error) {
	entry, err := lookup(id)
	if err != nil {
		return 0, nil, nil, err
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.nextId++
	subscriberId := entry.nextId
	feed := make(chan Event, feedBuffer)
	entry.subscribers[subscriberId] = feed

	history := []Event{}
	for _, event := range entry.events {
		if event.Seq > seen {
			history = append(history, event)
		}
	}
	return subscriberId, feed, history, nil
}

// Unsubscribe drops one feed. Closing is left to this side so that a publish
// racing with it cannot write to a closed channel.
func Unsubscribe(id string, subscriberId int) {
	entry, err := lookup(id)
	if err != nil {
		return
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if feed, ok := entry.subscribers[subscriberId]; ok {
		delete(entry.subscribers, subscriberId)
		close(feed)
	}
}

// publish records one event against the session and hands it to every listener.
func (entry *live) publish(event Event) {
	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.nextSeq++
	event.Seq = entry.nextSeq
	entry.events = append(entry.events, event)
	if len(entry.events) > historyLimit {
		entry.events = entry.events[len(entry.events)-historyLimit:]
	}
	for _, feed := range entry.subscribers {
		select {
		case feed <- event:
		default:
		}
	}
}

// Send asks the session's agent one thing. It returns as soon as the agent has
// been started: the answer arrives on the feed.
func Send(id, prompt string) (Session, error) {
	entry, err := lookup(id)
	if err != nil {
		return Session{}, err
	}

	entry.mu.Lock()
	if entry.session.State == StateRunning {
		entry.mu.Unlock()
		return Session{}, ErrBusy
	}

	headless := agent.HeadlessOf(entry.session.AgentId)
	if headless == nil {
		entry.mu.Unlock()
		return Session{}, ErrNotDrivable
	}

	if entry.session.Title == "" {
		entry.session.Title = titleFrom(prompt)
	}
	entry.session.State = StateRunning
	entry.session.LastError = ""
	entry.session.UpdatedTime = now()
	session := entry.session

	ctx, cancel := context.WithCancel(context.Background())
	entry.cancel = cancel
	entry.mu.Unlock()

	entry.publish(textEvent(EventPrompt, prompt))
	persist(session)

	go entry.turn(ctx, cancel, headless, prompt)
	return session, nil
}

// turn runs one exchange to its end and leaves the session ready for the next.
func (entry *live) turn(ctx context.Context, cancel context.CancelFunc, headless *agent.Headless, prompt string) {
	defer cancel()

	entry.mu.Lock()
	session := entry.session
	entry.mu.Unlock()

	nativeId := ""
	runErr := run(ctx, headless, session, prompt, func(event Event) {
		if event.NativeId != "" {
			nativeId = event.NativeId
		}
		entry.publish(event)
	})

	entry.mu.Lock()
	entry.cancel = nil
	entry.session.Turns++
	entry.session.UpdatedTime = now()
	// An agent that names its own conversation only says so once it has started
	// one, so the id is taken from the first turn that reported it.
	if nativeId != "" && entry.session.NativeId == "" {
		entry.session.NativeId = nativeId
	}
	if runErr != nil {
		entry.session.State = StateFailed
		entry.session.LastError = runErr.Error()
	} else {
		entry.session.State = StateIdle
	}
	session = entry.session
	entry.mu.Unlock()

	if runErr != nil {
		entry.publish(errorEvent(runErr))
	}
	done := newEvent(EventDone)
	done.NativeId = session.NativeId
	entry.publish(done)

	persist(session)
}

// Drivable reports whether an agent can be driven, and why not when it cannot.
func Drivable(agentId string) error {
	if agent.HeadlessOf(agentId) == nil {
		return fmt.Errorf("%s: %w", agent.DisplayNameOf(agentId), ErrNotDrivable)
	}
	return nil
}
