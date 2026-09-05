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
	"path/filepath"
	"strings"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/agentauth"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/object"
)

// agentAccountView is one stored sign-in as the page lists it, with whether it
// is the one the agent is using now. The credential itself never leaves here.
type agentAccountView struct {
	*object.AgentAccount
	Current bool `json:"current"`
}

// agentAccountsView is the accounts of one agent and what it is signed in to,
// which may be an account Gateway has not been given a copy of yet.
type agentAccountsView struct {
	Accounts []agentAccountView `json:"accounts"`
	// Current is the sign-in in place, nil when the agent has none.
	Current *agentauth.Credential `json:"current,omitempty"`
	// Stored marks a sign-in in place that Gateway already holds a copy of, so
	// the page knows whether switching away from it would lose it.
	Stored bool `json:"stored"`
	// Home is the directory the sign-in is read from.
	Home string `json:"home,omitempty"`
	// SignIn is the program a browser sign-in would be run with, empty when
	// there is none on this machine.
	SignIn string `json:"signIn,omitempty"`
}

// GetAgentAccounts lists the sign-ins stored for one agent beside the one it is
// using now.
func (c *ApiController) GetAgentAccounts() {
	if c.RequireAdmin() {
		return
	}

	agentId := c.Input().Get("agent")
	accounts, err := object.GetAgentAccounts(agentId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	view := agentAccountsView{Accounts: make([]agentAccountView, 0, len(accounts))}
	// The path names an installation, which is what says whose home the sign-in
	// in place is read from. A listing without one is the stored rows alone.
	if path := c.Input().Get("path"); path != "" && agentauth.Supports(agentId) {
		installation, err := findInstallation(agentId, path, c.Input().Get("owner"))
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		if view.Home, err = agentauth.HomeOf(agentId, installation.Path, installation.Owner); err != nil {
			c.ResponseError(err.Error())
			return
		}
		if view.Current, err = agentauth.Read(agentId, view.Home); err != nil {
			c.ResponseError(err.Error())
			return
		}
		if program, err := codexLoginProgram(installation); err == nil {
			view.SignIn = program
		}
	}

	for _, account := range accounts {
		current := view.Current != nil && account.Matches(*view.Current)
		view.Stored = view.Stored || current
		view.Accounts = append(view.Accounts, agentAccountView{AgentAccount: account, Current: current})
	}
	c.ResponseOk(view)
}

// agentAccountForm is what the account calls take: the installation, and
// whichever of the stored name, the label and the key the call needs.
type agentAccountForm struct {
	agentpatch.Target
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	ApiKey      string `json:"apiKey"`
}

// SaveAgentAccount keeps a copy of the sign-in the agent is using now, so that
// switching away from it can be undone without signing in again.
func (c *ApiController) SaveAgentAccount() {
	if c.RequireAdmin() {
		return
	}

	form, home, ok := c.readAgentAccountForm()
	if !ok {
		return
	}
	credential, err := agentauth.Read(form.AgentId, home)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if credential == nil {
		c.ResponseError(fmt.Sprintf("%s is not signed in to anything to save", form.AgentId))
		return
	}

	account, err := object.SaveAgentAccount(form.AgentId, *credential, form.DisplayName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentAccountView{AgentAccount: account, Current: true})
}

// AddAgentAccount stores an API key as an account of its own, so the key and a
// subscription sign-in are swapped the same way.
func (c *ApiController) AddAgentAccount() {
	if c.RequireAdmin() {
		return
	}

	form, _, ok := c.readAgentAccountForm()
	if !ok {
		return
	}
	credential, err := agentauth.ApiKeyCredential(form.AgentId, form.ApiKey)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	account, err := object.SaveAgentAccount(form.AgentId, credential, form.DisplayName)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentAccountView{AgentAccount: account})
}

