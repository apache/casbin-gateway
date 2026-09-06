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
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/agentsession"
)

// Channel is one chat platform Gateway is listening on, and what a conversation
// there is bound to. It is filled in from storage, which lives outside this
// package so that carrying a chat does not depend on the database.
type Channel struct {
	Name     string
	Platform string
	Token    string

	AgentId   string
	AgentPath string
	AgentUser string
	WorkDir   string
	Model     string

	// AllowedUsers are the platform ids that may drive the agent. An empty list
	// lets anybody who finds the bot drive it, which is only right while the bot
	// is not published anywhere.
	AllowedUsers []string
}

func (channel Channel) allows(userId string) bool {
	return len(channel.AllowedUsers) == 0 || slices.Contains(channel.AllowedUsers, userId)
}

// sourceOf names one chat in the way a session records who opened it. It is also
// how a session is found again after a restart, so no second table is needed to
// remember which conversation belongs to which chat.
func sourceOf(message Message) string {
	return fmt.Sprintf("im:%s:%s:%s", message.Platform, message.Channel, message.ChatId)
}

const (
	// editInterval is how often a growing answer is rewritten on a platform that
	// can edit. Often enough to look live, rarely enough not to be rate limited.
	editInterval = 2 * time.Second
	// typingInterval is how often a platform is reminded that the agent is still
	// working, for one that cannot show a partial answer.
	typingInterval = 4 * time.Second
)

// router carries the conversations of one channel.
type router struct {
	channel  Channel
	platform Platform

	// busy keeps one chat from starting a second turn while the first is still
	// running, which every one of these agents would refuse anyway.
	mu   sync.Mutex
	busy map[string]bool
}

func newRouter(channel Channel, platform Platform) *router {
	return &router{channel: channel, platform: platform, busy: map[string]bool{}}
}

func (r *router) handle(message Message) {
	if !r.channel.allows(message.UserId) {
		return
	}

	text := strings.TrimSpace(message.Text)
	if strings.HasPrefix(text, "/") {
		r.reply(message, r.command(message, text))
		return
	}
	if text == "" {
		return
	}

	r.mu.Lock()
	if r.busy[message.ChatId] {
		r.mu.Unlock()
		r.reply(message, "The agent is still working on the last one. /stop ends it.")
		return
	}
	r.busy[message.ChatId] = true
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			delete(r.busy, message.ChatId)
			r.mu.Unlock()
		}()
		r.ask(message, text)
	}()
}

// ask hands one message to the agent and writes the answer back as it arrives.
func (r *router) ask(message Message, prompt string) {
	session, err := r.session(message)
	if err != nil {
		r.reply(message, "This chat has no agent to talk to: "+err.Error())
		return
	}

	// Everything already on the feed belongs to turns that were written out
	// long ago, so this one starts from now rather than replaying them.
	subscriberId, feed, _, err := agentsession.Subscribe(session.Id, math.MaxInt64)
	if err != nil {
		r.reply(message, err.Error())
		return
	}
	defer agentsession.Unsubscribe(session.Id, subscriberId)

	if _, err := agentsession.Send(session.Id, prompt); err != nil {
		r.reply(message, err.Error())
		return
	}
	r.platform.Typing(message)

	answer := &answerBuffer{}
	posted := ""
	ticker := time.NewTicker(editInterval)
	defer ticker.Stop()
	typing := time.NewTicker(typingInterval)
	defer typing.Stop()

	for {
		select {
		case event, ok := <-feed:
			if !ok {
				return
			}
			answer.add(event)
			if event.Type == agentsession.EventDone {
				if final := answer.text(); final != "" {
					posted, _ = r.platform.Send(message, Reply{Text: final, Edit: posted})
				}
				return
			}
		case <-ticker.C:
			// A platform that cannot edit says nothing until the end: sending
			// every few seconds would be a wall of half-written answers.
			if !r.platform.CanEdit() || !answer.changed() {
				continue
			}
			if partial := answer.text(); partial != "" {
				posted, _ = r.platform.Send(message, Reply{Text: partial, Edit: posted})
			}
		case <-typing.C:
			if !r.platform.CanEdit() {
				r.platform.Typing(message)
			}
		}
	}
}

// session is the conversation this chat is bound to, opening one the first time.
func (r *router) session(message Message) (agentsession.Session, error) {
	source := sourceOf(message)
	for _, candidate := range agentsession.List() {
		if candidate.Source == source {
			return candidate, nil
		}
	}

	if r.channel.AgentId == "" {
		return agentsession.Session{}, fmt.Errorf("no agent is bound to the %s channel", r.channel.Name)
	}
	return agentsession.Open(agentsession.Spec{
		AgentId:   r.channel.AgentId,
		AgentPath: r.channel.AgentPath,
		Owner:     r.channel.AgentUser,
		WorkDir:   r.channel.WorkDir,
		Model:     r.channel.Model,
		Source:    source,
	})
}

func (r *router) reply(message Message, text string) {
	if text == "" {
		return
	}
	r.platform.Send(message, Reply{Text: text})
}
