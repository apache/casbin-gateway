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

// SignInProvider starts a browser sign-in for a vendor whose subscription
// Gateway can hold, and answers with the address to approve it at. The token it
// brings back stays here: the page carries only the id of the sign-in, which
// the save of the provider redeems.
func (c *ApiController) SignInProvider() {
	if c.RequireAdmin() {
		return
	}

	session, err := object.StartSubscriptionLogin(c.Input().Get("vendor"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(session)
}

// GetProviderSignin reports how a sign-in that was started is getting on. The
// page polls it: approving one takes as long as whoever is at the browser.
func (c *ApiController) GetProviderSignin() {
	if c.RequireAdmin() {
		return
	}

	session, ok := object.SubscriptionLoginSession(c.Input().Get("id"))
	if !ok {
		c.ResponseError("no sign-in was started under this id")
		return
	}
	c.ResponseOk(session)
}
