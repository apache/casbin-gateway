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
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
)

// llmStreamHeartbeat keeps an idle live feed from being closed by whatever sits
// between the browser and Gateway.
const llmStreamHeartbeat = 25 * time.Second

const llmStatsTopModels = 8

// llmStatsMaxTop bounds what a caller may ask for, so a "top models" table
// cannot be made to return every model name ever relayed.
const llmStatsMaxTop = 100

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

	records, count, err := object.GetLlmRecords(c.readLlmRecordFilter(), (page-1)*limit, limit)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(records, count)
}

// readLlmRecordFilter reads the filter the list and the stats share, so the two
// always describe the same set of requests.
func (c *ApiController) readLlmRecordFilter() object.LlmRecordFilter {
	filter := object.LlmRecordFilter{
		Model:    c.Input().Get("model"),
		Provider: c.Input().Get("provider"),
		Agent:    c.Input().Get("agent"),
		ClientIp: c.Input().Get("clientIp"),
		Outcome:  c.Input().Get("outcome"),
	}
	if hours, err := strconv.Atoi(c.Input().Get("windowHours")); err == nil && hours > 0 {
		filter.Since = util.FormatTime(time.Now().Add(-time.Duration(hours) * time.Hour))
	}
	return filter
}

// GetLlmRecordStats totals the window the list is showing.
func (c *ApiController) GetLlmRecordStats() {
	if c.RequireAdmin() {
		return
	}

	// The dashboard's tables want every model it has seen, where the records
	// page only has room for a row of badges.
	top, _ := strconv.Atoi(c.Input().Get("top"))
	if top < 1 {
		top = llmStatsTopModels
	}
	if top > llmStatsMaxTop {
		top = llmStatsMaxTop
	}

	stats, err := object.GetLlmRecordStats(c.readLlmRecordFilter(), top)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(stats)
}

// GetLlmProviderAudit reports what the kept records say about each provider
// that served them: the same window the stats cover, read for how an upstream
// behaved rather than for what it cost.
func (c *ApiController) GetLlmProviderAudit() {
	if c.RequireAdmin() {
		return
	}

	report, err := object.GetLlmProviderAudit(c.readLlmRecordFilter())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(report)
}

// GetLlmUsageTrend groups the same window into time buckets, which is what the
// usage dashboard draws. "bucket" is "hour" or "day"; anything else is a day.
func (c *ApiController) GetLlmUsageTrend() {
	if c.RequireAdmin() {
		return
	}

	points, err := object.GetLlmUsageTrend(c.readLlmRecordFilter(), c.Input().Get("bucket"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(points)
}

// GetLlmAgentStats totals what each agent has relayed, for the page that lists
// every agent side by side.
func (c *ApiController) GetLlmAgentStats() {
	if c.RequireAdmin() {
		return
	}

	stats, err := object.GetLlmAgentStats(c.readLlmRecordFilter())
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(stats)
}

// StreamLlmRecords pushes records to the page as they are written, so what a
// model is being sent is visible while it happens rather than on the next
// refresh.
func (c *ApiController) StreamLlmRecords() {
	if c.RequireAdmin() {
		return
	}

	c.EnableRender = false
	writer := c.Ctx.ResponseWriter
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	// An nginx in front of Gateway would otherwise hold every event back until
	// the response ends.
	writer.Header().Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	writer.Flush()

	id, feed := object.SubscribeLlmRecords()
	defer object.UnsubscribeLlmRecords(id)

	ticker := time.NewTicker(llmStreamHeartbeat)
	defer ticker.Stop()
	closed := c.Ctx.Request.Context().Done()
	for {
		select {
		case record, ok := <-feed:
			if !ok {
				return
			}
			data, err := json.Marshal(record)
			if err != nil {
				continue
			}
			if _, err := fmt.Fprintf(writer, "event: record\ndata: %s\n\n", data); err != nil {
				return
			}
			writer.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprint(writer, ": keep-alive\n\n"); err != nil {
				return
			}
			writer.Flush()
		case <-closed:
			return
		}
	}
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
	if record == nil {
		c.ResponseError("record not found")
		return
	}

	// The rates travel with the record, so the detail pane does not match model
	// names of its own.
	price, priced := object.GetLlmPrice(record.Model)
	if !priced {
		c.ResponseOk(record)
		return
	}
	c.ResponseOk(record, price)
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
