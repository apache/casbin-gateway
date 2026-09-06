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
	"os"
	"time"

	"github.com/apache/casbin-gateway/agenthook"
	"github.com/apache/casbin-gateway/agentlink"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/agentpatch"
	"github.com/apache/casbin-gateway/casdoor"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/controllers"
	"github.com/apache/casbin-gateway/mcpproxy"
	"github.com/apache/casbin-gateway/mcpserver"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/routers"
	"github.com/apache/casbin-gateway/service"
	"github.com/apache/casbin-gateway/util"
	"github.com/apache/casbin-gateway/version"
	"github.com/beego/beego"
)

// daemonLogPath is where a background Gateway sends the console output nobody
// is watching.
const daemonLogPath = "./logs/casbin-gateway.out"

func main() {
	// Hooks and MCP servers are launched by an agent as a short-lived child
	// process. They must exit before Gateway initializes its own services.
	agenthook.ServeIfInvoked()
	mcpserver.ServeIfInvoked()
	mcpproxy.ServeIfInvoked()

	// A link in the URL scheme of an agent opens this executable while Gateway
	// is routing one to a particular copy. It is a launch, not a service.
	agentlink.HandleIfInvoked()

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

	// "uninstall" gives back what installing took outside the install
	// directory, which deleting that directory cannot do for itself.
	if service.RunUninstall(os.Args, conf.GetHttpPort()) {
		return
	}

	// An update that restarted into this process left the version it replaced
	// on disk, and this is where that is either finished with or put back.
	version.Configure(daemonLogPath)
	version.BeginStartup()
	defer version.RollBackOnPanic()

	// A URL scheme is only Gateway while it waits for one link. A Gateway that
	// did not get to hand one over left the scheme registered to itself, and
	// this is the only process that can now give it back.
	agentlink.Restore()

	// The tray runs as a process of its own, so the credential it presents in
	// place of a session has to be on disk before it asks for anything.
	if err := service.IssueLocalToken(); err != nil {
		beego.Error("the desktop tray cannot be given a local token:", err)
	}

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
	// The probe suite is stored, so the shipped cases have to exist before
	// anything can run one.
	object.InitProbeCases()
	// Probes go out over the same transport, so they can only start once it is
	// configured. The sweep waits a while longer before spending anything.
	object.StartProviderProbeSweep()
	// Prices are read over the same transport, and only when the sync is on.
	object.StartModelsDevSync()
	// A snapshot of the configuration, taken on a schedule and beside the
	// database rather than inside it.
	object.StartBackupSchedule()

	agentmonitor.Configure(
		conf.GetAgentPatchStateDir(),
		time.Duration(conf.GetAgentMonitorPollSeconds())*time.Second,
	)
	// Monitoring observes; the database is what keeps what it saw, so records
	// outlive the process that collected them.
	object.StartAgentRecordWriter()
	defer object.StopAgentRecordWriter()
	agentmonitor.SetRecordSink(object.AddAgentRecord)
	if err := agentmonitor.Start(); err != nil {
		beego.Error("agent monitor could not start:", err)
	}
	defer agentmonitor.Stop()

	// The conversations Gateway drives on somebody's behalf, put back where a
	// restart left them.
	object.InitAgentSessions()
	// A driven agent answers through the provider bound to it, so a session
	// needs no sign-in of the agent's own.
	service.InitAgentSessionEnv()

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
		object.StopAgentRecordWriter()
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

	controllers.InitSessions()

	port := conf.GetHttpPort()
	addr := conf.GetHttpAddr()

	// A previous Gateway still holding the management port would keep this one
	// from starting, so it is stopped first - and the port it held stays busy
	// for a moment after it goes, which a restart into an update runs straight
	// into. beego.Run() would report that as a stack trace, so the port is
	// claimed here to explain it in one line instead.
	if err := util.ClaimPort(addr, port); err != nil {
		// Being the version an update just installed and not being able to
		// serve is what a rollback is for: the machine keeps a Gateway. A port
		// held by another program is not that, and putting the previous version
		// back would take the update away without freeing anything.
		var foreign *util.ForeignPortError
		if !errors.As(err, &foreign) && version.RollBackFailedStart(err.Error()) {
			return
		}
		util.FatalListenError(port, `change "httpport" in conf/app.conf`, err)
	}

	// Nothing is left to roll back to from here: this version has the port.
	version.FinishStartup()

	service.PrintStartupSummary()

	// The agent endpoint of a sandboxed agent names this host's own address, and
	// the configuration naming it outlives this process.
	if err := service.SyncLanAccess(); err != nil {
		beego.Error("the endpoint of an agent that runs in a sandbox cannot be served:", err)
	}

	beego.Run(fmt.Sprintf("%s:%v", addr, port))
}
