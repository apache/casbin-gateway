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

package agentpatch

import "github.com/apache/casbin-gateway/agenthook"

// Qwen Code is a Gemini CLI fork: the same settings file under its own
// directory, with Claude Code's event names in it.
func init() {
	register(settingsHookPatcher{
		agentId: "qwen-code",
		name:    "Qwen Code",
		dir:     ".qwen",
		events:  agenthook.QwenEvents,
	})
}
