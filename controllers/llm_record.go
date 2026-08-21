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
	"strconv"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
)

// llmUsageTopN caps how many models/channels/agents each usage breakdown lists.
const llmUsageTopN = 8

// A record holds what a user asked a model, so unlike the proxy endpoints
// themselves these are limited to Gateway administrators.

// GetLlmRecords lists one page of relayed requests, without their bodies.
func (c *ApiController) GetLlmRecords() {
	if c.RequireAdmin() {
		return
	}

	page, _ := strconv.Atoi(c.Input().Get("p"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(c.Input().Get("pageSize"))
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	filter := object.LlmRecordFilter{
		Model:    c.Input().Get("model"),
		Channel:  c.Input().Get("channel"),
		Agent:    c.Input().Get("agent"),
		ClientIp: c.Input().Get("clientIp"),
		Outcome:  c.Input().Get("outcome"),
	}
	records, count, err := object.GetLlmRecords(filter, (page-1)*limit, limit)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(records, count)
}

// GetLlmRecord returns one record with its stored request body.
func (c *ApiController) GetLlmRecord() {
	if c.RequireAdmin() {
		return
	}

	id, err := strconv.ParseInt(c.Input().Get("id"), 10, 64)
	if err != nil {
		c.ResponseError("invalid record id")
		return
	}
	record, err := object.GetLlmRecord(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(record)
}

// GetLlmUsage aggregates token usage over a time window: headline totals, a
// per-bucket series to chart over time, and the top models, channels and agents
// by tokens. It honours the same filters as the record list.
func (c *ApiController) GetLlmUsage() {
	if c.RequireAdmin() {
		return
	}

	rangeType := c.Input().Get("rangeType")
	count := util.ParseInt(c.Input().Get("count"))
	if count <= 0 {
		count = 1
	}
	startTime := time.Now().Add(time.Duration(-count) * rangeType2Duration(rangeType))
	timeType := granularity2TimeType(c.Input().Get("granularity"))

	filter := object.LlmRecordFilter{
		Model:   c.Input().Get("model"),
		Channel: c.Input().Get("channel"),
		Agent:   c.Input().Get("agent"),
		Outcome: c.Input().Get("outcome"),
	}

	totals, err := object.GetLlmUsageTotals(filter, startTime)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	overTime, err := object.GetLlmTokensOverTime(filter, startTime, timeType)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	byModel, err := object.GetLlmTokensByDimension(filter, startTime, "model", llmUsageTopN)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	byChannel, err := object.GetLlmTokensByDimension(filter, startTime, "channel", llmUsageTopN)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	byAgent, err := object.GetLlmTokensByDimension(filter, startTime, "agent", llmUsageTopN)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(map[string]interface{}{
		"totals":    totals,
		"overTime":  overTime,
		"byModel":   byModel,
		"byChannel": byChannel,
		"byAgent":   byAgent,
	})
}

// GetLlmRecordStatus reports the recorder's own settings and how many records
// it had to drop, so a gap in the list can be explained.
func (c *ApiController) GetLlmRecordStatus() {
	if c.RequireAdmin() {
		return
	}

	status, err := object.GetLlmRecordStatus()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(status)
}

func (c *ApiController) DeleteLlmRecord() {
	if c.RequireAdmin() {
		return
	}

	id, err := strconv.ParseInt(c.Input().Get("id"), 10, 64)
	if err != nil {
		c.ResponseError("invalid record id")
		return
	}
	if err := object.DeleteLlmRecord(id); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}

// ClearLlmRecords drops every record, for the operator who has to answer a
// request to erase retained prompts.
func (c *ApiController) ClearLlmRecords() {
	if c.RequireAdmin() {
		return
	}

	if err := object.ClearLlmRecords(); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}
