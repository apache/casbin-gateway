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

// Package imbridge carries a conversation between a chat platform and an agent
// on this machine. The platforms differ only in how a message arrives and how a
// reply goes back; everything between - which session a chat is bound to, what a
// slash command means, how an answer is written out - is the same for all of
// them and lives here.
package imbridge

import "context"

// Message is one thing somebody said to the bot.
type Message struct {
	Platform string
	// Channel is the stored channel this arrived on, which is what binds the
	// conversation to an agent and a working directory.
	Channel string
	// ChatId is where a reply goes.
	ChatId   string
	UserId   string
	UserName string
	Text     string
	// ReplyToken is what a platform demands back on the reply for it to land in
	// the right conversation. WeChat's context_token is one; Telegram needs none.
	ReplyToken string
}

// Reply is one thing the bot says. An empty Edit posts a new message; otherwise
// it replaces the one already posted under that id.
type Reply struct {
	Text string
	Edit string
}

// Platform is one chat service. An implementation owns its own connection and
// its own idea of an id, and nothing else about it reaches the rest of Gateway.
type Platform interface {
	Name() string
	// Receive delivers messages to handle until ctx is done. It is expected to
	// keep going through the ordinary network failures, and to return only when
	// the channel is stopped or the credential is refused.
	Receive(ctx context.Context, handle func(Message)) error
	// Send posts a reply and reports the id of the message it created, empty on
	// a platform that has no way to name one.
	Send(message Message, reply Reply) (string, error)
	// Typing shows that the agent is working, where the platform has a way to.
	Typing(message Message)
	// CanEdit reports whether Send can replace a message it already posted,
	// which is what decides between an answer that grows in place and one that
	// arrives when it is finished.
	CanEdit() bool
}
