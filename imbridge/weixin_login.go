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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/apache/casbin-gateway/proxy"
)

// weixinBotType is the kind of bot the QR code signs in: the personal-account
// pipeline, which is the only one this connects.
const weixinBotType = "3"

// WeixinQrcode is a sign-in waiting to be scanned. WeChat hands back a link
// rather than a picture, so the code drawn on screen is one the page makes out
// of Url; Qrcode is what the status is then asked about.
type WeixinQrcode struct {
	Qrcode string `json:"qrcode"`
	Url    string `json:"url"`
}

// WeixinStatus is where a sign-in has got to. Token is filled in only once the
// account has confirmed, and is what the channel is stored with.
type WeixinStatus struct {
	Status string `json:"status"`
	Token  string `json:"token"`
}

func weixinLoginClient() *http.Client {
	return &http.Client{Transport: proxy.Transport(), Timeout: 70 * time.Second}
}

// StartWeixinLogin asks for a QR code to be scanned by the WeChat account that
// will carry the bot.
func StartWeixinLogin(ctx context.Context) (*WeixinQrcode, error) {
	result := struct {
		Qrcode string `json:"qrcode"`
		// The endpoint calls this "img_content", but what it holds is the link
		// the WeChat scanner opens.
		QrcodeImgContent string `json:"qrcode_img_content"`
		Ret              int    `json:"ret"`
		ErrMsg           string `json:"errmsg"`
	}{}
	query := url.Values{"bot_type": {weixinBotType}}
	if err := weixinGet(ctx, "/ilink/bot/get_bot_qrcode", query, "", &result); err != nil {
		return nil, err
	}

	if result.Ret != 0 {
		return nil, fmt.Errorf("weixin returned %d: %s", result.Ret, result.ErrMsg)
	}
	return &WeixinQrcode{Qrcode: result.Qrcode, Url: result.QrcodeImgContent}, nil
}

// PollWeixinLogin reports whether the code has been scanned yet. It answers
// straight away with "wait" while nothing has happened, so the caller has to
// pace itself rather than calling again the moment this returns.
func PollWeixinLogin(ctx context.Context, qrcode string) (*WeixinStatus, error) {
	result := struct {
		Status   string `json:"status"`
		BotToken string `json:"bot_token"`
		BaseUrl  string `json:"baseurl"`
		Ret      int    `json:"ret"`
		ErrMsg   string `json:"errmsg"`
	}{}
	query := url.Values{"qrcode": {qrcode}}
	if err := weixinGet(ctx, "/ilink/bot/get_qrcode_status", query, "", &result); err != nil {
		return nil, err
	}
	if result.Ret != 0 {
		return nil, fmt.Errorf("weixin returned %d: %s", result.Ret, result.ErrMsg)
	}
	return &WeixinStatus{Status: result.Status, Token: result.BotToken}, nil
}

func weixinGet(ctx context.Context, path string, query url.Values, token string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, weixinApi+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	weixinHeaders(request, token)

	response, err := weixinLoginClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	raw, err := readAll(response.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
