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

package version

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

	"github.com/beego/beego"
)

// The tray listens on a loopback socket and leaves its port in a marker file.
// These mirror its own constants: it is a separate module.
const (
	desktopMarkerName  = ".desktop-instance"
	desktopRestartWord = "restart"
	desktopOkReply     = "ok"
	desktopDialTimeout = 2 * time.Second

	serveWait = 90 * time.Second
	servePoll = 200 * time.Millisecond
)

// RestartDesktopLauncher tells the tray still running the build this update
// replaced to start the launcher that came with it. Without it the server is
// the new version and the window in front of the reader is not.
func RestartDesktopLauncher(port int) {
	go func() {
		// The launcher opens its window on this Gateway, so it is only told
		// once there is something to open it on.
		if !waitUntilServing(port) {
			return
		}
		if err := askDesktop(desktopRestartWord); err != nil {
			beego.Error("the desktop launcher was left at the previous version:", err)
		}
	}()
}

func waitUntilServing(port int) bool {
	deadline := time.Now().Add(serveWait)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), desktopDialTimeout)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(servePoll)
	}
	return false
}

// askDesktop sends one request to the tray. No desktop, or a marker that
// outlived the tray which wrote it, is nothing to tell rather than a failure.
func askDesktop(request string) error {
	port := desktopInstancePort()
	if port == 0 {
		return nil
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), desktopDialTimeout)
	if err != nil {
		return nil
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(desktopDialTimeout))
	if _, err := io.WriteString(conn, request+"\n"); err != nil {
		return err
	}

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(reply) != desktopOkReply {
		return fmt.Errorf("the desktop launcher answered %q", strings.TrimSpace(reply))
	}

	return nil
}

// desktopInstancePort reads the marker the tray left next to the installation,
// falling back to the working directory.
func desktopInstancePort() int {
	var dirs []string
	if executable, err := executablePath(); err == nil {
		dirs = append(dirs, filepath.Dir(executable))
	}
	if workingDir, err := os.Getwd(); err == nil {
		dirs = append(dirs, workingDir)
	}

	for _, dir := range dirs {
		content, err := os.ReadFile(filepath.Join(dir, desktopMarkerName))
		if err != nil {
			continue
		}
		if port, err := strconv.Atoi(strings.TrimSpace(string(content))); err == nil && port > 0 {
			return port
		}
	}

	return 0
}
