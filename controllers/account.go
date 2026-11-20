// Copyright 2023 The casbin Authors. All Rights Reserved.
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
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/apache/casbin-gateway/auth"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
)

type signinForm struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type accountForm struct {
	DisplayName     string `json:"displayName"`
	Avatar          string `json:"avatar"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// GetSigninOptions tells the web UI how to sign in: redirect to Casdoor, or
// show the built-in username/password form.
func (c *ApiController) GetSigninOptions() {
	casdoorAvailable := conf.IsCasdoorAvailable()
	signinEnabled := object.IsSigninEnabled()

	c.ResponseOk(map[string]interface{}{
		"casdoorAvailable": casdoorAvailable,
		"signinAvailable":  signinEnabled,
		"autoSignin":       c.canAutoSignin(),
		"authConfig": map[string]string{
			"serverUrl":        getUnquotedConfig("casdoorEndpoint"),
			"clientId":         getUnquotedConfig("clientId"),
			"appName":          getUnquotedConfig("casdoorApplication"),
			"organizationName": getUnquotedConfig("casdoorOrganization"),
		},
	})
}

// getUnquotedConfig reads an app.conf value, dropping the quotes that beego
// keeps around values such as casdoorApplication = "app-casibase".
func getUnquotedConfig(key string) string {
	return strings.Trim(conf.GetConfigString(key), `"' `)
}

// Signin handles both sign-in flows: with an OAuth code it completes the
// Casdoor login, without one it falls back to the local password login.
func (c *ApiController) Signin() {
	code := c.Input().Get("code")
	state := c.Input().Get("state")
	if code == "" && state == "" {
		c.signinWithPassword()
		return
	}

	if !conf.IsCasdoorAvailable() {
		c.ResponseError("Casdoor is not configured")
		return
	}

	token, err := auth.GetOAuthToken(code, state)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	claims, err := auth.ParseJwtToken(token.AccessToken)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	claims.AccessToken = token.AccessToken
	c.completeSignin(claims)
}

func (c *ApiController) signinWithPassword() {
	if !object.IsSigninEnabled() {
		c.ResponseError("sign in is unavailable")
		return
	}

	form := signinForm{}
	if len(c.Ctx.Input.RequestBody) > 0 {
		if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
			c.ResponseError(err.Error())
			return
		}
	}
	if conf.GetRemoteOrigin() == "" {
		if form.Username == "" {
			form.Username = c.Input().Get("username")
		}
		if form.Password == "" {
			form.Password = c.Input().Get("password")
		}
	}

	user, ok, err := object.VerifyUser(form.Username, form.Password)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if !ok {
		c.ResponseError("invalid username or password")
		return
	}

	claims := &auth.Claims{User: user.ToCasdoorUser()}
	c.completeSignin(claims)
}

func (c *ApiController) canAutoSignin() bool {
	return conf.GetRemoteOrigin() == "" && util.IsLoopbackRequest(c.Ctx.Request) &&
		object.IsSigninEnabled() && object.IsAdminUsingDefaultPassword()
}

func (c *ApiController) Signout() {
	c.SetSessionClaims(nil)

	c.ResponseOk()
}

// autoLoginAdmin signs the built-in admin in while the seeded password is still
// in use, so that a fresh install is usable without any configuration. Only a
// request from this machine is let through: the convenience is for the person
// sitting at the keyboard, and the seeded password is public.
// It returns false when the response has already been written.
func (c *ApiController) autoLoginAdmin() bool {
	user, ok, err := object.VerifyUser("admin", "123")
	if err != nil {
		c.ResponseError(err.Error())
		return false
	}
	if !ok {
		c.ResponseError("please sign in first")
		return false
	}

	c.SetSessionClaims(&auth.Claims{User: user.ToCasdoorUser()})
	return true
}

func (c *ApiController) GetAccount() {
	if c.GetSessionUser() == nil {
		fromPath := c.Input().Get("fromPath")
		if fromPath != "/signin" && c.canAutoSignin() {
			if !c.autoLoginAdmin() {
				return
			}
		} else {
			c.ResponseError("please sign in first")
			return
		}
	}

	claims := c.GetSessionClaims()
	hostname := util.GetHostname()

	c.ResponseOk(claims, hostname)
}

// UpdateAccount lets a locally signed-in user edit their own profile and
// password. Casdoor-backed accounts are managed in Casdoor itself.
func (c *ApiController) UpdateAccount() {
	sessionUser := c.GetSessionUser()
	if sessionUser == nil || sessionUser.Owner != object.UserOwner {
		c.ResponseError("unauthorized operation")
		return
	}

	form := accountForm{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &form); err != nil {
		c.ResponseError(err.Error())
		return
	}

	user, err := object.GetUser(sessionUser.Name)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if user == nil {
		c.ResponseError("unauthorized operation")
		return
	}

	if form.NewPassword != "" && !object.CheckUserPassword(user, form.CurrentPassword) {
		c.ResponseError("invalid username or password")
		return
	}

	user.DisplayName = form.DisplayName
	user.Avatar = form.Avatar
	if err = object.UpdateUserProfile(user); err != nil {
		c.ResponseError(err.Error())
		return
	}
	if form.NewPassword != "" {
		if err = object.UpdateUserPassword(user, form.NewPassword); err != nil {
			c.ResponseError(err.Error())
			return
		}
	}

	casdoorUser := user.ToCasdoorUser()
	c.SetSessionUser(&casdoorUser)
	c.ResponseOk(casdoorUser)
}
