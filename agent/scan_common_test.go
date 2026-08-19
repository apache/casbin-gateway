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
	"os"
	"path/filepath"
	"testing"
)

func TestNpmrcPrefix(t *testing.T) {
	t.Setenv("CASBIN_NPMRC_TEST_HOME", `D:\develop`)

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "plain", content: "prefix=D:\\develop\\npmPackage", want: `D:\develop\npmPackage`},
		{name: "spaces and quotes", content: `prefix = "D:\develop\npmPackage"`, want: `D:\develop\npmPackage`},
		{name: "ignores comments and other keys", content: "; comment\nregistry=https://example.com\nprefix=/opt/npm\n", want: "/opt/npm"},
		{name: "env expansion", content: "prefix=${CASBIN_NPMRC_TEST_HOME}\\npmPackage", want: `D:\develop\npmPackage`},
		{name: "no prefix set", content: "registry=https://example.com\n", want: ""},
		{name: "empty file", content: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".npmrc")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatalf("write npmrc: %v", err)
			}
			if got := npmrcPrefix(path); got != test.want {
				t.Errorf("npmrcPrefix() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestNpmrcPrefixMissingFile(t *testing.T) {
	if got := npmrcPrefix(filepath.Join(t.TempDir(), "does-not-exist")); got != "" {
		t.Errorf("npmrcPrefix() = %q, want empty for missing file", got)
	}
}
