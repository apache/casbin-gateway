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
	"net/http"
	"net/url"

	"github.com/beego/beego"
	"github.com/beego/beego/session"
	_ "github.com/beego/beego/session/redis"
)

// InitSessions takes the session away from beego and gives it to the
// controller. With SessionOn, beego starts one for every request that reaches
// the router, minting a new id for each request that arrives without a cookie.
// A page load makes several of those at once, so the browser is handed several
// ids and keeps whichever response lands last — discarding the session the
// sign-in on another of them had just written, which is why a freshly opened UI
// reported "please sign in first" until it was reloaded.
func InitSessions() {
	beego.BConfig.WebConfig.Session.SessionOn = false

	config := &session.ManagerConfig{
		CookieName:      beego.BConfig.WebConfig.Session.SessionName,
		EnableSetCookie: true,
		Gclifetime:      3600 * 24 * 365,
		Secure:          beego.BConfig.Listen.EnableHTTPS,
		// The session is only ever presented by Gateway's own pages, so the
		// browser is told to keep it off requests another site makes.
		CookieSameSite: http.SameSiteStrictMode,
		// Sessions are kept in files beside the binary, or in Redis when one is
		// configured.
		ProviderConfig: "./tmp",
	}
	provider := "file"
	if redisEndpoint := beego.AppConfig.String("redisEndpoint"); redisEndpoint != "" {
		provider = "redis"
		config.ProviderConfig = redisEndpoint
	}

	sessions, err := session.NewManager(provider, config)
	if err != nil {
		panic(err)
	}

	beego.GlobalSessions = sessions
	go sessions.GC()
}

// sessionStore is this request's session, or nil when it has none: it is read
// only when the request carries one, and created only when something is about
// to be stored in it.
func (c *ApiController) sessionStore(create bool) session.Store {
	if c.CruSession != nil {
		return c.CruSession
	}
	if !create && !c.hasSession() {
		return nil
	}

	store, err := beego.GlobalSessions.SessionStart(c.Ctx.ResponseWriter, c.Ctx.Request)
	if err != nil {
		beego.Error("the session could not be started:", err)
		return nil
	}

	c.CruSession = store
	return store
}

// hasSession reports whether the request presents a session that still exists,
// so that a cookie left over from a session that is gone is not answered with a
// new one.
func (c *ApiController) hasSession() bool {
	cookie, err := c.Ctx.Request.Cookie(beego.BConfig.WebConfig.Session.SessionName)
	if err != nil || cookie.Value == "" {
		return false
	}

	sid, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return false
	}

	return beego.GlobalSessions.GetProvider().SessionExist(sid)
}
