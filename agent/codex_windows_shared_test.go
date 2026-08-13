// Copyright 2025 The casbin Authors. All Rights Reserved.
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

package agent

import "testing"

func TestExpandSharedCodexWindowsInstallations(t *testing.T) {
	installation := Installation{AgentId: "codex", Path: `C:\Program Files\WindowsApps\Codex.exe`, Owner: "SYSTEM"}
	tests := []struct {
		name  string
		homes []homeDir
		want  []string
	}{
		{name: "no profiles", want: []string{"SYSTEM"}},
		{
			name:  "no user profiles",
			homes: []homeDir{{owner: "SYSTEM", path: `C:\Windows\System32\config\systemprofile`}},
			want:  []string{"SYSTEM"},
		},
		{
			name: "user profiles",
			homes: []homeDir{
				{owner: "SYSTEM", path: `C:\Windows\System32\config\systemprofile`},
				{owner: "alice", path: `C:\Users\alice`},
				{owner: "bob", path: `D:\Users\bob`},
			},
			want: []string{"alice", "bob"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := expandSharedCodexWindowsInstallations([]Installation{installation}, test.homes)
			if len(got) != len(test.want) {
				t.Fatalf("got %d installations, want %d", len(got), len(test.want))
			}
			for i, owner := range test.want {
				if got[i].Owner != owner {
					t.Errorf("installation %d owner = %q, want %q", i, got[i].Owner, owner)
				}
			}
		})
	}
}
