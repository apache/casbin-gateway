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

	"github.com/apache/casbin-gateway/cloudsync"
	"github.com/apache/casbin-gateway/object"
)

// GetCloudSyncState is where the backups are copied to, how the last run went,
// and what else this Gateway could be pointed at. Admin-only like the rest of
// the backup API: the answer carries the credentials of the target.
func (c *ApiController) GetCloudSyncState() {
	if c.RequireAdmin() {
		return
	}

	c.ResponseOk(object.GetCloudSyncState())
}

// UpdateCloudSync stores where the copies go.
func (c *ApiController) UpdateCloudSync() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Mode    string            `json:"mode"`
		Kind    string            `json:"kind"`
		Options map[string]string `json:"options"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := object.SaveCloudSyncConfig(request.Mode, request.Kind, request.Options); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.GetCloudSyncState()
}

// TestCloudSync reaches the target the form is describing, without storing it.
// Listing is the only check worth making: it proves the credentials, the path
// and the permission to read in one request.
func (c *ApiController) TestCloudSync() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Kind    string            `json:"kind"`
		Options map[string]string `json:"options"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	target, files, err := object.TestCloudSync(request.Kind, request.Options)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(map[string]interface{}{"target": target, "files": files})
}

// RunCloudSync copies now rather than on the schedule. The direction is what
// the buttons offer: everything both ways, only up, or only down.
func (c *ApiController) RunCloudSync() {
	if c.RequireAdmin() {
		return
	}

	var request struct {
		Direction string `json:"direction"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	switch request.Direction {
	case cloudsync.DirectionUp, cloudsync.DirectionDown:
	default:
		request.Direction = cloudsync.DirectionBoth
	}

	if _, err := object.RunCloudSync(request.Direction); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.GetCloudSyncState()
}
