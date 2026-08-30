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

package util

import (
	"context"
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

const (
	// portLookupTimeout bounds the external "who owns this port" commands so a
	// slow lsof or netstat can never hold up startup.
	portLookupTimeout = 2 * time.Second
	// procNameMaxLen is how much of another process's name Linux reports, from
	// TASK_COMM_LEN. Anything longer comes back cut to this.
	procNameMaxLen = 15
	// ExitCodeFatalConfig marks an exit caused by configuration, such as a port
	// that is already taken. Restarting cannot fix it, so a service manager has
	// to be told not to try; the systemd unit the installer writes says so. The
	// value is the conventional sysexits.h EX_CONFIG.
	ExitCodeFatalConfig = 78
)

// ListenTcp binds a TCP listener on all interfaces for the given port. Callers
// keep the listener and serve on it, so a port that was reported free cannot be
// taken by someone else in between the check and the bind.
func ListenTcp(port int) (net.Listener, error) {
	return ListenTcpAddr("", port)
}

// ListenTcpAddr binds one interface rather than all of them. An empty addr
// means every interface.
func ListenTcpAddr(addr string, port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("%s:%d", addr, port))
}

// CheckPortAvailable binds the port and immediately releases it. Use it only
// where the port has to be handed to a server that insists on binding itself,
// such as beego.Run().
func CheckPortAvailable(port int) error {
	return CheckPortAvailableOn("", port)
}

// CheckPortAvailableOn is CheckPortAvailable for one interface.
func CheckPortAvailableOn(addr string, port int) error {
	listener, err := ListenTcpAddr(addr, port)
	if err != nil {
		return err
	}

	return listener.Close()
}

// FatalListenError explains a failed bind in plain language and stops the
// process without a stack trace. It exits with ExitCodeFatalConfig so that a
// service manager knows this is a configuration problem and does not restart
// the process straight back into the same conflict.
//
// "remedy" completes the sentence "Free the port, or ...", for example
// `change "httpport" in conf/app.conf`.
func FatalListenError(port int, remedy string, err error) {
	fmt.Println()
	if holder := DescribePortHolder(port); holder != "" {
		fmt.Printf("Casbin Gateway: port %d is in use (%s). Free it, or %s.\n", port, holder, remedy)
	} else {
		fmt.Printf("Casbin Gateway: cannot listen on port %d: %v\n", port, err)
		fmt.Printf("  Free the port, or %s.\n", remedy)
	}

	if port < 1024 && runtime.GOOS != "windows" {
		fmt.Printf("  Ports below 1024 also need root, so \"sudo\" may be required.\n")
	}
	fmt.Println()

	os.Exit(ExitCodeFatalConfig)
}

// PortHolder is the process listening on a port: its pid, its executable name
// when that could be read, and whether that executable is this same program.
type PortHolder struct {
	Pid  int
	Name string
	Ours bool
}

// String renders a holder as "pid 1234, nginx", or "pid 1234" when its name
// could not be read.
func (h *PortHolder) String() string {
	if h.Name == "" {
		return fmt.Sprintf("pid %d", h.Pid)
	}

	return fmt.Sprintf("pid %d, %s", h.Pid, h.Name)
}

// LookupPortHolder is a best-effort, read-only lookup of the process listening
// on the port. It turns "bind failed" into something actionable such as
// "pid 1234, nginx", and tells a previous Gateway apart from an unrelated
// program. It returns nil when nothing is listening or the pid cannot be read.
func LookupPortHolder(port int) *PortHolder {
	pid := findListenerPid(port)
	if pid <= 0 {
		return nil
	}

	name := findProcessName(pid)
	return &PortHolder{Pid: pid, Name: name, Ours: isSelfExecutable(name)}
}

// DescribePortHolder names the process listening on the port, or "" when there
// is none to name. It only ever reads: a wrong guess costs an inaccurate hint,
// never someone else's process.
func DescribePortHolder(port int) string {
	holder := LookupPortHolder(port)
	if holder == nil {
		return ""
	}

	return holder.String()
}

// isSelfExecutable reports whether an executable name is this program's own.
// It is the check that keeps Gateway from stopping a process that is not a
// previous Gateway, so it is deliberately conservative: a name that could not
// be read is never a match.
func isSelfExecutable(name string) bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}

	mine, other := executableBaseName(self), executableBaseName(name)
	if mine == "" || other == "" {
		return false
	}

	// Linux reports the name of another process cut to procNameMaxLen, so a
	// name that long has to be compared as a prefix of this executable's.
	if len(other) >= procNameMaxLen {
		return strings.HasPrefix(mine, other)
	}

	return mine == other
}

// executableBaseName drops the directory and the ".exe" suffix and lowercases
// the rest, so that "C:\Program Files\Casbin-Gateway.exe" and
// "casbin-gateway" are recognised as the same program.
func executableBaseName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	return strings.TrimSuffix(strings.ToLower(filepath.Base(value)), ".exe")
}

// findListenerPid returns the pid of a process listening on the port, or 0 when
// it cannot be determined. Only sockets in the LISTEN state are considered, so
// an unrelated process that merely holds a connection to a remote port with the
// same number is never reported.
func findListenerPid(port int) int {
	var output string
	if runtime.GOOS == "windows" {
		output = runLookup("netstat", "-ano", "-p", "TCP")
		return parseNetstatPid(output, port)
	}

	output = runLookup("lsof", "-sTCP:LISTEN", "-t", "-i", "TCP:"+strconv.Itoa(port))
	for _, line := range strings.Split(output, "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			return pid
		}
	}

	return 0
}

// parseNetstatPid picks the pid of the LISTENING row for the port out of
// "netstat -ano" output. Rows in other states (ESTABLISHED, TIME_WAIT) are
// skipped on purpose.
func parseNetstatPid(output string, port int) int {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || !strings.EqualFold(fields[0], "TCP") || !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}

		_, localPort, err := net.SplitHostPort(fields[1])
		if err != nil || localPort != strconv.Itoa(port) {
			continue
		}

		if pid, err := strconv.Atoi(fields[4]); err == nil {
			return pid
		}
	}

	return 0
}

// findProcessName resolves a pid to an executable name, returning "" when the
// platform command is missing or says nothing useful.
func findProcessName(pid int) string {
	if runtime.GOOS == "windows" {
		output := runLookup("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV")
		// A CSV row looks like: "nginx.exe","1234","Console","1","5,000 K"
		name := strings.TrimSpace(strings.SplitN(output, ",", 2)[0])
		return strings.Trim(name, `"`)
	}

	return strings.TrimSpace(runLookup("ps", "-p", strconv.Itoa(pid), "-o", "comm="))
}

// runLookup runs a short diagnostic command and returns its stdout, or "" on
// any failure. Every caller treats the result as a hint, so errors are not
// worth surfacing.
func runLookup(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), portLookupTimeout)
	defer cancel()

	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}

	return string(output)
}
