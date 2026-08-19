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
	"fmt"
	"os"
	"time"
)

const (
	// portReleaseTimeout bounds the wait for a killed process to let go of its
	// port. The kill is asynchronous, so the port stays busy for a short while
	// after the process itself is gone.
	portReleaseTimeout = 3 * time.Second
	// portReleasePollInterval is how often the port is retried while waiting.
	portReleasePollInterval = 100 * time.Millisecond
)

// StopOldInstance kills whatever process is listening on the port, so that a
// restart never has to wait for the previous run to be shut down by hand.
//
// This takes the port for Casbin Gateway unconditionally: an unrelated program
// that happens to be listening there is killed just the same, so the ports in
// conf/app.conf have to be ones this machine really is allowed to hand over.
// Only sockets in the LISTEN state count, so a process that merely holds a
// connection to a remote port with the same number is never touched.
func StopOldInstance(port int) error {
	pid := findListenerPid(port)
	if pid <= 0 || pid == os.Getpid() {
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	name := findProcessName(pid)

	err = process.Kill()
	if err != nil {
		return err
	}

	if name == "" {
		fmt.Printf("Casbin Gateway: stopped pid %d, which was holding port %d\n", pid, port)
	} else {
		fmt.Printf("Casbin Gateway: stopped %s (pid %d), which was holding port %d\n", name, pid, port)
	}

	return waitForPortRelease(port)
}

// waitForPortRelease blocks until the port can be bound again, because a killed
// process releases it a moment after it disappears. It gives up quietly on
// timeout and leaves the caller's own bind to produce the error.
func waitForPortRelease(port int) error {
	deadline := time.Now().Add(portReleaseTimeout)
	for {
		if err := CheckPortAvailable(port); err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return nil
		}

		time.Sleep(portReleasePollInterval)
	}
}
