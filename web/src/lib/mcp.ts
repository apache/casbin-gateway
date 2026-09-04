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

import type {McpTransport} from "@/types";

/** One server as the Add dialog holds it, whichever way it was filled in. */
export interface McpDraft {
  name: string;
  transport: McpTransport;
  command: string;
  args: string[];
  env: Record<string, string>;
  url: string;
  headers: Record<string, string>;
}

export function emptyDraft(): McpDraft {
  return {name: "", transport: "stdio", command: "", args: [], env: {}, url: "", headers: {}};
}

/** What the catalogue groups servers by: the job they are installed to do. */
export const mcpCategories = ["essential", "dev", "data", "web", "work"] as const;
export type McpCategory = (typeof mcpCategories)[number];

/** The handful of things a preset asks for, each with its own label. */
export type McpInputKind = "key" | "token" | "path" | "url" | "id";

/** A value the server will not run without, substituted for {{key}} below. */
export interface McpPresetInput {
  key: string;
  kind: McpInputKind;
  placeholder?: string;
}

export interface McpPreset {
  key: string;
  label: string;
  category: McpCategory;
  transport: McpTransport;
  /** The vendor's own page, for the icon and for where a key comes from. */
  website: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  inputs?: McpPresetInput[];
}

/**
 * The servers offered by name instead of by command line. Every entry is the
 * install its own documentation gives, with the parts that differ per machine —
 * a folder, a key — left as {{...}} for the wizard to ask for.
 */
