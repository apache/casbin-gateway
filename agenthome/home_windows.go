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

package agenthome

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// systemAccount is what the Windows scan stamps on an installation found in a
// machine-wide location; it is a marker, not an account that runs an agent.
const systemAccount = "SYSTEM"

// machineWideOwner reports whether an owner names the local system account.
func machineWideOwner(owner string) bool {
	return strings.EqualFold(accountName(owner), systemAccount)
}

// expandHome resolves the "%SystemRoot%" that os/user hands back unexpanded for
// Windows built-in accounts.
func expandHome(home string) string {
	if !strings.Contains(home, "%") {
		return home
	}
	if expanded, err := registry.ExpandString(home); err == nil && expanded != "" {
		return expanded
	}
	return home
}
