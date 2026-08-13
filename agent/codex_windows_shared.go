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

import "strings"

func expandSharedCodexWindowsInstallations(installations []Installation, homes []homeDir) []Installation {
	var result []Installation
	for _, installation := range installations {
		if (installation.AgentId != "codex" && installation.AgentId != "codex-cli") ||
			!strings.EqualFold(installation.Owner, "SYSTEM") {
			result = append(result, installation)
			continue
		}
		matchedProfile := false
		for _, home := range homes {
			normalized := strings.ToLower(strings.ReplaceAll(home.path, `/`, `\`))
			if !strings.Contains(normalized, `\users\`) {
				continue
			}
			row := installation
			row.Owner = home.owner
			result = append(result, row)
			matchedProfile = true
		}
		if !matchedProfile {
			result = append(result, installation)
		}
	}
	return result
}
