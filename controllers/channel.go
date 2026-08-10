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

	"github.com/beego/beego/utils/pagination"
	"github.com/casbin/caswaf/object"
	"github.com/casbin/caswaf/util"
)

func (c *ApiController) GetGlobalChannels() {
	if c.RequireSignedIn() {
		return
	}

	channels, err := object.GetGlobalChannels()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedChannels(channels))
}

func (c *ApiController) GetChannels() {
	if c.RequireSignedIn() {
		return
	}

	owner := c.Input().Get("owner")
	if owner == "admin" {
		owner = ""
	}

	limit := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := c.Input().Get("sortField")
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

func (c *ApiController) GetChannel() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")

	channel, err := object.GetChannel(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedChannel(channel))
}

func (c *ApiController) UpdateChannel() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")

	var channel object.Channel
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateChannel(id, &channel))
	c.ServeJSON()
}

func (c *ApiController) AddChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.AddChannel(&channel))
	c.ServeJSON()
}

func (c *ApiController) DeleteChannel() {
	if c.RequireSignedIn() {
		return
	}

	var channel object.Channel
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &channel)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteChannel(&channel))
	c.ServeJSON()
}
