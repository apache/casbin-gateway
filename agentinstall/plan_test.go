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

package agentinstall

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/apache/casbin-gateway/agent"
)

// rooted builds an absolute path out of segments, which is what a scan reports
// and what a prefix is derived from. Drive letters are no part of that.
func rooted(segments ...string) string {
	return filepath.Join(append([]string{string(filepath.Separator)}, segments...)...)
}

func TestNpmPrefixOf(t *testing.T) {
	prefix := rooted("home", "a", "npm-global")
	libPrefix := rooted("usr", "local")
	// Off Windows the prefix keeps its global packages under "lib"; on Windows
	// it keeps them directly.
	libWant := libPrefix
	if runtime.GOOS == "windows" {
		libWant = filepath.Join(libPrefix, "lib")
	}

	cases := []struct {
		name   string
		method string
		path   string
		pkg    string
		want   string
	}{
		{"scoped package",
			ManagerNpm, filepath.Join(prefix, "node_modules", "@google", "gemini-cli"),
			"@google/gemini-cli", prefix},
		{"unscoped package",
			ManagerNpm, filepath.Join(prefix, "node_modules", "opencode-ai"),
			"opencode-ai", prefix},
		{"package under a lib layout",
			ManagerNpm, filepath.Join(libPrefix, "lib", "node_modules", "@google", "gemini-cli"),
			"@google/gemini-cli", libWant},
		{"installation npm does not own",
			"native", filepath.Join(prefix, "node_modules", "@google", "gemini-cli"),
			"@google/gemini-cli", ""},
		{"installation with no path",
			ManagerNpm, "", "@google/gemini-cli", ""},
		{"path that is not inside node_modules",
			ManagerNpm, rooted("tools", "gemini", "gemini-cli"), "@google/gemini-cli", ""},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			installation := agent.Installation{InstallMethod: test.method, Path: test.path}
			if got := npmPrefixOf(installation, test.pkg); got != test.want {
				t.Errorf("npmPrefixOf() = %q, want %q", got, test.want)
			}
		})
	}
}

// An uninstall has to name the tree the scan found the agent in: npm otherwise
// acts on the prefix its own configuration names, which on a host with more
// than one is not where the agent is, and it exits zero having removed nothing.
func TestUninstallPlanNamesTheFoundPrefix(t *testing.T) {
	if lookup("npm") == "" {
		t.Skip("npm is not on PATH")
	}

	prefix := rooted("home", "a", "npm-global")
	path := filepath.Join(prefix, "node_modules", "@google", "gemini-cli")
	plan := UninstallPlan(agent.Installation{
		AgentId: "gemini-cli", InstallMethod: ManagerNpm, Path: path,
	})

	if !plan.Available || plan.Manager != ManagerNpm {
		t.Fatalf("plan = %+v, want an available npm plan", plan)
	}
	if !strings.Contains(plan.Command, "--prefix "+prefix) {
		t.Errorf("command = %q, want it to name prefix %q", plan.Command, prefix)
	}
	if plan.removedPath != path {
		t.Errorf("removedPath = %q, want %q", plan.removedPath, path)
	}
}

// Nothing is removed for an agent found by its configuration alone, so there is
// nothing for a finished job to confirm either.
func TestUninstallPlanForConfigOnlyInstallation(t *testing.T) {
	plan := UninstallPlan(agent.Installation{
		AgentId:       "gemini-cli",
		InstallMethod: agent.InstallMethodConfig,
		Path:          rooted("home", "a", ".gemini"),
	})

	if plan.Available {
		t.Error("plan.Available = true, want false for a configuration directory")
	}
	if plan.removedPath != "" {
		t.Errorf("removedPath = %q, want empty", plan.removedPath)
	}
}
