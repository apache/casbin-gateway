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
	"time"

	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/casdoor"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
)

func (c *ApiController) GetSetting() {
	if c.RequireAdmin() {
		return
	}

	setting, err := object.GetBuiltInSetting()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(setting)
}

// UpdateSetting stores the settings and applies them to the running Gateway, so
// the Settings page is all there is to it: no file to edit, and nothing to
// restart afterwards.
func (c *ApiController) UpdateSetting() {
	if c.RequireAdmin() {
		return
	}

	var setting object.Setting
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &setting); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := validateSetting(&setting); err != nil {
		c.ResponseError(err.Error())
		return
	}

	previous, err := object.GetBuiltInSetting()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if previous == nil {
		c.ResponseError("the built-in setting does not exist")
		return
	}

	affected, err := object.UpdateSetting(object.BuiltInSettingId, &setting)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !affected {
		c.ResponseError("the built-in setting could not be updated")
		return
	}

	// Everything is saved by now, so a subsystem that refuses the new setting is
	// reported without losing it: the admin can correct the value, or start
	// Gateway again with the privileges it turned out to need.
	if err = applySetting(previous, &setting); err != nil {
		c.ResponseError(fmt.Sprintf("the settings were saved, but they could not all be applied: %s", err.Error()))
		return
	}

	c.ResponseOk(setting)
}

func validateSetting(setting *object.Setting) error {
	switch setting.LlmRecordMode {
	case conf.LlmRecordOff, conf.LlmRecordMetadata, conf.LlmRecordFull:
	default:
		return fmt.Errorf("llmRecordMode must be one of \"off\", \"metadata\" or \"full\"")
	}

	return nil
}

// applySetting hands the stored settings to the subsystems that hold on to
// them. conf already answers with the new values, so each call below is the
// same one main() makes at startup.
func applySetting(previous *object.Setting, setting *object.Setting) error {
	casdoor.InitCasdoorConfig()
	proxy.InitHttpClient()
	object.ReloadLlmPrices()
	agentmonitor.Configure(
		conf.GetAgentPatchStateDir(),
		time.Duration(conf.GetAgentMonitorPollSeconds())*time.Second,
	)

	if setting.LlmRecordMode == conf.LlmRecordOff {
		object.StopLlmRecordWriter()
	} else {
		// The queue is sized when the writer starts, so a new capacity only
		// takes hold once it has been through a stop.
		if previous.LlmRecordQueueCapacity != setting.LlmRecordQueueCapacity {
			object.StopLlmRecordWriter()
		}
		object.StartLlmRecordWriter()
	}

	return nil
}