export const mcpPresets: McpPreset[] = [
  {
    key: "filesystem",
    label: "Filesystem",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-filesystem", "{{path}}"],
    inputs: [{key: "path", kind: "path", placeholder: "/Users/me/projects"}],
  },
  {
    key: "fetch",
    label: "Fetch",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/fetch",
    command: "uvx",
    args: ["mcp-server-fetch"],
  },
  {
    key: "memory",
    label: "Memory",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/memory",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-memory"],
  },
  {
    key: "sequential-thinking",
    label: "Sequential Thinking",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/sequentialthinking",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-sequential-thinking"],
  },
  {
    key: "time",
    label: "Time",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/time",
    command: "uvx",
    args: ["mcp-server-time"],
  },
  {
    key: "git",
    label: "Git",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/git",
    command: "uvx",
    args: ["mcp-server-git", "--repository", "{{path}}"],
    inputs: [{key: "path", kind: "path", placeholder: "/Users/me/projects/repo"}],
  },
  {
    key: "everything",
    label: "Everything",
    category: "essential",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers/tree/main/src/everything",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-everything"],
  },
  {
    key: "github",
    label: "GitHub",
    category: "dev",
    transport: "http",
    website: "https://github.com/github/github-mcp-server",
    url: "https://api.githubcopilot.com/mcp/",
    headers: {Authorization: "Bearer {{token}}"},
    inputs: [{key: "token", kind: "token", placeholder: "github_pat_..."}],
  },
  {
    key: "context7",
    label: "Context7",
    category: "dev",
    transport: "stdio",
    website: "https://context7.com",
    command: "npx",
    args: ["-y", "@upstash/context7-mcp"],
  },
  {
    key: "playwright",
    label: "Playwright",
    category: "dev",
    transport: "stdio",
    website: "https://github.com/microsoft/playwright-mcp",
    command: "npx",
    args: ["-y", "@playwright/mcp@latest"],
  },
  {
    key: "chrome-devtools",
    label: "Chrome DevTools",
    category: "dev",
    transport: "stdio",
    website: "https://github.com/ChromeDevTools/chrome-devtools-mcp",
    command: "npx",
    args: ["-y", "chrome-devtools-mcp@latest"],
  },
  {
    key: "deepwiki",
    label: "DeepWiki",
    category: "dev",
    transport: "http",
    website: "https://deepwiki.com",
    url: "https://mcp.deepwiki.com/mcp",
  },
  {
    key: "sentry",
    label: "Sentry",
    category: "dev",
    transport: "http",
    website: "https://docs.sentry.io/product/sentry-mcp/",
    url: "https://mcp.sentry.dev/mcp",
  },
  {
    key: "supabase",
    label: "Supabase",
    category: "dev",
    transport: "stdio",
    website: "https://supabase.com/docs/guides/getting-started/mcp",
    command: "npx",
    args: ["-y", "@supabase/mcp-server-supabase@latest"],
    env: {SUPABASE_ACCESS_TOKEN: "{{token}}"},
    inputs: [{key: "token", kind: "token", placeholder: "sbp_..."}],
  },
  {
    key: "figma",
    label: "Figma",
    category: "dev",
    transport: "stdio",
    website: "https://github.com/GLips/Figma-Context-MCP",
    command: "npx",
    args: ["-y", "figma-developer-mcp", "--stdio"],
    env: {FIGMA_API_KEY: "{{key}}"},
    inputs: [{key: "key", kind: "key", placeholder: "figd_..."}],
  },
  {
    key: "cloudflare-docs",
    label: "Cloudflare Docs",
    category: "dev",
    transport: "http",
    website: "https://developers.cloudflare.com/agents/model-context-protocol/",
    url: "https://docs.mcp.cloudflare.com/mcp",
  },
  {
    key: "sqlite",
    label: "SQLite",
    category: "data",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers-archived/tree/main/src/sqlite",
    command: "uvx",
    args: ["mcp-server-sqlite", "--db-path", "{{path}}"],
    inputs: [{key: "path", kind: "path", placeholder: "/Users/me/data.db"}],
  },
  {
    key: "postgres",
    label: "PostgreSQL",
    category: "data",
    transport: "stdio",
    website: "https://github.com/modelcontextprotocol/servers-archived/tree/main/src/postgres",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-postgres", "{{url}}"],
    inputs: [{key: "url", kind: "url", placeholder: "postgresql://localhost/mydb"}],
  },
  {
    key: "huggingface",
    label: "Hugging Face",
    category: "data",
    transport: "http",
    website: "https://huggingface.co/settings/mcp",
    url: "https://huggingface.co/mcp",
  },
  {
    key: "brave-search",
    label: "Brave Search",
    category: "web",
    transport: "stdio",
    website: "https://brave.com/search/api/",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-brave-search"],
    env: {BRAVE_API_KEY: "{{key}}"},
    inputs: [{key: "key", kind: "key", placeholder: "BSA..."}],
  },
  {
    key: "tavily",
    label: "Tavily",
    category: "web",
    transport: "stdio",
    website: "https://app.tavily.com",
    command: "npx",
    args: ["-y", "tavily-mcp"],
    env: {TAVILY_API_KEY: "{{key}}"},
    inputs: [{key: "key", kind: "key", placeholder: "tvly-..."}],
  },
  {
    key: "exa",
    label: "Exa",
    category: "web",
    transport: "http",
    website: "https://exa.ai",
    url: "https://mcp.exa.ai/mcp",
  },
  {
    key: "firecrawl",
    label: "Firecrawl",
    category: "web",
    transport: "stdio",
    website: "https://www.firecrawl.dev",
    command: "npx",
    args: ["-y", "firecrawl-mcp"],
    env: {FIRECRAWL_API_KEY: "{{key}}"},
    inputs: [{key: "key", kind: "key", placeholder: "fc-..."}],
  },
  {
    key: "notion",
    label: "Notion",
    category: "work",
    transport: "http",
    website: "https://developers.notion.com/docs/mcp",
    url: "https://mcp.notion.com/mcp",
  },
  {
    key: "linear",
    label: "Linear",
    category: "work",
    transport: "http",
    website: "https://linear.app/docs/mcp",
    url: "https://mcp.linear.app/mcp",
  },
  {
    key: "slack",
    label: "Slack",
    category: "work",
    transport: "stdio",
    website: "https://api.slack.com/apps",
    command: "npx",
    args: ["-y", "@modelcontextprotocol/server-slack"],
    env: {SLACK_BOT_TOKEN: "{{token}}", SLACK_TEAM_ID: "{{team}}"},
    inputs: [
      {key: "token", kind: "token", placeholder: "xoxb-..."},
      {key: "team", kind: "id", placeholder: "T01234567"},
    ],
  },
  {
    key: "stripe",
    label: "Stripe",
    category: "work",
    transport: "http",
    website: "https://docs.stripe.com/mcp",
    url: "https://mcp.stripe.com",
  },
];

