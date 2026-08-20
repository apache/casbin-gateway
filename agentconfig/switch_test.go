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

package agentconfig

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeAdapter renders "provider=<id>\n" into a fixed path, so Switch can be
// exercised without resolving a real account home.
type fakeAdapter struct{ path string }

func (fakeAdapter) AgentID() string                      { return "fake" }
func (f fakeAdapter) ConfigPath(Install) (string, error) { return f.path, nil }
func (fakeAdapter) DefaultMode() os.FileMode             { return 0o600 }
func (fakeAdapter) Render(_ []byte, p Provider) ([]byte, error) {
	return []byte("provider=" + p.ID + "\n"), nil
}

func registerFake(t *testing.T, path string) {
	t.Helper()
	adapters["fake"] = fakeAdapter{path: path}
	t.Cleanup(func() { delete(adapters, "fake") })
}

func TestSwitchWritesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	registerFake(t, path)

	res, err := Switch(Install{AgentID: "fake"}, Provider{ID: "alpha"})
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if !res.Changed {
		t.Error("Changed = false, want true on first write")
	}
	if got, _ := os.ReadFile(path); string(got) != "provider=alpha\n" {
		t.Errorf("config = %q", got)
	}
}

func TestSwitchIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	registerFake(t, path)

	if _, err := Switch(Install{AgentID: "fake"}, Provider{ID: "alpha"}); err != nil {
		t.Fatalf("first Switch: %v", err)
	}
	res, err := Switch(Install{AgentID: "fake"}, Provider{ID: "alpha"})
	if err != nil {
		t.Fatalf("second Switch: %v", err)
	}
	if res.Changed {
		t.Error("Changed = true, want false when the provider already matches")
	}
}

func TestSwitchBacksUpUserOriginalOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	registerFake(t, path)
	if err := os.WriteFile(path, []byte("user original\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := Switch(Install{AgentID: "fake"}, Provider{ID: "alpha"})
	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if !res.BackedUp {
		t.Error("BackedUp = false, want true when replacing a user file")
	}
	backup := path + backupSuffix
	if got, _ := os.ReadFile(backup); string(got) != "user original\n" {
		t.Errorf("backup = %q, want the pre-Gateway original", got)
	}

	// A second switch must not overwrite the original backup with Gateway's own
	// content.
	if _, err := Switch(Install{AgentID: "fake"}, Provider{ID: "beta"}); err != nil {
		t.Fatalf("second Switch: %v", err)
	}
	if got, _ := os.ReadFile(backup); string(got) != "user original\n" {
		t.Errorf("backup after second switch = %q, want unchanged original", got)
	}
}

func TestSwitchUnknownAgent(t *testing.T) {
	if _, err := Switch(Install{AgentID: "nope"}, Provider{ID: "alpha"}); err == nil {
		t.Error("expected an error for an unknown agent")
	}
}