// SwitchAgentAccount puts one stored sign-in back into the agent. What is there is
// saved first: the agent keeps one credential file, and this overwrites it.
func (c *ApiController) SwitchAgentAccount() {
	if c.RequireAdmin() {
		return
	}

	form, home, ok := c.readAgentAccountForm()
	if !ok {
		return
	}
	account, err := object.GetAgentAccount(form.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if account == nil {
		c.ResponseError(fmt.Sprintf("no agent account is stored under this name: %s", form.Name))
		return
	}
	if account.AgentId != form.AgentId {
		c.ResponseError(fmt.Sprintf("%s is signed in to %s, not to this agent", form.Name, account.AgentId))
		return
	}

	replaced, err := agentauth.Read(form.AgentId, home)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if replaced != nil {
		if _, err := object.SaveAgentAccount(form.AgentId, *replaced, ""); err != nil {
			c.ResponseError(err.Error())
			return
		}
	}

	if err := agentauth.Write(home, account.Credential); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.TouchAgentAccount(account.Name); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(agentAccountView{AgentAccount: account, Current: true})
}

// UpdateAgentAccount renames one stored account in the lists.
func (c *ApiController) UpdateAgentAccount() {
	if c.RequireAdmin() {
		return
	}

	var form agentAccountForm
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.SetAgentAccountDisplayName(form.Name, strings.TrimSpace(form.DisplayName)); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk()
}

// DeleteAgentAccount forgets one stored sign-in. The agent keeps whatever it is
// using: this drops Gateway's copy, it does not sign anybody out.
func (c *ApiController) DeleteAgentAccount() {
	if c.RequireAdmin() {
		return
	}

	var form agentAccountForm
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err := object.DeleteAgentAccount(form.Name); err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk()
}

// SignInAgentAccount starts a browser sign-in and stores the account it brings
// back. It runs against a directory of its own, so the account in the agent is
// untouched until it is switched to.
func (c *ApiController) SignInAgentAccount() {
	if c.RequireAdmin() {
		return
	}

	installation, ok := c.readAgentInstallation()
	if !ok {
		return
	}
	if !agentauth.Supports(installation.AgentId) {
		c.ResponseError(fmt.Sprintf("gateway cannot sign %s in", installation.AgentId))
		return
	}
	program, err := codexLoginProgram(installation)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	agentId := installation.AgentId
	session, err := agentauth.StartLogin(agentId, program, func(credential agentauth.Credential) error {
		_, err := object.SaveAgentAccount(agentId, credential, "")
		return err
	})
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(session)
}

// GetAgentSignin reports how a sign-in that was started is getting on. The page
// polls it: a browser sign-in takes as long as whoever is at the machine.
func (c *ApiController) GetAgentSignin() {
	if c.RequireAdmin() {
		return
	}

	session, ok := agentauth.LoginSession(c.Input().Get("id"))
	if !ok {
		c.ResponseError("no sign-in was started under this id")
		return
	}
	c.ResponseOk(session)
}

// readAgentAccountForm resolves the body against the installations a scan found
// and answers with the directory its sign-in is read from. Writing there is
// writing into somebody's home, so an unverified body would name any of them.
func (c *ApiController) readAgentAccountForm() (agentAccountForm, string, bool) {
	var form agentAccountForm
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return form, "", false
	}

	installation, err := findInstallation(form.AgentId, form.Path, form.Owner)
	if err != nil {
		c.ResponseError(err.Error())
		return form, "", false
	}
	home, err := agentauth.HomeOf(installation.AgentId, installation.Path, installation.Owner)
	if err != nil {
		c.ResponseError(err.Error())
		return form, "", false
	}
	form.AgentId = installation.AgentId
	return form, home, true
}

// codexLoginProgram is the Codex program a sign-in is run with. The desktop app
// is opened by its own window rather than by a command, so the CLI beside it -
// which the same scan finds, and which reads the same ~/.codex - is what signs
// in for both.
func codexLoginProgram(installation agent.Installation) (string, error) {
	if program := codexProgramOf(installation); program != "" {
		return program, nil
	}

	installations, err := agent.Scan(false)
	if err != nil {
		return "", err
	}
	for _, other := range installations {
		if !agentauth.Supports(other.AgentId) {
			continue
		}
		if program := codexProgramOf(other); program != "" {
			return program, nil
		}
	}
	return "", errors.New("gateway found no Codex program on this machine to sign in with")
}

func codexProgramOf(installation agent.Installation) string {
	executable := agent.LaunchOf(installation).Executable
	if executable == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(filepath.Base(executable)), "codex") {
		return executable
	}
	return ""
}
