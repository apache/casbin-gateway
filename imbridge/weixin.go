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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apache/casbin-gateway/proxy"
	"github.com/google/uuid"
)

// PlatformWeixin is the id this platform is stored under.
const PlatformWeixin = "weixin"

const (
	// weixinApi is Tencent's own endpoint for the ClawBot pipeline. The protocol
	// is called iLink and it is an official one: a personal WeChat account can
	// carry a bot without any third-party client standing in for it.
	weixinApi = "https://ilinkai.weixin.qq.com"
	// weixinChannelVersion is what the protocol expects to be told about the
	// client. It is sent on every poll.
	weixinChannelVersion = "1.0.2"
	// weixinMessageLimit keeps one reply to a length a chat bubble can show.
	// WeChat has no editing, so a long answer arrives as several messages.
	weixinMessageLimit = 2000
)

const (
	// weixinFromBot is what a message a bot sent is marked with, on the way in
	// and on the way out.
	weixinFromBot = 2
	// weixinFinished is a message that is complete rather than still being
	// dictated.
	weixinFinished = 2
	// weixinTypingOn and weixinTypingOff are the two states of a typing notice.
	weixinTypingOn  = 1
	weixinTypingOff = 2
	// weixinSessionExpired is the protocol saying the sign-in behind the token
	// is gone, which no amount of retrying fixes.
	weixinSessionExpired = -14
)

// Weixin reaches WeChat over iLink, which is long polling: Gateway calls out and
// the answer is held open, so no public address is needed here either.
type Weixin struct {
	channel string
	token   string
	client  *http.Client
	// cursor is the protocol's own "get_updates_buf". It must be handed back on
	// the next poll or the same messages arrive again.
	cursor string
	// tickets are the typing tickets, one per chat, asked for once and reused.
	mu      sync.Mutex
	tickets map[string]string
}

func NewWeixin(channel, token string) *Weixin {
	return &Weixin{
		channel: channel,
		token:   token,
		client:  &http.Client{Transport: proxy.Transport(), Timeout: 70 * time.Second},
		tickets: map[string]string{},
	}
}

func (w *Weixin) Name() string { return PlatformWeixin }

// CanEdit is false: iLink has no way to replace a message already sent, so an
// answer arrives when it is finished rather than growing in place.
func (w *Weixin) CanEdit() bool { return false }

// weixinItem is one piece of a message. Only text is read; a voice message
// arrives with its own transcription, which is text by the time it is here.
type weixinItem struct {
	Type     int `json:"type"`
	TextItem *struct {
		Text string `json:"text"`
	} `json:"text_item,omitempty"`
	VoiceItem *struct {
		Text string `json:"text"`
	} `json:"voice_item,omitempty"`
}

type weixinMessage struct {
	FromUserId   string       `json:"from_user_id"`
	ToUserId     string       `json:"to_user_id"`
	MessageType  int          `json:"message_type"`
	MessageState int          `json:"message_state"`
	ContextToken string       `json:"context_token"`
	ItemList     []weixinItem `json:"item_list"`
}

func (w *Weixin) Receive(ctx context.Context, handle func(Message)) error {
	for ctx.Err() == nil {
		result := struct {
			Ret    int             `json:"ret"`
			Msgs   []weixinMessage `json:"msgs"`
			Cursor string          `json:"get_updates_buf"`
		}{}
		body := map[string]any{"get_updates_buf": w.cursor}
		if err := w.call(ctx, "/ilink/bot/getupdates", body, &result); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, errUnauthorized) {
				return err
			}
			waitBeforeRetry(ctx)
			continue
		}
		// Losing the cursor is what makes the same messages arrive twice, so it
		// is taken before anything is handled.
		w.cursor = result.Cursor

		for _, msg := range result.Msgs {
			// A bot's own messages come back on the same feed, and answering
			// those would be answering itself.
			if msg.MessageType == weixinFromBot || strings.HasSuffix(msg.FromUserId, "@im.bot") {
				continue
			}
			if msg.MessageState != weixinFinished {
				continue
			}
			text := weixinText(msg)
			if text == "" {
				continue
			}
			handle(Message{
				Platform: PlatformWeixin,
				Channel:  w.channel,
				ChatId:   msg.FromUserId,
				UserId:   msg.FromUserId,
				// iLink carries no display name, so the account id is the name.
				UserName:   msg.FromUserId,
				Text:       text,
				ReplyToken: msg.ContextToken,
			})
		}
	}
	return nil
}

