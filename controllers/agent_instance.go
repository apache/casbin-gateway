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
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentlink"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/agentprocess"
	"github.com/apache/casbin-gateway/object"
	"github.com/beego/beego"
)

// maxInstances caps the names Gateway will hand out on its own, so a page that
// keeps asking cannot fill the disk with empty profiles.
const maxInstances = 50

// linkCaptureTtl is how long a copy waiting to be signed in holds the URL scheme
// of its agent: long enough for a sign-in in a browser, short enough that a link
// arriving later still opens whichever copy the agent registered itself for.
const linkCaptureTtl = 15 * time.Minute

// agentInstanceView is one instance as the pages show it: what was stored, the
// account its own state is signed in to, and whether it is running right now.
type agentInstanceView struct {
	*object.AgentInstance
	Account *agent.Account `json:"account,omitempty"`
	// Desktop tells the UI what a start would open: the app itself, or a console
	// window for a CLI.
	Desktop bool `json:"desktop"`
	// CanCapture is whether Gateway can route this agent's own links here at
	// all, and Capturing whether the next one will open this copy.
	CanCapture bool `json:"canCapture"`
	Capturing  bool `json:"capturing"`
	agentprocess.Status
}

// GetAgentInstances lists the extra copies of one agent, or of every agent when
// no agent is named. Each is signed in on its own, so each reports its own
// account and its own run state.
func (c *ApiController) GetAgentInstances() {
	if c.RequireAdmin() {
		return
	}

	instances, err := object.GetAgentInstances(c.Input().Get("agent"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if c.GetString("refresh") == "true" {
		agentprocess.Refresh()
	}

	result := make([]*agentInstanceView, 0, len(instances))
	for _, instance := range instances {
		result = append(result, instanceView(instance))
	}
	c.ResponseOk(result)
}

// AddAgentInstance registers one more copy of an installed agent and lays out
// the state directory that keeps it apart from the others. Nothing is signed in
// yet: the instance is started, and the sign-in happens in the agent itself.
//
// The name is optional, and the pages leave it out: adding a copy is one click,
// and what to call it is answered later, once there is an account in it.
func (c *ApiController) AddAgentInstance() {
	if c.RequireAdmin() {
		return
	}

	var form struct {
		AgentId     string `json:"agentId"`
		Path        string `json:"path"`
		Owner       string `json:"owner"`
		Instance    string `json:"instance"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	installation, err := findInstallation(form.AgentId, form.Path, form.Owner)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !agent.SupportsInstances(installation.AgentId) {
		c.ResponseError(fmt.Sprintf("gateway cannot run a second copy of %s", installation.AgentId))
		return
	}

	if form.Instance == "" {
		if form.Instance, err = nextInstanceName(installation.AgentId); err != nil {
			c.ResponseError(err.Error())
			return
		}
	}
	if err := agent.CheckInstanceName(form.Instance); err != nil {
		c.ResponseError(err.Error())
		return
	}

	dataDir, err := agent.InstanceDir(installation.AgentId, form.Instance)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	instance := &object.AgentInstance{
		AgentId:     installation.AgentId,
		Instance:    form.Instance,
		DisplayName: form.DisplayName,
		DataDir:     dataDir,
		Path:        installation.Path,
		HostUser:    installation.Owner,
	}
	if err := object.AddAgentInstance(instance); err != nil {
		c.ResponseError(err.Error())
		return
	}
	// The row is what reserves the name, so it goes in first; an instance whose
	// state could not be laid out is no instance at all, and is taken back.
	if err := agent.PrepareInstance(installation, dataDir); err != nil {
		_ = object.DeleteAgentInstance(instance.Name)
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(instanceView(instance))
}

// UpdateAgentInstance renames one instance in the lists. Only the label is
// editable: the name the state directory was laid out under stays put.
func (c *ApiController) UpdateAgentInstance() {
	if c.RequireAdmin() {
		return
	}

	instance, ok := c.readAgentInstance()
	if !ok {
		return
	}
	var form struct {
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.SetAgentInstanceDisplayName(instance.Name, form.DisplayName); err != nil {
		c.ResponseError(err.Error())
		return
	}

	instance.DisplayName = form.DisplayName
	c.ResponseOk(instanceView(instance))
}

// nextInstanceName is what a copy is called when the caller names none. The
// installation itself is the first copy, so the numbering starts at the second.
func nextInstanceName(agentId string) (string, error) {
	instances, err := object.GetAgentInstances(agentId)
	if err != nil {
		return "", err
	}

	taken := make(map[string]bool, len(instances))
	for _, instance := range instances {
		taken[instance.Instance] = true
	}
	for number := 2; number <= maxInstances; number++ {
		if name := strconv.Itoa(number); !taken[name] {
			return name, nil
		}
	}
	return "", errors.New("this agent already has as many instances as Gateway will name")
}

// DeleteAgentInstance forgets one instance. Its state directory stays on disk:
// it holds a signed-in account and that account's work, which is the operator's
// to remove.
func (c *ApiController) DeleteAgentInstance() {
	if c.RequireAdmin() {
		return
	}

	instance, ok := c.readAgentInstance()
	if !ok {
		return
	}
	if err := object.DeleteAgentInstance(instance.Name); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(instance.DataDir)
}

// StartAgentInstance runs one instance against its own state directory, beside
// whichever other instances are already running.
func (c *ApiController) StartAgentInstance() {
	if c.RequireAdmin() {
		return
	}

	instance, ok := c.readAgentInstance()
	if !ok {
		return
	}
	installation, err := findInstallation(instance.AgentId, instance.Path, instance.HostUser)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := agent.PrepareInstance(installation, instance.DataDir); err != nil {
		c.ResponseError(err.Error())
		return
	}

	target, err := instanceTarget(installation, instance)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := agentprocess.Start(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	// A sign-in started in this copy comes back as a link in the agent's own
	// scheme, which would otherwise open the copy on the default state
	// directory. A copy with no account in it is one that is about to sign in.
	// The start is not worth failing over it: the copy runs either way, and the
	// pages show whether the link was captured.
	if agentlink.Supported() && agent.AccountOfInstance(instance.AgentId, instance.DataDir) == nil {
		if err := captureLink(instance, target); err != nil {
			beego.Error("the sign-in link of", instance.Name, "cannot be routed to it:", err)
		}
	}
	c.ResponseOk(instanceView(instance))
}

// CaptureAgentInstanceLink points the URL scheme of an agent at one copy, so
// that the next sign-in a browser hands back opens that one. A copy started
// without an account takes the scheme on its own; this is how a copy that is
// already running, or already signed in, takes it too.
func (c *ApiController) CaptureAgentInstanceLink() {
	if c.RequireAdmin() {
		return
	}

	instance, ok := c.readAgentInstance()
	if !ok {
		return
	}
	var form struct {
		Capture bool `json:"capture"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	scheme := agent.LinkSchemeOf(instance.AgentId)
	if scheme == "" {
		c.ResponseError(fmt.Sprintf("gateway knows no sign-in link of %s to route", instance.AgentId))
		return
	}
	if !form.Capture {
		if err := agentlink.Release(scheme); err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(instanceView(instance))
		return
	}

	installation, err := findInstallation(instance.AgentId, instance.Path, instance.HostUser)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	target, err := instanceTarget(installation, instance)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := captureLink(instance, target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(instanceView(instance))
}

// captureLink hands the next link in the agent's own scheme to one copy, which
// is opened with the same command the instance is started with.
func captureLink(instance *object.AgentInstance, target agentprocess.Target) error {
	scheme := agent.LinkSchemeOf(instance.AgentId)
	if scheme == "" {
		return fmt.Errorf("gateway knows no sign-in link of %s to route", instance.AgentId)
	}

	_, err := agentlink.Capture(scheme, instance.Name, agentlink.Target{
		Executable: target.Executable,
		Args:       target.Args,
	}, linkCaptureTtl)
	return err
}

// StopAgentInstance ends the processes of one instance, leaving the others as
// they are.
func (c *ApiController) StopAgentInstance() {
	if c.RequireAdmin() {
		return
	}

	instance, ok := c.readAgentInstance()
	if !ok {
		return
	}
	installation, err := findInstallation(instance.AgentId, instance.Path, instance.HostUser)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	target, err := instanceTarget(installation, instance)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := agentprocess.Stop(target); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(instanceView(instance))
}

// readAgentInstance resolves the request body against the stored instances, so
// that a request names one Gateway created rather than a directory of its own.
func (c *ApiController) readAgentInstance() (*object.AgentInstance, bool) {
	var form struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return nil, false
	}

	instance, err := object.GetAgentInstance(form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return nil, false
	}
	if instance == nil {
		c.ResponseError("no agent instance is stored under this name")
		return nil, false
	}
	return instance, true
}

func instanceView(instance *object.AgentInstance) *agentInstanceView {
	view := &agentInstanceView{
		AgentInstance: instance,
		Account:       agent.AccountOfInstance(instance.AgentId, instance.DataDir),
		Status:        agentprocess.Status{Pids: []int{}},
	}

	if scheme := agent.LinkSchemeOf(instance.AgentId); scheme != "" && agentlink.Supported() {
		view.CanCapture = true
		claim, pending := agentlink.Pending(scheme)
		view.Capturing = pending && claim.Instance == instance.Name
	}

	installation, err := findInstallation(instance.AgentId, instance.Path, instance.HostUser)
	if err == nil {
		var target agentprocess.Target
		if target, err = instanceTarget(installation, instance); err == nil {
			view.Desktop = target.Desktop
			view.Status = agentprocess.StatusOf(target)
			return view
		}
	}
	view.Status.Detail = err.Error()
	return view
}

func instanceTarget(installation agent.Installation, instance *object.AgentInstance) (agentprocess.Target, error) {
	launch, err := agent.InstanceLaunchOf(installation, instance.DataDir)
	if err != nil {
		return agentprocess.Target{}, err
	}
	return agentprocess.Target{
		AgentId:    installation.AgentId,
		Path:       installation.Path,
		Owner:      installation.Owner,
		Executable: launch.Executable,
		Args:       launch.Args,
		Desktop:    launch.Desktop,
		Marker:     instance.DataDir,
	}, nil
}

// findInstallation resolves an agent id, path and owner against the
// installations a scan actually found. Starting one runs a program, so an
// unverified path would let a caller name any file on the host.
func findInstallation(agentId string, path string, owner string) (agent.Installation, error) {
	installations, err := agent.Scan(false)
	if err != nil {
		return agent.Installation{}, err
	}
	requested := agentpatch.Target{AgentId: agentId, Path: path, Owner: owner}
	for _, installation := range installations {
		if matchesTarget(targetOf(installation), requested) {
			return installation, nil
		}
	}
	return agent.Installation{}, errors.New("no discovered agent installation matches this target")
}

// instanceMarkers are the state directories of one agent's instances, so an
// installation's own run state does not count their processes as its own.
func instanceMarkers(agentId string) []string {
	if !agent.SupportsInstances(agentId) {
		return nil
	}
	instances, err := object.GetAgentInstances(agentId)
	if err != nil || len(instances) == 0 {
		return nil
	}

	markers := make([]string, 0, len(instances))
	for _, instance := range instances {
		markers = append(markers, instance.DataDir)
	}
	return markers
}
