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

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/apache/casbin-gateway/agenthook"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/casdoor"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/mcpserver"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/routers"
	"github.com/apache/casbin-gateway/service"
	"github.com/apache/casbin-gateway/util"
	"github.com/apache/casbin-gateway/version"
	"github.com/beego/beego"
	_ "github.com/beego/beego/session/redis"
)

// daemonLogPath is where a background Gateway sends the console output nobody
// is watching.
const daemonLogPath = "./logs/casbin-gateway.out"

func main() {
	// Hooks and MCP servers are launched by an agent as a short-lived child
	// process. They must exit before Gateway initializes its own services.
	agenthook.ServeIfInvoked()
	mcpserver.ServeIfInvoked()

	// "version" prints the build and exits. An update runs it on what it just
	// downloaded, before that executable replaces this one.
	if version.RunCommand(os.Args) {
		return
	}

	// "start", "stop" and "status" manage a Gateway running in the background,
	// so that using it does not mean keeping a terminal open.
	if util.RunCommand(os.Args, conf.GetHttpPort(), daemonLogPath) {
		return
	}

	// The executable an earlier update replaced is only removable once nothing
	// is running from it, which is now.
	version.CleanupBackup()
	version.Configure(daemonLogPath)

	object.InitFlag()
	object.InitAdapter()
	object.CreateTables()
	// The built-in Setting row answers conf from here on, so it has to be loaded
	// before anything reads a setting out of conf.
	object.InitBuiltInSetting()
	object.StartLlmRecordWriter()
	defer object.StopLlmRecordWriter()
	casdoor.InitCasdoorConfig()
	object.InitUsers()
	proxy.InitHttpClient()
	// Probes go out over the same transport, so they can only start once it is
	// configured. The sweep waits a while longer before spending anything.
	object.StartProviderProbeSweep()

	agentmonitor.Configure(
		conf.GetAgentPatchStateDir(),
		time.Duration(conf.GetAgentMonitorPollSeconds())*time.Second,
		conf.GetAgentRecordCapacity(),
	)
	if err := agentmonitor.Start(); err != nil {
		beego.Error("agent monitor could not start:", err)
	}
	defer agentmonitor.Stop()

	// Monitoring is on by default, so the agents already on this host are
	// patched without anyone opening the UI. The scan walks the disk, hence the
	// goroutine.
	go func() {
		if err := agentpatch.EnableAll(); err != nil {
			beego.Error("agent monitoring could not be enabled:", err)
		}
	}()

	// An update ends this process without unwinding main, so what the deferred
	// calls above would have flushed has to be flushed there instead.
	version.BeforeRestart = func() {
		agentmonitor.Stop()
		object.StopLlmRecordWriter()
	}

	// Before anything else: a request from a browser has to be one this Gateway
	// serves the pages of, not one a page on another site made with the
	// operator's session.
	beego.InsertFilter("*", beego.BeforeRouter, routers.OriginFilter)

	// https://studygolang.com/articles/2303
	beego.InsertFilter("/", beego.BeforeRouter, routers.TransparentStatic) // must has this for default page
	beego.InsertFilter("/*", beego.BeforeRouter, routers.TransparentStatic)
	beego.InsertFilter("/api/*", beego.BeforeRouter, routers.ApiFilter)

	if beego.AppConfig.String("redisEndpoint") == "" {
		beego.BConfig.WebConfig.Session.SessionProvider = "file"
		beego.BConfig.WebConfig.Session.SessionProviderConfig = "./tmp"
	} else {
		beego.BConfig.WebConfig.Session.SessionProvider = "redis"
		beego.BConfig.WebConfig.Session.SessionProviderConfig = beego.AppConfig.String("redisEndpoint")
	}
	beego.BConfig.WebConfig.Session.SessionGCMaxLifetime = 3600 * 24 * 365
	// The session is only ever presented by Gateway's own pages, so the browser
	// is told to keep it off requests another site makes.
	beego.BConfig.WebConfig.Session.SessionCookieSameSite = http.SameSiteStrictMode

	port := conf.GetHttpPort()
	addr := conf.GetHttpAddr()

	// A previous Gateway still holding the management port would keep this one
	// from starting, so it is stopped first.
	err := util.StopOldInstance(port)
	if err != nil {
		// A port held by something that is not Gateway stays with it; the bind
		// below reports the conflict in full, so a failed kill only needs a note.
		var foreign *util.ForeignPortError
		if !errors.As(err, &foreign) {
			fmt.Printf("Casbin Gateway: could not free port %d: %v\n", port, err)
		}
	}

	service.PrintStartupSummary()

	// beego.Run() binds the management port itself and reports a conflict as a
	// stack trace, so the port is probed here first to explain it in one line.
	// The gap between probe and bind is unavoidable but harmless: losing the
	// race just puts us back to beego's own error.
	if err := util.CheckPortAvailableOn(addr, port); err != nil {
		util.FatalListenError(port, `change "httpport" in conf/app.conf`, err)
	}

	// The agent endpoint of a sandboxed agent names this host's own address, and
	// the configuration naming it outlives this process.
	if err := service.SyncLanAccess(); err != nil {
		beego.Error("the endpoint of an agent that runs in a sandbox cannot be served:", err)
	}

	beego.Run(fmt.Sprintf("%s:%v", addr, port))
}
