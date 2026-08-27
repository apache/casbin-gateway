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

package service

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/apache/casbin-gateway/agent"
	"github.com/apache/casbin-gateway/conf"
	"github.com/apache/casbin-gateway/object"
	"github.com/apache/casbin-gateway/util"
	"github.com/beego/beego"
)

var (
	lanMutex  sync.Mutex
	lanServer *http.Server
)

// LanHost is the address an agent that runs in a sandbox reaches Gateway at.
// Inside such a sandbox 127.0.0.1 is the sandbox itself, so the loopback
// endpoint every other agent is given never arrives here.
func LanHost() (string, error) {
	addr := conf.GetHttpAddr()
	if ip := net.ParseIP(addr); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
		return addr, nil
	}
	return util.LanIPv4()
}

// SyncLanAccess makes the management port answer on LanHost() for as long as an
// agent that runs in a sandbox is routed through the gateway, and only then: the
// endpoint such an agent is given has to be served, while every other agent is
// reached over loopback and needs nothing opened.
//
// It takes effect without a restart and leaves httpaddr alone — one more address
// is exactly what the agent needs, where widening the bind would also serve
// every other interface of this host. Requests arriving there are off-box as far
// as the rest of Gateway is concerned, so they carry the relay token, which is
// written into the agent's configuration along with the endpoint.
func SyncLanAccess() error {
	needed, err := sandboxedAgentIsBound()
	if err != nil {
		return err
	}
	if !needed {
		closeLanAccess()
		return nil
	}

	host, err := LanHost()
	if err != nil {
		return err
	}
	// A non-loopback httpaddr is already served by beego itself.
	if !conf.IsHttpAddrLoopback() {
		return nil
	}

	address := net.JoinHostPort(host, strconv.Itoa(conf.GetHttpPort()))
	lanMutex.Lock()
	defer lanMutex.Unlock()
	if lanServer != nil {
		if lanServer.Addr == address {
			return nil
		}
		// The host changed networks since the listener was opened.
		_ = lanServer.Close()
		lanServer = nil
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("cannot serve the agent endpoint on %s: %w", address, err)
	}
	server := &http.Server{Addr: address, Handler: beego.BeeApp.Handlers}
	lanServer = server

	go func() {
		err := server.Serve(listener)
		lanMutex.Lock()
		if lanServer == server {
			lanServer = nil
		}
		lanMutex.Unlock()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			beego.Error("the agent endpoint on", address, "stopped serving:", err)
		}
	}()

	fmt.Printf("Casbin Gateway: also serving http://%s, which is where an agent running in a sandbox reaches it\n", address)
	return nil
}

func closeLanAccess() {
	lanMutex.Lock()
	server := lanServer
	lanServer = nil
	lanMutex.Unlock()
	if server == nil {
		return
	}

	_ = server.Close()
	fmt.Printf("Casbin Gateway: stopped serving http://%s, no agent runs in a sandbox any more\n", server.Addr)
}

// sandboxedAgentIsBound reports whether any agent that runs in a sandbox reaches
// its provider through this gateway. In direct mode the agent is given the
// provider's own upstream and never calls here.
func sandboxedAgentIsBound() (bool, error) {
	agents, err := object.GetAgents()
	if err != nil {
		return false, err
	}

	for id, routing := range agents {
		if routing.Provider != "" && routing.Mode == object.ModeGateway && agent.RunsSandboxed(id) {
			return true, nil
		}
	}
	return false, nil
}
