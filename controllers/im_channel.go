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

package controllers

import (
	"encoding/json"

	"github.com/apache/casbin-gateway/imbridge"
	"github.com/apache/casbin-gateway/object"
)

// imChannelView is one stored channel and what its listener is doing.
type imChannelView struct {
	*object.ImChannel
	Status imbridge.Status `json:"status"`
}

// GetImChannels lists the chat platforms Gateway listens on. Every credential is
// masked: a token stored here never goes back to the browser.
func (c *ApiController) GetImChannels() {
	if c.RequireAdmin() {
		return
	}

	channels, err := object.GetImChannels()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	statuses := map[string]imbridge.Status{}
	for _, status := range imbridge.Statuses() {
		statuses[status.Name] = status
	}

	result := []*imChannelView{}
	for _, channel := range channels {
		result = append(result, &imChannelView{ImChannel: channel.Masked(), Status: statuses[channel.Name]})
	}
	c.ResponseOk(result)
}

// UpdateImChannel writes one channel and starts or stops its listener to match.
func (c *ApiController) UpdateImChannel() {
	if c.RequireAdmin() {
		return
	}

	channel := object.ImChannel{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.SaveImChannel(&channel); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(channel.Masked())
}

func (c *ApiController) DeleteImChannel() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.DeleteImChannel(request.Name); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk()
}

// StartWeixinLogin asks WeChat for a code to be scanned by the account that will
// carry the bot.
func (c *ApiController) StartWeixinLogin() {
	if c.RequireAdmin() {
		return
	}

	qrcode, err := imbridge.StartWeixinLogin(c.Ctx.Request.Context())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(qrcode)
}

// GetWeixinLoginStatus reports whether the code has been scanned. The token that
// comes back is written straight into the named channel rather than handed to
// the browser: it is the credential, and it has no business making that trip.
func (c *ApiController) GetWeixinLoginStatus() {
	if c.RequireAdmin() {
		return
	}

	status, err := imbridge.PollWeixinLogin(c.Ctx.Request.Context(), c.Input().Get("qrcode"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	if status.Token != "" {
		channel, err := object.GetImChannel(c.Input().Get("channel"))
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		if channel == nil {
			c.ResponseError("no channel is waiting for this sign-in")
			return
		}
		channel.Token = status.Token
		if err := object.SaveImChannel(channel); err != nil {
			c.ResponseError(err.Error())
			return
		}
		status.Token = ""
	}
	c.ResponseOk(status)
}
