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
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/apache/casbin-gateway/agent"
)

func TestConfirmRemoved(t *testing.T) {
	path := filepath.Join(rooted("home", "a", "npm-global"),
		"node_modules", "@google", "gemini-cli")
	elsewhere := filepath.Join(rooted("opt", "npm"),
		"node_modules", "@google", "gemini-cli")
	found := func(paths ...string) []agent.Installation {
		result := make([]agent.Installation, 0, len(paths))
		for _, each := range paths {
			result = append(result, agent.Installation{AgentId: "gemini-cli", Path: each})
		}
		return result
	}

	cases := []struct {
		name          string
		plan          Plan
		installations []agent.Installation
		scanErr       error
		wantErr       bool
	}{
		{"agent gone",
			Plan{AgentId: "gemini-cli", Action: ActionUninstall, removedPath: path},
			found(elsewhere), nil, false},
		{"agent still where it was",
			Plan{AgentId: "gemini-cli", Action: ActionUninstall, removedPath: path},
			found(path), nil, true},
		{"another agent at the same path",
			Plan{AgentId: "qwen-code", Action: ActionUninstall, removedPath: path},
			found(path), nil, false},
		{"scan that could not say",
			Plan{AgentId: "gemini-cli", Action: ActionUninstall, removedPath: path},
			nil, errors.New("agent scan timed out"), true},
		{"install rather than uninstall",
			Plan{AgentId: "gemini-cli", Action: ActionInstall, removedPath: path},
			found(path), nil, false},
		{"uninstall with nothing to take off disk",
			Plan{AgentId: "gemini-cli", Action: ActionUninstall},
			found(path), errors.New("agent scan timed out"), false},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := confirmRemoved(test.plan, test.installations, test.scanErr)
			if (err != nil) != test.wantErr {
				t.Fatalf("confirmRemoved() error = %v, want error %v", err, test.wantErr)
			}
			if err == nil {
				return
			}
			// A failure is only useful if it says which of the two it was.
			if !strings.Contains(err.Error(), test.plan.removedPath) &&
				!strings.Contains(err.Error(), "could not check") {
				t.Errorf("error = %q, want it to name %q", err, test.plan.removedPath)
			}
		})
	}
}

// Windows tells no two paths apart by their case, and a scan reports whatever
// case the directory it walked was written in.
func TestConfirmRemovedIgnoresCaseOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("paths are case-sensitive here")
	}

	path := filepath.Join(rooted("home", "a", "npm-global"),
		"node_modules", "@google", "gemini-cli")
	plan := Plan{AgentId: "gemini-cli", Action: ActionUninstall, removedPath: path}
	installations := []agent.Installation{{AgentId: "gemini-cli", Path: strings.ToUpper(path)}}

	if err := confirmRemoved(plan, installations, nil); err == nil {
		t.Error("confirmRemoved() = nil, want an error for the agent still installed")
	}
}
