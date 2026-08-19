// Copyright 2023 The casbin Authors. All Rights Reserved.
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
	"strings"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego/utils/pagination"
)

func (c *ApiController) channelAccess(owner string) bool {
	user := c.GetSessionUser()
	return user != nil && (user.IsAdmin || owner == c.GetSessionUsername())
}

// getChannelOwnerAndName splits an "owner/name" id. util.GetOwnerAndNameFromId()
// panics on malformed input, and the id here comes straight from the query
// string, so it is validated before being passed on.
func getChannelOwnerAndName(id string) (string, string, bool) {
	tokens := strings.Split(id, "/")
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return "", "", false
	}

	return tokens[0], tokens[1], true
}

// channelSortFields is a whitelist: the sort field is interpolated into the
// ORDER BY clause by object.GetSession(), so it must not be taken from the
// query string unchecked.
var channelSortFields = map[string]bool{
	"owner": true, "name": true, "createdTime": true, "updatedTime": true,
	"displayName": true, "type": true, "baseUrl": true, "priority": true, "status": true,
}

func getChannelSortField(sortField string) string {
	if channelSortFields[sortField] {
		return sortField
	}

	return ""
}

// GetChannels returns a paginated list of channels with tenant isolation.
func (c *ApiController) GetChannels() {
	if c.RequireSignedIn() {
		return
	}

	owner := c.Input().Get("owner")
	if !c.GetSessionUser().IsAdmin {
		// A non-admin may only ever see their own channels.
		owner = c.GetSessionUsername()
	} else if owner == "admin" {
		// The admin's own owner name means "no filter", i.e. every channel.
		owner = ""
	}

	limit := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := getChannelSortField(c.Input().Get("sortField"))
	sortOrder := c.Input().Get("sortOrder")

	if limit == "" || page == "" {
		channels, err := object.GetChannels(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		c.ResponseOk(object.GetMaskedChannels(channels))
		return
	}

	limitInt := util.ParseInt(limit)
	count, err := object.GetChannelCount(owner, field, value)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	paginator := pagination.SetPaginator(c.Ctx, limitInt, count)
	channels, err := object.GetPaginationChannels(owner, paginator.Offset(), limitInt, field, value, sortField, sortOrder)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedChannels(channels), paginator.Nums())
}

// GetChannel returns a single channel by owner/name id.
func (c *ApiController) GetChannel() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _, ok := getChannelOwnerAndName(id)
	if !ok {
		c.ResponseError("invalid channel ID: " + id)
		return
	}
	if !c.channelAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	channel, err := object.GetChannel(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if channel == nil {
		c.ResponseError("the channel does not exist")
		return
	}

	c.ResponseOk(object.GetMaskedChannel(channel))
}

// AddChannel creates a new channel.
func (c *ApiController) AddChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if channel.Name == "" {
		c.ResponseError("the channel name cannot be empty")
		return
	}

	channel.Owner = c.GetSessionUsername()
	c.Data["json"] = wrapActionResponse(object.AddChannel(&channel))
	c.ServeJSON()
}

// UpdateChannel updates an existing channel.
func (c *ApiController) UpdateChannel() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	owner, _, ok := getChannelOwnerAndName(id)
	if !ok {
		c.ResponseError("invalid channel ID: " + id)
		return
	}
	if !c.channelAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateChannel(id, &channel))
	c.ServeJSON()
}

// DeleteChannel deletes a channel.
func (c *ApiController) DeleteChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if !c.channelAccess(channel.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteChannel(&channel))
	c.ServeJSON()
}

// TestChannel tests connectivity to an upstream channel.
func (c *ApiController) TestChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if channel.Owner == "" || channel.Name == "" {
		c.ResponseError("the channel owner and name cannot be empty")
		return
	}
	if !c.channelAccess(channel.Owner) {
		c.ResponseError("unauthorized")
		return
	}

	success, statusCode, message := object.TestChannelConnectivity(&channel)
	c.ResponseOk(map[string]interface{}{"success": success, "statusCode": statusCode, "message": message})
}