function fill(text: string, values: Record<string, string>) {
  return text.replace(/\{\{(\w+)\}\}/g, (match, key: string) =>
    values[key] === undefined ? match : values[key].trim(),
  );
}

function fillPairs(pairs: Record<string, string> | undefined, values: Record<string, string>) {
  return Object.fromEntries(Object.entries(pairs ?? {}).map(([key, value]) => [key, fill(value, values)]));
}

/** The preset with what the wizard asked for put in, ready to be written. */
export function draftFromPreset(preset: McpPreset, values: Record<string, string>): McpDraft {
  return {
    name: preset.key,
    transport: preset.transport,
    command: fill(preset.command ?? "", values),
    args: (preset.args ?? []).map(arg => fill(arg, values)),
    env: fillPairs(preset.env, values),
    url: fill(preset.url ?? "", values),
    headers: fillPairs(preset.headers, values),
  };
}

export function presetFilled(preset: McpPreset, values: Record<string, string>) {
  return (preset.inputs ?? []).every(input => (values[input.key] ?? "").trim() !== "");
}

/** What a preset runs, for the line under its name in the catalogue. */
export function presetSummary(preset: McpPreset) {
  return preset.transport === "http" ? (preset.url ?? "") : [preset.command, ...(preset.args ?? [])].join(" ");
}

const serverMaps = ["mcpServers", "servers", "mcp_servers"];
const httpTypes = ["http", "streamable-http", "streamablehttp", "sse"];

function entryDraft(name: string, entry: Record<string, unknown>): McpDraft | null {
  const url = String(entry.url ?? entry.httpUrl ?? entry.serverUrl ?? "").trim();
  const command = String(entry.command ?? "").trim();
  const type = String(entry.type ?? entry.transport ?? "").toLowerCase();
  const http = httpTypes.includes(type) || (url !== "" && command === "");
  if (http ? url === "" : command === "") {
    return null;
  }

  const strings = (value: unknown) =>
    Object.fromEntries(
      Object.entries((value ?? {}) as Record<string, unknown>).map(([key, item]) => [key, String(item ?? "")]),
    );

  return {
    name: name,
    transport: http ? "http" : "stdio",
    command: http ? "" : command,
    args: http || !Array.isArray(entry.args) ? [] : entry.args.map(arg => String(arg)),
    env: http ? {} : strings(entry.env),
    url: url,
    headers: http ? strings(entry.headers) : {},
  };
}

function isEntry(value: unknown) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return false;
  }
  const entry = value as Record<string, unknown>;
  return ["command", "url", "httpUrl", "serverUrl", "type", "args"].some(field => entry[field] !== undefined);
}

/**
 * The block a server's own README hands out, read as the servers it declares.
 * Every shape that block comes in is accepted — the mcpServers wrapper Claude
 * writes, the servers one VS Code writes, a bare name-to-entry map, or a single
 * unwrapped entry — because which one a page shows is not the reader's choice.
 */
export function parseMcpJson(text: string): {servers: McpDraft[]; error: string} {
  const trimmed = text.trim();
  if (trimmed === "") {
    return {servers: [], error: ""};
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return {servers: [], error: "notJson"};
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return {servers: [], error: "notServers"};
  }

  const root = parsed as Record<string, unknown>;
  const wrapper = serverMaps.find(field => typeof root[field] === "object" && root[field] !== null);
  if (wrapper === undefined && isEntry(root)) {
    const draft = entryDraft("", root);
    return draft === null ? {servers: [], error: "noCommand"} : {servers: [draft], error: ""};
  }

  const map = (wrapper === undefined ? root : root[wrapper]) as Record<string, unknown>;
  const entries = Object.entries(map).filter(([, value]) => isEntry(value));
  if (entries.length === 0) {
    return {servers: [], error: "notServers"};
  }

  const servers: McpDraft[] = [];
  for (const [name, value] of entries) {
    const draft = entryDraft(name, value as Record<string, unknown>);
    if (draft === null) {
      return {servers: [], error: "noCommand"};
    }
    servers.push(draft);
  }
  return {servers: servers, error: ""};
}
