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

	"github.com/apache/casbin-gateway/autostart"
)

// AutostartState is the login entry as the Settings page shows it. It is host
// state rather than a stored setting: the machine holds it, so a restored
// backup does not bring another machine's answer with it.
type AutostartState struct {
	Supported bool   `json:"supported"`
	Enabled   bool   `json:"enabled"`
	Launcher  string `json:"launcher"`
}

func getAutostartState() (*AutostartState, error) {
	on, err := autostart.Enabled()
	if err != nil {
		return nil, err
	}

	return &AutostartState{
		Supported: autostart.Supported(),
		Enabled:   on,
		Launcher:  autostart.LauncherPath(),
	}, nil
}

func (c *ApiController) GetAutostart() {
	if c.RequireAdmin() {
		return
	}

	state, err := getAutostartState()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(state)
}

func (c *ApiController) UpdateAutostart() {
	if c.RequireAdmin() {
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &body); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := autostart.Set(body.Enabled); err != nil {
		c.ResponseError(err.Error())
		return
	}

	state, err := getAutostartState()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(state)
}