func weixinText(msg weixinMessage) string {
	for _, item := range msg.ItemList {
		if item.TextItem != nil && item.TextItem.Text != "" {
			return item.TextItem.Text
		}
		if item.VoiceItem != nil && item.VoiceItem.Text != "" {
			return item.VoiceItem.Text
		}
	}
	return ""
}

func (w *Weixin) Send(message Message, reply Reply) (string, error) {
	if reply.Text == "" {
		return "", nil
	}

	// WeChat shows a message exactly as it is written, so the markup comes off
	// rather than being read out as asterisks and backticks.
	for _, chunk := range splitForChat(plainText(reply.Text), weixinMessageLimit) {
		body := map[string]any{
			"msg": map[string]any{
				"from_user_id": "",
				"to_user_id":   message.ChatId,
				// client_id names this message and nothing else. Without one the
				// protocol takes a second reply for a repeat of the first, answers
				// 200, and shows nothing.
				"client_id":     "casbin-gateway-" + uuid.NewString(),
				"message_type":  weixinFromBot,
				"message_state": weixinFinished,
				// The reply carries the token of the message it answers. Without
				// it the protocol has nowhere to put the reply.
				"context_token": message.ReplyToken,
				"item_list":     []any{map[string]any{"type": 1, "text_item": map[string]any{"text": chunk}}},
			},
		}
		if err := w.call(context.Background(), "/ilink/bot/sendmessage", body, nil); err != nil {
			return "", err
		}
	}

	w.stopTyping(message)
	return "", nil
}

func (w *Weixin) Typing(message Message) {
	w.sendTyping(message, w.ticket(message), weixinTypingOn)
}

// stopTyping takes the notice down once the answer is out. It asks for no ticket
// of its own: with none already in hand there was no notice to take down.
func (w *Weixin) stopTyping(message Message) {
	w.mu.Lock()
	ticket := w.tickets[message.ChatId]
	w.mu.Unlock()
	w.sendTyping(message, ticket, weixinTypingOff)
}

func (w *Weixin) sendTyping(message Message, ticket string, status int) {
	if ticket == "" {
		return
	}
	body := map[string]any{
		"ilink_user_id": message.ChatId,
		"typing_ticket": ticket,
		"status":        status,
	}
	w.call(context.Background(), "/ilink/bot/sendtyping", body, nil)
}

// ticket is what a typing notice has to carry. It is asked for once per chat,
// since the request for it needs a message to name and the answer outlives one.
func (w *Weixin) ticket(message Message) string {
	w.mu.Lock()
	ticket := w.tickets[message.ChatId]
	w.mu.Unlock()
	if ticket != "" {
		return ticket
	}

	config := struct {
		TypingTicket string `json:"typing_ticket"`
	}{}
	body := map[string]any{"ilink_user_id": message.ChatId, "context_token": message.ReplyToken}
	if err := w.call(context.Background(), "/ilink/bot/getconfig", body, &config); err != nil || config.TypingTicket == "" {
		return ""
	}

	w.mu.Lock()
	w.tickets[message.ChatId] = config.TypingTicket
	w.mu.Unlock()
	return config.TypingTicket
}

// call posts one iLink method. A non-zero "ret" is the protocol's own way of
// reporting a failure, and comes back with 200.
func (w *Weixin) call(ctx context.Context, path string, body map[string]any, out any) error {
	if body == nil {
		body = map[string]any{}
	}
	// Every iLink request is told what client it came from, not only the poll.
	body["base_info"] = map[string]any{"channel_version": weixinChannelVersion}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, weixinApi+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	weixinHeaders(request, w.token)

	response, err := w.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w: HTTP %d", errUnauthorized, response.StatusCode)
	}

	raw, err := readAll(response.Body)
	if err != nil {
		return err
	}
	envelope := struct {
		Ret     int    `json:"ret"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
	}
	if envelope.ErrCode == weixinSessionExpired {
		return fmt.Errorf("%w: the WeChat sign-in behind it has expired, so the code has to be scanned again", errUnauthorized)
	}
	if envelope.Ret != 0 {
		return fmt.Errorf("weixin returned %d: %s", envelope.Ret, envelope.ErrMsg)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// weixinHeaders are what every iLink request carries. X-WECHAT-UIN is a fresh
// random number each time, which is the protocol's guard against a request being
// replayed.
func weixinHeaders(request *http.Request, token string) {
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("AuthorizationType", "ilink_bot_token")
	request.Header.Set("X-WECHAT-UIN", base64.StdEncoding.EncodeToString([]byte(strconv.FormatUint(uint64(rand.Uint32()), 10))))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
}
