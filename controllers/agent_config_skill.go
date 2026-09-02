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
	"io"

	"github.com/apache/casbin-gateway/agentconfig"
)

// GetSkillSources lists the places skills can be installed from: the built-in
// repositories and whatever the operator has added.
func (c *ApiController) GetSkillSources() {
	if c.RequireAdmin() {
		return
	}

	sources, err := agentconfig.ListSources(c.GetString("owner"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(sources)
}

// AddSkillSource records one new place to install from: a GitHub repository, a
// .zip or .tar.gz at a URL, or a folder on this machine.
func (c *ApiController) AddSkillSource() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Owner  string `json:"owner"`
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Url    string `json:"url"`
		Ref    string `json:"ref"`
		Subdir string `json:"subdir"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	source, err := agentconfig.AddSource(form.Owner, &agentconfig.SkillSource{
		Name:   form.Name,
		Kind:   form.Kind,
		Url:    form.Url,
		Ref:    form.Ref,
		Subdir: form.Subdir,
	})
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(source)
}

// UploadSkillSource takes an archive of skills the operator picked in the
// browser and adds it as a source, so a skill that is nowhere public can be
// installed the same way as one that is.
func (c *ApiController) UploadSkillSource() {
	if c.RequireAdmin() {
		return
	}

	file, header, err := c.GetFile("file")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	defer file.Close()

	// One byte past the limit tells an archive that is exactly the limit from
	// one that was cut short here.
	data, err := io.ReadAll(io.LimitReader(file, agentconfig.MaxUploadBytes+1))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if len(data) > agentconfig.MaxUploadBytes {
		c.ResponseError(fmt.Sprintf("this archive is larger than %d MB", agentconfig.MaxUploadBytes>>20))
		return
	}

	name := c.GetString("name")
	if name == "" && header != nil {
		name = header.Filename
	}

	source, err := agentconfig.UploadSource(c.GetString("owner"), name, data)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(source)
}

// DeleteSkillSource takes one source off the list and drops Gateway's copy of
// it. Skills already installed from it stay where they are.
func (c *ApiController) DeleteSkillSource() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		Owner string `json:"owner"`
		Id    string `json:"id"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	if err := agentconfig.DeleteSource(form.Owner, form.Id); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(form.Id)
}

// GetSkillCatalog lists the skills one source holds, downloading it first when
// Gateway has no copy of it or when refresh asks for it again.
func (c *ApiController) GetSkillCatalog() {
	if c.RequireAdmin() {
		return
	}

	catalog, err := agentconfig.ReadCatalog(
		c.GetString("owner"), c.GetString("id"), c.GetString("refresh") == "true")
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(catalog)
}

// InstallSkills writes the chosen skills of one source into the agents it
// names, as a copy each agent then owns or as a link that follows the source.
func (c *ApiController) InstallSkills() {
	if c.RequireAdmin() {
		return
	}

	var request agentconfig.InstallRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &request); err != nil {
		c.ResponseError(err.Error())
		return
	}

	result, err := agentconfig.InstallSkills(request)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(result)
}
