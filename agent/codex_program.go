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

package agent

import (
	"path/filepath"
	"strings"
)

// CodexProgram is the Codex command line to run on behalf of one installation,
// empty when this host has none. The desktop app ships no command line of its
// own, so the CLI installed beside it answers for both.
func CodexProgram(installation Installation) string {
	if program := codexProgramOf(installation); program != "" {
		return program
	}

	installations, err := Scan(false)
	if err != nil {
		return ""
	}
	for _, other := range installations {
		if other.AgentId != "codex" && other.AgentId != "codex-cli" {
			continue
		}
		if program := codexProgramOf(other); program != "" {
			return program
		}
	}
	return ""
}

func codexProgramOf(installation Installation) string {
	executable := LaunchOf(installation).Executable
	if executable == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(filepath.Base(executable)), "codex") {
		return executable
	}
	return ""
}
