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
	"fmt"
	"time"

	"github.com/apache/casbin-gateway/agenthook"
	"github.com/apache/casbin-gateway/agentmonitor"
	"github.com/apache/casbin-gateway/casdoor"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/ip"
	"github.com/apache/casbin-gateway/mcpserver"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/proxy"
	"github.com/apache/casbin-gateway/routers"
	"github.com/apache/casbin-gateway/run"
	"github.com/apache/casbin-gateway/service"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
	"github.com/beego/beego/plugins/cors"
	_ "github.com/beego/beego/session/redis"
)

func main() {
	// Hooks and MCP servers are launched by an agent as a short-lived child
	// process. They must exit before Gateway initializes its own services.
	agenthook.ServeIfInvoked()
	mcpserver.ServeIfInvoked()

	util.InitSelfGuard()
	object.InitFlag()
	object.InitAdapter()
	object.CreateTables()
	casdoor.InitCasdoorConfig()
	object.InitUsers()
	proxy.InitHttpClient()
	ip.InitIpDb()
	object.InitSiteMap()
	object.InitRuleMap()
	run.InitAppMap()
	run.InitRdsClient()
	run.InitSelfStart()
	object.StartMonitorSitesLoop()

	agentmonitor.Configure(
		conf.GetAgentPatchStateDir(),
		time.Duration(conf.GetAgentMonitorPollSeconds())*time.Second,
		conf.GetAgentRecordCapacity(),
	)
	if err := agentmonitor.Start(); err != nil {
		beego.Error("agent monitor could not start:", err)
	}
	defer agentmonitor.Stop()

	beego.InsertFilter("*", beego.BeforeRouter, cors.Allow(&cors.Options{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "PUT", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "X-Requested-With", "Content-Type", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

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

	port := conf.GetHttpPort()

	// A previous run still holding one of these ports would keep this one from
	// starting, so it is stopped first. The gateway ports come before the
	// management port because service.Start() binds them first.
	stopPorts := []int{}
	if conf.IsGatewayEnabled() {
		stopPorts = append(stopPorts, conf.GetGatewayHttpPort(), conf.GetGatewayHttpsPort())
	}
	stopPorts = append(stopPorts, port)
	for _, stopPort := range stopPorts {
		if err := util.StopOldInstance(stopPort); err != nil {
			// The bind below reports the conflict in full, so a failed kill only
			// needs a note here and never stops the startup by itself.
			fmt.Printf("Casbin Gateway: could not free port %d: %v\n", stopPort, err)
		}
	}

	service.PrintStartupSummary()

	// beego.Run() binds the management port itself and reports a conflict as a
	// stack trace, so the port is probed here first to explain it in one line,
	// and before the gateway below binds anything. The gap between probe and
	// bind is unavoidable but harmless: losing the race just puts us back to
	// beego's own error.
	if err := util.CheckPortAvailable(port); err != nil {
		util.FatalListenError(port, `change "httpport" in conf/app.conf`, err)
	}

	service.Start()

	beego.Run(fmt.Sprintf(":%v", port))
}
