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

//go:build !windows

package agentmonitor

import "errors"

func startCoworkMonitor() error { return nil }

func stopCoworkMonitor() {}

// EnableCoworkMonitor enables Windows Cowork transcript monitoring for one
// Claude Desktop installation. It is unavailable on non-Windows hosts.
func EnableCoworkMonitor(string, string) error {
	return errors.New("Cowork monitoring is only supported on Windows")
}

// DisableCoworkMonitor removes a Windows Cowork transcript declaration.
func DisableCoworkMonitor(string) error { return nil }

// CoworkMonitorStatus reports that Cowork transcript monitoring is Windows-only.
func CoworkMonitorStatus(string) (bool, string) {
	return false, "Cowork monitoring is only supported on Windows"
}
