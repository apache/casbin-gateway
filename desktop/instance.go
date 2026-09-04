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

package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	showRequest  = "show"
	showReply    = "ok"
	instanceWait = 2 * time.Second
)

// instanceMarker records the loopback port the running tray listens on. The
// port is ephemeral rather than fixed, because a fixed one would sooner or
// later be a port something else on the machine wanted - a second Gateway home
// keeps a marker of its own next to its own data.
func instanceMarker() string {
	return filepath.Join(gatewayHome(), ".desktop-instance")
}

// raiseRunningInstance hands this launch to the tray that is already running.
// Clicking the shortcut of an app that is open means "show me the window", so
// what would have been a second tray icon and a second window becomes the one
// that is there, raised. It reports whether a tray took the launch.
func raiseRunningInstance() bool {
	data, err := os.ReadFile(instanceMarker())
	if err != nil {
		return false
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port <= 0 {
		return false
	}

	// A marker that outlived the tray which wrote it leaves the launch to run
	// the tray itself, which is also what overwrites the marker.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), instanceWait)
	if err != nil {
		return false
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(instanceWait))
	if _, err = io.WriteString(conn, showRequest+"\n"); err != nil {
		return false
	}

	// Whatever holds that port now may not be a Gateway tray, so the launch is
	// only given up once one has answered.
	reply, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(reply) == showReply
}

// holdInstance opens the socket later launches look for. Failing to leaves the
// desktop as it was before any of this: a second launch is a second tray.
func holdInstance() {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
		return
	}

	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		listener.Close()
		return
	}
	if err = os.WriteFile(instanceMarker(), []byte(strconv.Itoa(address.Port)), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop:", err)
		listener.Close()
		return
	}

	go serveInstance(listener)
}

func releaseInstance() {
	_ = os.Remove(instanceMarker())
}

func serveInstance(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go serveInstanceRequest(conn)
	}
}

func serveInstanceRequest(conn net.Conn) {
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(instanceWait))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != showRequest {
		return
	}

	// Answering first keeps the launch that is waiting on this from outliving
	// the window it asked for.
	_, _ = io.WriteString(conn, showReply+"\n")
	showWindow()
}
