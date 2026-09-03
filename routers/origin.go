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

package routers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego/context"
)

// OriginFilter keeps a page on another site from driving Gateway through the
// browser of the person sitting at this machine. Loopback is not a trust
// boundary in a browser: the local admin is signed in without a password, so
// any page the operator visits can reach 127.0.0.1 with those rights.
//
// Two checks, and a client that is not a browser — curl, an agent CLI — passes
// both without sending anything. The Host has to name this machine, which the
// attacker domain of a DNS rebinding does not, and a request carrying an Origin
// has to come from Gateway's own pages or from an origin the operator listed.
func OriginFilter(ctx *context.Context) {
	host := ctx.Request.Host
	if !isAllowedHost(host) {
		denyOrigin(ctx, fmt.Sprintf("%q is not a name this Gateway answers to; add it to \"allowedHosts\" in conf/app.conf to reach Gateway under that name",
			hostnameOf(host)))
		return
	}

	origin := ctx.Input.Header("Origin")
	if origin == "" || sameOrigin(origin, host) {
		return
	}
	if !isAllowedOrigin(origin) {
		denyOrigin(ctx, fmt.Sprintf("%s is another site, and Gateway is not callable from one; add it to \"allowedOrigins\" in conf/app.conf to allow it", origin))
		return
	}

	// An origin the operator listed gets the CORS headers the browser needs to
	// hand the answer to it.
	writeCorsHeaders(ctx, origin)
	if isPreflight(ctx) {
		ctx.ResponseWriter.WriteHeader(http.StatusOK)
	}
}

func denyOrigin(ctx *context.Context, message string) {
	ctx.Output.SetStatus(http.StatusForbidden)
	responseError(ctx, message)
}

// isAllowedHost reports whether the Host header names this machine. An address
// is always one: rebinding needs a name to move, so a literal is what the
// caller really dialed.
func isAllowedHost(host string) bool {
	name := hostnameOf(host)
	if name == "" {
		return true
	}
	if net.ParseIP(name) != nil {
		return true
	}

	lower := strings.ToLower(name)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	if isOwnHostname(lower) {
		return true
	}

	for _, allowed := range conf.GetAllowedHosts() {
		if strings.EqualFold(hostnameOf(allowed), name) {
			return true
		}
	}
	return false
}

// isOwnHostname matches this host's own name, both bare and as the first label
// of the fully qualified form the same machine answers to.
func isOwnHostname(lower string) bool {
	own := strings.ToLower(util.GetHostname())
	if own == "" {
		return false
	}
	return lower == own || strings.HasPrefix(lower, own+".")
}

func isAllowedOrigin(origin string) bool {
	for _, allowed := range conf.GetAllowedOrigins() {
		if strings.EqualFold(strings.TrimSuffix(allowed, "/"), origin) {
			return true
		}
	}
	return false
}

// sameOrigin reports whether the Origin is Gateway itself, which is what every
// page it serves sends: the web UI is served from here, and the dev server
// proxies to here under its own name.
func sameOrigin(origin string, host string) bool {
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, host)
}

func isPreflight(ctx *context.Context) bool {
	return ctx.Request.Method == http.MethodOptions && ctx.Input.Header("Access-Control-Request-Method") != ""
}

func writeCorsHeaders(ctx *context.Context, origin string) {
	header := ctx.ResponseWriter.Header()
	// The answer differs per origin, so a cache must not serve one origin's
	// answer to another.
	header.Add("Vary", "Origin")
	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Credentials", "true")
	if !isPreflight(ctx) {
		return
	}

	header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	if requested := ctx.Input.Header("Access-Control-Request-Headers"); requested != "" {
		header.Set("Access-Control-Allow-Headers", requested)
	}
	header.Set("Access-Control-Max-Age", "600")
}

func hostnameOf(host string) string {
	if host == "" {
		return ""
	}
	if name, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(name, "[]")
	}
	return strings.Trim(host, "[]")
}
