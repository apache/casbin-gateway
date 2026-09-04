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

import * as React from "react";

const agentSites: Record<string, string> = {
  "Claude Code": "claude.com",
  "Claude Desktop": "claude.ai",
  OpenClaw: "openclaw.ai",
  Codex: "openai.com",
  "ChatGPT Desktop (Codex)": "openai.com",
  "Codex CLI": "openai.com",
  "Hermes Agent": "hermes-agent.nousresearch.com",
  Cursor: "cursor.com",
  "Cursor Agent": "cursor.com",
  Windsurf: "windsurf.com",
  OpenAgent: "openagentai.org",
  opencode: "opencode.ai",
  "opencode Desktop": "opencode.ai",
  "DeepSeek Harness": "deepseek.com",
  "Gemini CLI": "gemini.google.com",
  "Qwen Code": "qwen.ai",
  "iFlow CLI": "iflow.cn",
  Cline: "cline.bot",
  "Roo Code": "roocode.com",
  "Copilot CLI": "github.com",
};

const agentKey = (agent: unknown) =>
  String(agent || "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "");

const agentSitesByKey = Object.keys(agentSites).reduce<Record<string, string>>((sites, name) => {
  sites[agentKey(name)] = agentSites[name];
  return sites;
}, {});

export function AgentIcon({
  agent,
  fallback = null,
  size = 20,
}: {
  agent: string | undefined;
  fallback?: React.ReactNode;
  size?: number;
}) {
  const [broken, setBroken] = React.useState(false);
  const site = agentSitesByKey[agentKey(agent)];

  if (!site || broken) {
    return <>{fallback}</>;
  }

  return (
    <img
      src={`https://www.google.com/s2/favicons?domain=${site}&sz=64`}
      alt={agent}
      width={size}
      height={size}
      onError={() => setBroken(true)}
      className="block rounded"
    />
  );
}
