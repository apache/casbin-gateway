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
	"sort"
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

	result, err := object.SyncModelsDevPrices(seenModels(), c.Input().Get("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(result)
}

// seenModels is every model name this machine has a record of running: the
// relayed requests, the agents' own transcripts, and the models the configured
// providers say they serve. A source that cannot be read contributes nothing
// rather than failing the sync, which the other two can still be useful without.
func seenModels() []string {
	found := map[string]bool{}
	add := func(name string) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			found[name] = true
		}
	}

	if models, err := object.GetSeenLlmModels(); err == nil {
		for _, model := range models {
			add(model)
		}
	}

	for _, stat := range object.GetAgentUsage(historicalSessions(""), "").Models {
		add(stat.Name)
	}

	if providers, err := object.GetProviders(""); err == nil {
		for _, provider := range providers {
			for _, model := range provider.Models {
				add(model)
			}
		}
	}

	models := make([]string, 0, len(found))
	for model := range found {
		models = append(models, model)
	}
	sort.Strings(models)
	return models
}
