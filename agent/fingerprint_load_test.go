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

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestFingerprintsLoad(t *testing.T) {
	want := []string{
		"claude-code", "claude-desktop", "codex-cli", "codex", "cursor-agent",
		"cursor", "dsh", "gemini-cli", "hermes-agent", "openagent", "openclaw",
		"opencode-desktop", "opencode",
		"windsurf",
	}

	loaded, err := loadFingerprints(fingerprintFS)
	if err != nil {
		t.Fatalf("loadFingerprints() failed: %v", err)
	}
	if len(loaded) != len(want) {
		t.Fatalf("loaded %d fingerprints, want %d", len(loaded), len(want))
	}
	for i, id := range want {
		if loaded[i].ID != id {
			t.Errorf("fingerprint %d is %q, want %q", i, loaded[i].ID, id)
		}
	}
}

func TestLoadFingerprintsRejects(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
		want    string
	}{{
		name:    "malformed json",
		file:    "fingerprints/broken.json",
		content: `{"id": "broken",`,
		want:    "unexpected EOF",
	}, {
		name:    "misspelled field",
		file:    "fingerprints/typo.json",
		content: `{"id": "typo", "displayName": "Typo", "execName": "typo", "stateDir": "typo", "execNam": "typo"}`,
		want:    "unknown field",
	}, {
		name:    "legacy cmdline field",
		file:    "fingerprints/legacy-cmd.json",
		content: `{"id": "legacy-cmd", "displayName": "Legacy", "cmdMarkers": ["legacy"]}`,
		want:    "unknown field",
	}, {
		name:    "legacy executable rule field",
		file:    "fingerprints/legacy-rule.json",
		content: `{"id": "legacy-rule", "displayName": "Legacy", "extraExecRules": [{"suffix": "/legacy"}]}`,
		want:    "unknown field",
	}, {
		name:    "legacy notes field",
		file:    "fingerprints/legacy-notes.json",
		content: `{"id": "legacy-notes", "displayName": "Legacy", "notes": ["unused"]}`,
		want:    "unknown field",
	}, {
		name:    "id does not match file name",
		file:    "fingerprints/renamed.json",
		content: `{"id": "other", "displayName": "Other", "execName": "other", "stateDir": "other"}`,
		want:    "does not match the file name",
	}, {
		name:    "missing display name",
		file:    "fingerprints/nameless.json",
		content: `{"id": "nameless", "execName": "nameless", "stateDir": "nameless"}`,
		want:    "displayName is required",
	}, {
		name:    "no evidence at all",
		file:    "fingerprints/ghost.json",
		content: `{"id": "ghost", "displayName": "Ghost", "execName": "ghost"}`,
		want:    "no supported installation layout",
	}, {
		name:    "incomplete local server",
		file:    "fingerprints/server.json",
		content: `{"id": "server", "displayName": "Server", "localServer": {"ports": [1234]}}`,
		want:    "probePath and probeMarkers",
	}, {
		name:    "local server without ports",
		file:    "fingerprints/portless.json",
		content: `{"id": "portless", "displayName": "Portless", "localServer": {"probePath": "/health", "probeMarkers": ["x"]}}`,
		want:    "localServer.ports is required",
	}, {
		name:    "local server with empty marker",
		file:    "fingerprints/empty-marker.json",
		content: `{"id": "empty-marker", "displayName": "Empty Marker", "localServer": {"ports": [1234], "probePath": "/health", "probeMarkers": [""]}}`,
		want:    "confirms nothing",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := fstest.MapFS{test.file: &fstest.MapFile{Data: []byte(test.content)}}
			_, err := loadFingerprints(files)
			if err == nil {
				t.Fatalf("loadFingerprints() accepted %s", test.name)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("loadFingerprints() error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestLoadFingerprintsRejectsEmptyRegistry(t *testing.T) {
	_, err := loadFingerprints(fstest.MapFS{"other/claude-code.json": &fstest.MapFile{Data: []byte("{}")}})
	if err == nil || !strings.Contains(err.Error(), "no fingerprint files") {
		t.Errorf("loadFingerprints() on an empty registry = %v, want a no-fingerprint-files error", err)
	}
}
