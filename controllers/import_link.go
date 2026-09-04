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

// ParseImportLink reads a vendor's "add this to Gateway" link and answers with
// what it would fill in, without storing anything: the link comes from a
// website, so the person importing it sees the values first.
func (c *ApiController) ParseImportLink() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	link, err := object.ParseImportLink(c.GetSessionUsername(), form.Link)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(link)
}

// OpenImportLink is how a link that was clicked outside the browser reaches the
// page. The URL scheme handler runs as a program on this machine and hands the
// link over here rather than putting it in the address of the page it opens:
// a provider link carries an API key, and an address is kept in the browser's
// history and sent to whatever the page later navigates to.
func (c *ApiController) OpenImportLink() {
	if c.RequireLocalAdmin() {
		return
	}

	var form struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	raw := strings.TrimSpace(form.Link)
	if raw == "" || len(raw) > object.MaxImportLinkLength {
		c.ResponseError("this is not a link Gateway can import")
		return
	}
	// Read once here so that a link nothing can be made of is reported to
	// whoever clicked it, rather than opening a page that says so.
	if _, err := object.ParseImportLink(c.GetSessionUsername(), raw); err != nil {
		c.ResponseError(err.Error())
		return
	}

	object.SetPendingImportLink(raw)
	c.ResponseOk()
}

// GetPendingImportLink is what the import page asks for on the way in: the link
// the scheme handler left, read and then forgotten, so that reloading the page
// cannot import the same link a second time.
func (c *ApiController) GetPendingImportLink() {
	if c.RequireAdmin() {
		return
	}

	raw := object.TakePendingImportLink()
	if raw == "" {
		c.ResponseOk(nil)
		return
	}

	link, err := object.ParseImportLink(c.GetSessionUsername(), raw)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(link)
}
