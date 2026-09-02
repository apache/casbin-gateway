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
	"github.com/apache/casbin-gateway/object"
)

// GetProviderProbes is the newest probe of every provider that has one. The
// second payload is how probes are started, so the page can say whether an
// unprobed provider is waiting for a sweep or for a button.
func (c *ApiController) GetProviderProbes() {
	if c.RequireAdmin() {
		return
	}

	probes, err := object.GetProviderProbes()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(probes, object.GetProviderProbeMode())
}

// GetProviderProbeHistory is every kept run for one provider, newest first: two
// runs of the same suite that disagree are what says a backend was swapped.
func (c *ApiController) GetProviderProbeHistory() {
	if c.RequireAdmin() {
		return
	}

	id := c.Input().Get("id")
	owner, _, ok := getProviderOwnerAndName(id)
	if !ok {
		c.ResponseError("invalid provider ID: " + id)
		return
	}
	if !c.providerAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	probes, err := object.GetProviderProbeHistory(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(probes)
}

// ProbeProvider runs the suite against one stored provider and answers with the
// report. It spends the account's own credit, which is why it is a request of
// its own rather than something a page load does.
func (c *ApiController) ProbeProvider() {
	if c.RequireAdmin() {
		return
	}

	id := c.Input().Get("id")
	owner, _, ok := getProviderOwnerAndName(id)
	if !ok {
		c.ResponseError("invalid provider ID: " + id)
		return
	}
	if !c.providerAccess(owner) {
		c.ResponseError("unauthorized")
		return
	}

	provider, err := object.GetProvider(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if provider == nil {
		c.ResponseError("the provider does not exist")
		return
	}

	probe, err := object.ProbeProviderNow(provider, object.ProbeTriggerManual)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(probe)
}
