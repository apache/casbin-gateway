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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// defaultHttpPort mirrors conf.GetHttpPort(): the launcher reads the same
// app.conf, and falls back to the same number when there is none to read.
const defaultHttpPort = 17000

const (
	homeEnvKey    = "CASBIN_GATEWAY_HOME"
	urlEnvKey     = "CASBIN_GATEWAY_URL"
	debugEnvKey   = "CASBIN_GATEWAY_DEBUG"
	serverName    = "casbin-gateway"
	startupWait   = 40 * time.Second
	startupPoll   = 200 * time.Millisecond
	probeInterval = 3 * time.Second
)

// gatewayHome is the directory holding the server executable and its data.
// Gateway keeps its database, logs and tmp files in its working directory, so
// everything the launcher starts has to start there.
func gatewayHome() string {
	if dir := os.Getenv(homeEnvKey); dir != "" {
		return dir
	}

	executable, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	dir := filepath.Dir(executable)

	// Inside a macOS .app the executable sits in Contents/MacOS, which is not
	// where the data lives. The installer records the real directory next to it
	// rather than wrapping the binary in a script, because a wrapper that execs
	// outside the bundle loses the Dock icon and the app name with it.
	if recorded := readBundleHome(dir); recorded != "" {
		return recorded
	}
	return dir
}

func readBundleHome(macOSDir string) string {
	if runtime.GOOS != "darwin" || filepath.Base(macOSDir) != "MacOS" {
		return ""
	}

	data, err := os.ReadFile(filepath.Join(filepath.Dir(macOSDir), "Resources", "home"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func serverPath() string {
	name := serverName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(gatewayHome(), name)
}

// httpPort is the port the server will bind. Only the keys read before the
// database opens come from app.conf, and "httpport" is one of them — but an
// environment variable of the same name wins over the file there, so it has to
// win here too, or the launcher waits on a port nothing is serving.
func httpPort() int {
	if port, err := strconv.Atoi(strings.Trim(os.Getenv("httpport"), `"' `)); err == nil && port > 0 {
		return port
	}

	file, err := os.Open(filepath.Join(gatewayHome(), "conf", "app.conf"))
	if err != nil {
		return defaultHttpPort
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found || strings.TrimSpace(key) != "httpport" {
			continue
		}
		if port, err := strconv.Atoi(strings.Trim(strings.TrimSpace(value), `"' `)); err == nil && port > 0 {
			return port
		}
	}
	return defaultHttpPort
}

// gatewayUrl is what the window shows. It is the server's own address, unless
// something points the window elsewhere — during frontend work that is the Vite
// dev server, which serves the UI from source and proxies the API back to the
// server this launcher started.
func gatewayUrl() string {
	if url := strings.TrimSpace(os.Getenv(urlEnvKey)); url != "" {
		return url
	}
	return fmt.Sprintf("http://127.0.0.1:%d", httpPort())
}

// debugEnabled turns on the webview's own context menu and developer tools,
// which are off by default because a desktop window has no address bar to
// recover from whatever they lead to.
func debugEnabled() bool {
	return os.Getenv(debugEnvKey) != ""
}

// isServing reports whether something answers on the port, which is all a
// process that did not start the server can tell.
func isServing(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startServer runs "casbin-gateway start", which detaches a copy of the server
// and returns once it answers. It reports whether this call is what started it,
// so that quitting only stops a server the launcher owns.
func startServer(port int) (started bool, err error) {
	if isServing(port) {
		return false, nil
	}

	cmd := exec.Command(serverPath(), "start")
	cmd.Dir = gatewayHome()
	hideConsole(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("could not start the Gateway server: %v: %s", err, strings.TrimSpace(string(output)))
	}

	deadline := time.Now().Add(startupWait)
	for time.Now().Before(deadline) {
		if isServing(port) {
			return true, nil
		}
		time.Sleep(startupPoll)
	}
	return false, fmt.Errorf("the Gateway server did not answer on port %d", port)
}

func stopServer() error {
	cmd := exec.Command(serverPath(), "stop")
	cmd.Dir = gatewayHome()
	hideConsole(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// quitRunning stops the whole of a Gateway that is running out of this home:
// the tray and its window through the socket they listen on, and the server
// behind them whether or not the tray was what started it. It reports whether
// there was anything to stop.
//
// This is what an installer calls before it replaces the executables. Windows
// will not write over one that is running, and everywhere else an install that
// left the old process serving would look like an update that did nothing.
func quitRunning() bool {
	port := instancePort()
	stopped := askInstance(quitRequest)
	if stopped {
		waitForInstanceGone(port)
	}

	if !isServing(httpPort()) {
		return stopped
	}
	if err := stopServer(); err != nil {
		fmt.Fprintln(os.Stderr, "casbin-gateway-desktop: could not stop the Gateway server:", err)
		return stopped
	}
	return true
}
