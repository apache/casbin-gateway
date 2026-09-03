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
	"strings"

	"github.com/apache/casbin-gateway/object"
)

// Prices decide what every recorded request is said to have cost, so like the
// records themselves they are the administrator's to read and to change.

// GetLlmPrices lists every price in effect, built-in ones included, each with
// the layer it came from.
func (c *ApiController) GetLlmPrices() {
	if c.RequireAdmin() {
		return
	}

	c.ResponseOk(object.GetLlmPriceViews())
}

// UpdateLlmPrice stores one price, which from then on overrides whatever the
// built-in table or the pricing file says for that model.
func (c *ApiController) UpdateLlmPrice() {
	if c.RequireAdmin() {
		return
	}

	entry := object.LlmPriceEntry{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &entry); err != nil {
		c.ResponseError(err.Error())
		return
	}
	// A price arriving from the page is somebody's own correction, whatever the
	// row it was opened from was written by.
	entry.Source = object.LlmPriceSourceManual

	if err := object.SetLlmPriceEntry(&entry); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(entry)
}

// DeleteLlmPrice drops a stored price, putting the built-in one for that model
// back in effect where there is one.
func (c *ApiController) DeleteLlmPrice() {
	if c.RequireAdmin() {
		return
	}

	model := c.Input().Get("model")
	if strings.TrimSpace(model) == "" {
		c.ResponseError("a price needs a model name")
		return
	}
	if err := object.DeleteLlmPriceEntry(model); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(true)
}

// SearchModelsDevModels answers the dialog that adds a price for a model this
// machine has not run yet, so the rates are copied from the catalogue rather
// than typed in from a vendor's pricing page.
func (c *ApiController) SearchModelsDevModels() {
	if c.RequireAdmin() {
		return
	}

	models, err := object.SearchModelsDev(c.Input().Get("q"), c.Input().Get("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(models)
}

// SyncModelsDevPrices prices every model this machine has been seen running,
// from the models.dev catalogue. It answers with what it did, because the half
// that matters is the models the catalogue could not price: those are the ones
// still costing nothing on the Usage page.
func (c *ApiController) SyncModelsDevPrices() {
	if c.RequireAdmin() {
		return
	}

	result, err := object.RunModelsDevSync(c.Input().Get("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(result)
}

// GetModelsDevSync reports the schedule an automatic sync is on and what the
// last run did, so the Pricing page can say when prices were last read and
// which models the catalogue still cannot price.
func (c *ApiController) GetModelsDevSync() {
	if c.RequireAdmin() {
		return
	}

	c.ResponseOk(object.GetModelsDevSyncState())
}

// UpdateModelsDevSync turns the automatic sync on or off and sets how often it
// runs. It writes the same two settings the Settings page would, so the choice
// survives a restart.
func (c *ApiController) UpdateModelsDevSync() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Mode          string `json:"mode"`
		IntervalHours int    `json:"intervalHours"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if request.Mode != object.ModelsDevSyncAuto && request.Mode != object.ModelsDevSyncOff {
		c.ResponseError("modelsDevSyncMode must be \"auto\" or \"off\"")
		return
	}
	if request.IntervalHours <= 0 {
		c.ResponseError("the sync interval must be at least one hour")
		return
	}

	setting, err := object.GetBuiltInSetting()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if setting == nil {
		c.ResponseError("the built-in setting does not exist")
		return
	}

	setting.ModelsDevSyncMode = request.Mode
	setting.ModelsDevSyncIntervalHours = request.IntervalHours
	if _, err := object.UpdateSetting(object.BuiltInSettingId, setting); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetModelsDevSyncState())
}
