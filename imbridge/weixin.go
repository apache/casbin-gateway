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
	"time"

	"github.com/apache/casbin-gateway/proxy"
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

// Weixin reaches WeChat over iLink, which is long polling: Gateway calls out and
// the answer is held open, so no public address is needed here either.
type Weixin struct {
	channel string
	token   string
	client  *http.Client
	// cursor is the protocol's own "get_updates_buf". It must be handed back on
	// the next poll or the same messages arrive again.
	cursor string
	// typingTicket is asked for once and reused, since it is what a typing
	// notice has to carry.
	typingTicket string
}

func NewWeixin(channel, token string) *Weixin {
	return &Weixin{
		channel: channel,
		token:   token,
		client:  &http.Client{Transport: proxy.Transport(), Timeout: 70 * time.Second},
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
		body := map[string]any{
			"get_updates_buf": w.cursor,
			"base_info":       map[string]any{"channel_version": weixinChannelVersion},
		}
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
			// message_type 1 is somebody talking; message_state 2 is a message
			// that is finished rather than still being dictated.
			if msg.MessageType != 1 || msg.MessageState != 2 {
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

	for _, chunk := range splitForChat(reply.Text, weixinMessageLimit) {
		body := map[string]any{
			"msg": map[string]any{
				"to_user_id":    message.ChatId,
				"message_type":  2,
				"message_state": 2,
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
	return "", nil
}

func (w *Weixin) Typing(message Message) {
	if w.typingTicket == "" {
		config := struct {
			TypingTicket string `json:"typing_ticket"`
		}{}
		if err := w.call(context.Background(), "/ilink/bot/getconfig", map[string]any{}, &config); err != nil {
			return
		}
		w.typingTicket = config.TypingTicket
	}

	body := map[string]any{
		"to_user_id":    message.ChatId,
		"context_token": message.ReplyToken,
		"typing_ticket": w.typingTicket,
	}
	w.call(context.Background(), "/ilink/bot/sendtyping", body, nil)
}

// call posts one iLink method. A non-zero "ret" is the protocol's own way of
// reporting a failure, and comes back with 200.
func (w *Weixin) call(ctx context.Context, path string, body map[string]any, out any) error {
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
		Ret    int    `json:"ret"`
		ErrMsg string `json:"errmsg"`
	}{}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return err
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
