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
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// netstatSample mixes the rows that matter with the ones that have to be
// ignored: a connection to a remote port 80, a listener on a port that merely
// ends in "80", and an IPv6 listener on the port being looked up.
const netstatSample = `
Active Connections

  Proto  Local Address          Foreign Address        State           PID
  TCP    0.0.0.0:8080           0.0.0.0:0              LISTENING       1111
  TCP    192.168.1.5:52341      93.184.216.34:80       ESTABLISHED     2222
  TCP    0.0.0.0:80             0.0.0.0:0              LISTENING       3333
  TCP    [::]:443               [::]:0                 LISTENING       4444
  TCP    127.0.0.1:9000         0.0.0.0:0              TIME_WAIT       5555
`

func TestParseNetstatPid(t *testing.T) {
	tests := []struct {
		name string
		port int
		want int
	}{
		{"listener on the port", 80, 3333},
		{"ipv6 listener", 443, 4444},
		{"port that only shares a suffix", 8080, 1111},
		{"remote port is not a listener", 25, 0},
		{"non-listening state is skipped", 9000, 0},
		{"nothing on the port", 1234, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseNetstatPid(netstatSample, tt.port)
			if got != tt.want {
				t.Errorf("parseNetstatPid(port %d) = %d, want %d", tt.port, got, tt.want)
			}
		})
	}
}

// TestDescribePortHolder checks the whole lookup against a port this test is
// holding itself. The platform commands it shells out to are not guaranteed to
// exist, so an empty answer is tolerated: the description is only ever a hint.
func TestDescribePortHolder(t *testing.T) {
	listener, err := ListenTcp(0)
	if err != nil {
		t.Fatalf("ListenTcp() error: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	description := DescribePortHolder(port)
	if description == "" {
		t.Skipf("no port lookup available on this host, skipping")
	}

	if !strings.HasPrefix(description, "pid ") {
		t.Errorf("DescribePortHolder(%d) = %q, want it to start with \"pid \"", port, description)
	}
}

// TestFatalListenError runs the fatal path in a subprocess, because it ends the
// process on purpose. Both halves matter: the message has to name the port, and
// the exit code has to be the one a service manager treats as "do not restart".
func TestFatalListenError(t *testing.T) {
	if os.Getenv("CASBIN_GATEWAY_TEST_FATAL_LISTEN") == "1" {
		listener, err := ListenTcp(0)
		if err != nil {
			t.Fatalf("ListenTcp() error: %v", err)
		}
		defer listener.Close()

		port := listener.Addr().(*net.TCPAddr).Port
		_, err = ListenTcp(port)
		if err == nil {
			t.Fatalf("ListenTcp(%d) succeeded twice, expected the port to be held", port)
		}

		FatalListenError(port, `change "httpport" in conf/app.conf`, err)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFatalListenError")
	cmd.Env = append(os.Environ(), "CASBIN_GATEWAY_TEST_FATAL_LISTEN=1")
	output, err := cmd.CombinedOutput()

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("FatalListenError() ended with %v, want a non-zero exit", err)
	}
	if exitErr.ExitCode() != ExitCodeFatalConfig {
		t.Errorf("FatalListenError() exit code = %d, want %d", exitErr.ExitCode(), ExitCodeFatalConfig)
	}

	if !strings.Contains(string(output), "is in use") && !strings.Contains(string(output), "cannot listen on port") {
		t.Errorf("FatalListenError() output = %q, want it to explain the port conflict", output)
	}
	if !strings.Contains(string(output), "httpport") {
		t.Errorf("FatalListenError() output = %q, want it to carry the remedy", output)
	}
}

func TestCheckPortAvailable(t *testing.T) {
	listener, err := ListenTcp(0)
	if err != nil {
		t.Fatalf("ListenTcp() error: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	if err = CheckPortAvailable(port); err == nil {
		t.Errorf("CheckPortAvailable(%d) = nil while the port is held, want an error", port)
	}

	listener.Close()
	if err = CheckPortAvailable(port); err != nil {
		t.Errorf("CheckPortAvailable(%d) error after release: %v", port, err)
	}
}
