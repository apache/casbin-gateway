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
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/apache/casbin-gateway/agentprovider"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
)

var nativePreviewKey = []byte(rand.Text())

// Preview and confirmation re-read the selected installation. The browser
// carries a revision of the preview, never the credential itself.
func (c *ApiController) ImportNativeProvider() {
	if c.RequireAdmin() {
		return
	}
	if !util.IsLoopbackRequest(c.Ctx.Request) {
		c.ResponseError("open Gateway on this machine to import native credentials")
		return
	}
	var form struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError("invalid import request")
		return
	}
	target, ok := c.readAgentPatchTarget()
	if !ok {
		return
	}
	native, err := agentprovider.ReadNativeProvider(providerTarget(target))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	mac := hmac.New(sha256.New, nativePreviewKey)
	_ = json.NewEncoder(mac).Encode([]any{c.GetSessionUsername(), target, native, native.ApiKey})
	revision := hex.EncodeToString(mac.Sum(nil))
	if form.Revision == "" {
		c.ResponseOk(map[string]interface{}{"provider": native, "revision": revision})
		return
	}
	if !hmac.Equal([]byte(form.Revision), []byte(revision)) {
		c.ResponseError("the native configuration changed; preview it again before saving")
		return
	}
	id, err := object.ImportNativeProvider(c.GetSessionUsername(), target.AgentId, native)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(id)
}
