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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/apache/casbin-gateway/proxy"
)

// PlatformTelegram is the id this platform is stored under.
const PlatformTelegram = "telegram"

const (
	telegramApi = "https://api.telegram.org"
	// telegramPollSeconds is how long the server holds a poll open with nothing
	// to say. It is well inside the client timeout below.
	telegramPollSeconds = 50
	// telegramMessageLimit is the longest message Telegram accepts.
	telegramMessageLimit = 4096
)

// Telegram reaches the Bot API by long polling, which needs no public address:
// Gateway calls out and the answer is held open until something is said.
type Telegram struct {
	channel string
	token   string
	client  *http.Client
	// offset is the update after the last one handled. Telegram keeps an update
	// until it is acknowledged this way, so it survives a restart.
	offset int64
}

func NewTelegram(channel, token string) *Telegram {
	return &Telegram{
		channel: channel,
		token:   token,
		client:  &http.Client{Transport: proxy.Transport(), Timeout: (telegramPollSeconds + 30) * time.Second},
	}
}

func (t *Telegram) Name() string { return PlatformTelegram }

func (t *Telegram) CanEdit() bool { return true }

type telegramUpdate struct {
	UpdateId int64 `json:"update_id"`
	Message  *struct {
		MessageId int64  `json:"message_id"`
		Text      string `json:"text"`
		From      struct {
			Id        int64  `json:"id"`
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			Id int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}

func (t *Telegram) Receive(ctx context.Context, handle func(Message)) error {
	// Whatever piled up while Gateway was down is dropped: answering a question
	// from hours ago, in a directory that has moved on, is worse than silence.
	if err := t.skipBacklog(ctx); err != nil {
		if !isWebhookConflict(err) {
			return err
		}
		// Telegram hands updates to a webhook or to a poll, never to both, and
		// refuses the poll while one is set. The webhook is taken down so this
		// channel can listen; nothing else about the bot is touched.
		if err := t.dropWebhook(ctx); err != nil {
			return err
		}
		if err := t.skipBacklog(ctx); err != nil {
			return err
		}
	}

	for ctx.Err() == nil {
		updates := []telegramUpdate{}
		body := map[string]any{"offset": t.offset, "timeout": telegramPollSeconds}
		if err := t.call(ctx, "getUpdates", body, &updates); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// A refused credential is worth stopping for; anything else is the
			// network, and the next poll is the retry.
			if errors.Is(err, errUnauthorized) {
				return err
			}
			// A webhook set on the bot after this started takes the updates
			// away again, so it is taken down the same way.
			if isWebhookConflict(err) {
				if err := t.dropWebhook(ctx); err != nil {
					return err
				}
				continue
			}
			waitBeforeRetry(ctx)
			continue
		}

		for _, update := range updates {
			t.offset = update.UpdateId + 1
			if update.Message == nil || update.Message.Text == "" {
				continue
			}
			handle(Message{
				Platform: PlatformTelegram,
				Channel:  t.channel,
				ChatId:   strconv.FormatInt(update.Message.Chat.Id, 10),
				UserId:   strconv.FormatInt(update.Message.From.Id, 10),
				UserName: firstNonEmpty(update.Message.From.Username, update.Message.From.FirstName),
				Text:     update.Message.Text,
			})
		}
	}
	return nil
}

// skipBacklog moves the cursor past everything already waiting.
func (t *Telegram) skipBacklog(ctx context.Context) error {
	updates := []telegramUpdate{}
	if err := t.call(ctx, "getUpdates", map[string]any{"offset": -1, "timeout": 0}, &updates); err != nil {
		return err
	}
	for _, update := range updates {
		t.offset = update.UpdateId + 1
	}
	return nil
}

func (t *Telegram) Send(message Message, reply Reply) (string, error) {
	text := reply.Text
	if text == "" {
		return "", nil
	}

	// Telegram refuses anything over its limit outright, so a long answer is cut
	// into messages and only the last one stays editable.
	// The cut is made on the Markdown rather than on the rendered HTML: tags only
	// ever shorten what a reader sees, so a chunk within the limit here is within
	// it after rendering too, and no tag is ever sliced in half.
	chunks := splitForChat(text, telegramMessageLimit)
	last := ""
	for index, chunk := range chunks {
		result := struct {
			MessageId int64 `json:"message_id"`
		}{}

		method := "sendMessage"
		body := map[string]any{
			"chat_id":    message.ChatId,
			"text":       telegramHtml(chunk),
			"parse_mode": "HTML",
			// A link in an answer would otherwise pull in a preview card, redrawn
			// on every edit while the answer is still growing.
			"link_preview_options": map[string]any{"is_disabled": true},
		}
		// Only the first chunk can replace what is already there; the rest are
		// new messages under it.
		if reply.Edit != "" && index == 0 {
			method = "editMessageText"
			body["message_id"] = reply.Edit
		}

		err := t.call(context.Background(), method, body, &result)
		// Nothing an agent writes is worth losing to a markup complaint: the same
		// message goes again as the plain text it was.
		if err != nil && isParseError(err) {
			delete(body, "parse_mode")
			body["text"] = chunk
			err = t.call(context.Background(), method, body, &result)
		}
		if err != nil {
			return last, err
		}
		last = strconv.FormatInt(result.MessageId, 10)
	}
	return last, nil
}

func (t *Telegram) Typing(message Message) {
	body := map[string]any{"chat_id": message.ChatId, "action": "typing"}
	t.call(context.Background(), "sendChatAction", body, nil)
}

var errUnauthorized = errors.New("the bot token was refused")

// isWebhookConflict recognises the one failure that is fixed rather than waited
// out. Telegram reports it in words rather than in a code of its own.
func isWebhookConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "webhook is active")
}

// isParseError recognises Telegram refusing the markup rather than the message.
func isParseError(err error) bool {
	if err == nil {
		return false
	}
	lowered := strings.ToLower(err.Error())
	return strings.Contains(lowered, "parse entities") || strings.Contains(lowered, "tag") && strings.Contains(lowered, "entities")
}

// dropWebhook takes the bot's webhook down. Updates already waiting are kept:
// they are dropped a moment later by the backlog skip, which is the one place
// that decides what is too old to answer.
func (t *Telegram) dropWebhook(ctx context.Context) error {
	return t.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}

// call posts one Bot API method and unpacks its "result" into out.
func (t *Telegram) call(ctx context.Context, method string, body map[string]any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/%s", telegramApi, t.token, method)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := t.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	envelope := struct {
		Ok          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}{}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if !envelope.Ok {
		if response.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("%w: %s", errUnauthorized, envelope.Description)
		}
		return errors.New(envelope.Description)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(envelope.Result, out)
}
