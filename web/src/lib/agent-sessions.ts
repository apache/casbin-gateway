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

import type {AgentSession} from "@/types";

// The backend only reports a resumeId that is a UUID, so it is safe to paste
// into a shell; the check here keeps it that way if that ever changes.
const uuid = /^[\da-f]{8}(-[\da-f]{4}){3}-[\da-f]{12}$/i;

export function sessionResumeCommand({agent, resumeId}: AgentSession): string {
  if (!resumeId || !uuid.test(resumeId)) {
    return "";
  }

  switch (agent) {
  case "claude-code":
    return `claude --resume ${resumeId}`;
  case "codex-cli":
    return `codex resume ${resumeId}`;
  default:
    return "";
  }
}
