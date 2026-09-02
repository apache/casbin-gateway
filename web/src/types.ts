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

/** The theme the shell renders in; "dark" toggles the .dark class on <html>. */
export type ThemeAlgorithm = ("default" | "dark")[];

/**
 * The envelope every /api handler returns (controllers.Response): "ok" or
 * "error" in `status`, the payload in `data`, and a second value such as a row
 * count or the hostname in `data2`.
 */
export interface ApiResponse<T = any, T2 = any> {
  status: "ok" | "error";
  msg: string;
  data: T;
  data2: T2;
}

export interface Account {
  owner: string;
  name: string;
  displayName: string;
  avatar: string;
  isAdmin: boolean;
  /** Filled in from `data2` of /api/get-account, not part of the user row. */
  hostname?: string;
}

export interface SigninOptions {
  casdoorAvailable: boolean;
  signinAvailable: boolean;
  autoSignin: boolean;
  authConfig: {
    serverUrl: string;
    clientId: string;
    appName: string;
    organizationName: string;
  };
}

export interface Provider {
  owner: string;
  name: string;
  displayName: string;
  type: string;
  status: string;
  models: string[];
  priority: number;
  baseUrl: string;
  apiKey: string;
  authMode: string;
  /** A site the vendor's icon is taken from, or an image URL. Empty derives it
   * from the base URL. */
  icon: string;
  /** Whatever the person who added the provider wants to remember about it. */
  notes: string;
  /** Null leaves the built-in balance endpoints to answer. */
  quota?: QuotaConfig | null;
}

/** Where a provider's balance is read from, when there is no built-in endpoint
 * for its vendor. Mirrors object.QuotaConfig on the server. */
export interface QuotaConfig {
  url: string;
  // An index signature rather than Record: this module declares a Record entity
  // of its own, which shadows the built-in type.
  headers: {[name: string]: string};
  token: string;
  remaining: string;
  used: string;
  total: string;
  unit: string;
  scale: number;
}

/** What the vendor last said is left on the account behind a provider. */
export interface ProviderQuota {
  provider: string;
  /** False when nothing knows where to ask, which is not an error. */
  supported: boolean;
  remaining: number | null;
  used: number | null;
  total: number | null;
  unit: string;
  error: string;
  time: string;
}

export interface ProviderTestResult {
  success: boolean;
  statusCode?: number;
  message: string;
}

/** One configuration file a provider switch writes, with what it will contain. */
export interface AgentProviderFile {
  path: string;
  format: string;
  preview: string;
}

/** The state of the agent's own configuration file, written by the orchestrator. */
export interface AgentProviderConfig {
  supported: boolean;
  /** The wire format the agent's client speaks, empty when Gateway cannot tell. */
  protocol: string;
  applied: boolean;
  provider: string;
  mode: string;
  baseUrl: string;
  time: string;
  files: string[];
  detail: string;
  /** The model the agent uses on its own, or the account it signs in to. */
  builtin: string;
  /** The endpoint the agent's files name now, whichever tool wrote them. */
  current: string;
}

export interface Agent {
  agentId: string;
  name: string;
  version: string;
  installMethod: string;
  owner: string;
  path: string;
  supported: boolean;
  patched: boolean;
  detail?: string;
  notice?: string;
  followup?: string;
  /** The "owner/name" id of the provider this agent's requests are sent to. */
  provider: string;
  /** The providers tried, in order, when the bound one cannot answer. */
  fallbacks: string[];
  /** "gateway" routes through the local proxy, "direct" writes the upstream. */
  mode: string;
  providerConfig: AgentProviderConfig;
  /** Where this agent reaches its bound provider, as the server resolved it. */
  proxyBaseUrl?: string;
  /** The command that would update this installation where it stands. */
  upgrade?: AgentInstallPlan;
}

/**
 * What installing or upgrading one agent would run here. The command is built
 * on the server from what Gateway knows about the agent, so the page shows it
 * and asks for it by agent, never by command.
 */
export interface AgentInstallPlan {
  agentId: string;
  /** "install" for an agent this machine lacks, "upgrade" for one it has. */
  action: string;
  /** The package manager that would do it: npm, winget or homebrew. */
  manager?: string;
  command?: string;
  available: boolean;
  /** Why there is no command, when there is none. */
  detail?: string;
  installUrl?: string;
}

/** One install or upgrade, polled while the package manager works. */
export interface AgentInstallJob {
  agentId: string;
  name: string;
  action: string;
  manager: string;
  command: string;
  running: boolean;
  ok: boolean;
  /** The tail of what the package manager printed, on both streams. */
  output: string;
  error?: string;
  startTime: string;
  endTime?: string;
}

/**
 * One agent Gateway knows how to detect, whether or not it is installed here.
 * The ones missing from a scan are listed as something to install.
 */
export interface AgentCatalogEntry {
  agentId: string;
  name: string;
  /** The vendor's own install page, opened in a new tab. */
  installUrl?: string;
  desktop?: boolean;
  /** The command that would install it here, when a package manager can. */
  install: AgentInstallPlan;
}

/** The live processes of one agent installation, keyed by owner and path. */
export interface AgentRuntime {
  agentId: string;
  path: string;
  owner: string;
  running: boolean;
  pids: number[];
  /** True for a windowed app: a CLI is started in a console window instead. */
  desktop: boolean;
  /** False when no launcher was resolved, so the agent cannot be started here. */
  canStart: boolean;
  detail?: string;
}

/** What the proxy has seen of one provider since Gateway started. */
export interface ProviderHealth {
  provider: string;
  healthy: boolean;
  successes: number;
  failures: number;
  consecutive: number;
  lastError: string;
  lastFailure: string;
  /** When a suspended provider is tried again, empty while it is healthy. */
  retryTime: string;
}

export interface AgentRecord {
  id: string;
  createdTime: string;
  agent: string;
  agentPath?: string;
  user?: string;
  sessionKey?: string;
  title?: string;
  promptId?: string;
  toolUseId?: string;
  eventType: string;
  action?: string;
  outcome?: string;
  toolName?: string;
  mcpServer?: string;
  mcpTool?: string;
  model?: string;
  durationMs?: number;
  clientIp?: string;
  detail?: string;
  object?: unknown;
}

export interface AgentSession {
  agent: string;
  sessionKey: string;
  title?: string;
  recordCount: number;
  firstTime: string;
  lastTime: string;
  /** Where the transcript is, for a session read off disk. */
  path?: string;
  /** The directory the agent was working in, when the transcript records one. */
  cwd?: string;
  /** True when the session comes from the agent's own transcript, not monitoring. */
  historical?: boolean;
}

/** One part of a transcript message: prose, thinking, a tool call or its result. */
export interface AgentMessageBlock {
  kind: "text" | "thinking" | "toolUse" | "toolResult" | "image";
  text?: string;
  tool?: string;
  isError?: boolean;
}

export interface AgentMessage {
  role: string;
  time?: string;
  blocks: AgentMessageBlock[];
}

/** One session read in full out of the agent's own transcript file. */
export interface AgentTranscript {
  session: AgentSession;
  messages: AgentMessage[];
  /** True when the file holds more than what was read. */
  truncated?: boolean;
}

export interface LlmFailure {
  provider: string;
  status: number;
  error: string;
}

export interface LlmRecord {
  id: number;
  createdTime: string;
  protocol: string;
  endpoint: string;
  model: string;
  provider: string;
  agent: string;
  clientIp: string;
  stream: boolean;
  status: number;
  durationMs: number;
  attempts: number;
  error: string;
  /** What the upstream answered a failed request with. Only returned by getLlmRecord. */
  errorBody: string;
  /** The attempts the provider chain failed over from, oldest first. */
  failures: LlmFailure[];
  /** Input billed as fresh: the cached part is counted separately. */
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  totalTokens: number;
  /** US dollars, meaningful only where `priced` is true. */
  cost: number;
  priced: boolean;
  systemBytes: number;
  messageCount: number;
  toolCount: number;
  summary: string;
  /** Only returned by getLlmRecord, the list endpoint leaves it out. */
  payload: string;
  redactions: number;
  truncated: boolean;
  bytes: number;
}

export interface LlmRecordStatus {
  mode: "off" | "metadata" | "full";
  retentionDays: number;
  maxRecords: number;
  dropped: number;
  count: number;
}

/** US dollars per million tokens, as the record was costed. */
export interface LlmPrice {
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
}

/** Where a price in effect came from, in the order each overrides the one before. */
export type LlmPriceSource = "built-in" | "file" | "models.dev" | "manual";

/** A stored override of what a model costs, as the edit dialog sends it. */
export interface LlmPriceEntry {
  model: string;
  displayName: string;
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
  cacheWrite1h: number;
  source: LlmPriceSource;
  updatedTime: string;
}

/** One model's price as it is actually in effect, with the layer it came from. */
export interface LlmPriceView {
  model: string;
  displayName: string;
  input: number;
  output: number;
  cacheWrite: number;
  cacheRead: number;
  cacheWrite1h: number;
  source: LlmPriceSource;
  /** True where a stored row stands in front of a built-in price. */
  overridden: boolean;
}

/** One model of the models.dev catalogue, priced at what most of the providers
 *  listing it agree that it costs. */
export interface ModelsDevModel {
  model: string;
  displayName: string;
  releaseDate: string;
  providers: number;
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
}

/** What one models.dev sync did. `missing` is the half that matters: those are
 *  the models still costing nothing. */
export interface ModelsDevSync {
  catalogue: number;
  considered: string[];
  updated: string[];
  skipped: string[];
  missing: string[];
  syncedTime: string;
}

export interface LlmModelStat {
  model: string;
  requests: number;
  tokens: number;
  cost: number;
}

export interface LlmProviderStat {
  provider: string;
  requests: number;
  failed: number;
  tokens: number;
  cost: number;
}

export interface LlmRecordStats {
  requests: number;
  failed: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  cost: number;
  /** Records whose model has no price entry. */
  unpriced: number;
  models: LlmModelStat[];
  providers: LlmProviderStat[];
}

/** One time bucket of the relayed traffic, as the usage trend is drawn from. */
export interface LlmTrendPoint {
  bucket: string;
  requests: number;
  failed: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  cost: number;
}

/** One agent's share of the relayed requests, with what it last asked for. */
export interface LlmAgentStat {
  agent: string;
  requests: number;
  failed: number;
  tokens: number;
  cost: number;
  lastTime: string;
  lastModel: string;
  lastProvider: string;
}

/**
 * What one agent, one model or one day spent, read from the transcripts the
 * agents keep themselves. Which of the three a stat counts is decided by the
 * list it is in; `name` is that key.
 */
export interface AgentUsageStat {
  name: string;
  requests: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  reasoningTokens: number;
  totalTokens: number;
  cost: number;
  /** Requests left out of `cost` because no list price matched their model. */
  unpriced: number;
  /** Filled for agents only, which is what the card beside the totals shows. */
  lastTime: string;
  lastModel: string;
}

/**
 * What the agents on this machine spent, whether or not Gateway relayed any of
 * it. LLM Records is empty until an agent is routed through the gateway, and
 * stays empty for one talking to its vendor directly; this is read off the
 * agent's own transcripts, so it counts either way.
 */
export interface AgentUsage {
  totals: AgentUsageStat;
  agents: AgentUsageStat[];
  models: AgentUsageStat[];
  /** Oldest first, which is the order a trend is drawn in. */
  days: AgentUsageStat[];
  /** How many transcripts the totals were read from. */
  sessions: number;
  /** The first day the totals cover, or empty when they cover everything. */
  since: string;
}

/** The settings the web UI can change, so nobody has to edit conf/app.conf. */
/**
 * The one built-in row of the Setting table, holding everything that used to be
 * hand-edited in conf/app.conf. The file only seeds it on the first start.
 */
export interface Setting {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;

  llmRecordMode: "off" | "metadata" | "full";
  llmRecordQueueCapacity: number;
  llmRecordRetentionDays: number;
  llmRecordMaxRecords: number;
  llmRecordMaxPayloadBytes: number;
  llmPricingFile: string;

  agentPatchStateDir: string;
  agentRecordCapacity: number;
  agentMonitorPollSeconds: number;

  casdoorEndpoint: string;
  clientId: string;
  clientSecret: string;
  casdoorOrganization: string;
  casdoorApplication: string;

  apiKeyEncryptionKey: string;
  relayToken: string;

  httpProxy: string;
}

/** The kinds of configuration the Skills, MCP & Prompts page manages. */
export type AgentConfigKind = "skill" | "mcp" | "prompt";

export interface AgentConfigItem {
  agentId: string;
  owner: string;
  kind: AgentConfigKind;
  name: string;
  /**
   * What two agents' copies of this item are matched by, when that is not the
   * name: every agent's instruction file holds the same thing under its own
   * file name.
   */
  shared?: string;
  description?: string;
  /**
   * The skill's own folder, the config file the MCP server is an entry of, or
   * the instruction file itself.
   */
  path: string;
  transport?: string;
  command?: string;
  url?: string;
  files?: number;
  bytes?: number;
  /**
   * "user" for a skill the operator wrote, "plugin" for one a plugin ships,
   * "project" for one that belongs to a checkout the agent works in.
   */
  scope?: string;
  /** The plugin or project folder a skill came from. */
  origin?: string;
  /** The checkout a project skill belongs to. */
  project?: string;
  /** Identifies the content: two agents with different digests hold different versions. */
  digest?: string;
  /** The newest file behind the item, in seconds. */
  modified?: number;
  /** Where a skill came from, and whether that source still holds the same content. */
  update?: AgentConfigSkillUpdate;
  /** Written by Gateway's own agent monitoring, and not the operator's to move. */
  managed?: boolean;
  /** Why Gateway will not delete this item. Empty when it may. */
  readOnly?: string;
  /** Listed because the agent would read it, not because it is there. */
  missing?: boolean;
}

/**
 * How one skill stands against the copy it came from: the same content, an
 * update waiting in the source, edits made here, or both at once.
 */
export type AgentConfigUpdateState = "current" | "available" | "modified" | "diverged" | "unknown";

export interface AgentConfigSkillUpdate {
  state: AgentConfigUpdateState;
  /** The folder this skill was copied from, and the agent that folder belongs to. */
  source?: string;
  sourceAgentId?: string;
  sourceName?: string;
  /** True when Gateway matched the source by name instead of recording the copy. */
  inferred?: boolean;
  sourceDigest?: string;
  sourceModified?: number;
  copiedAt?: number;
}

/** One installation's configuration, as it exists in its own files. */
export interface AgentConfigInventory {
  agentId: string;
  owner: string;
  name: string;
  path?: string;
  /** The account directory the locations below were resolved under. */
  home?: string;
  /** False when the configuration exists but no installation was detected. */
  installed: boolean;
  /** Other agents reading the same files, e.g. Cursor and its CLI. */
  sharedWith?: string[];
  /** Where a copied skill is written; skillsDirs is everything the listing was read from. */
  skillsDir?: string;
  skillsDirs?: string[];
  mcpFile?: string;
  /** The one Markdown file this agent reads before every session. */
  promptFile?: string;
  skillsSupported: boolean;
  mcpSupported: boolean;
  promptSupported: boolean;
  mcpWritable: boolean;
  mcpReadOnly?: string;
  skills: AgentConfigItem[];
  mcpServers: AgentConfigItem[];
  prompts: AgentConfigItem[];
  errors?: string[];
}

export interface AgentConfigDetail {
  item: AgentConfigItem;
  content: string;
  files?: string[];
}

/** One deleted item, waiting in Gateway's trash until it is restored or expires. */
export interface AgentConfigTrashEntry {
  id: string;
  agentId: string;
  owner: string;
  kind: AgentConfigKind;
  name: string;
  description?: string;
  /** Where it was, and where restoring puts it back. */
  path: string;
  files?: number;
  bytes?: number;
  deletedAt: number;
}

/** How an MCP server is reached: a spawned command, or an HTTP endpoint. */
export type McpTransport = "stdio" | "http";

export type AgentConfigAction = "create" | "overwrite" | "skip" | "failed";

/** What a copy would do, or did, to one item at one target agent. */
export interface AgentConfigPlanItem {
  agentId: string;
  name: string;
  action: AgentConfigAction;
  reason?: string;
  path?: string;
}

/** What this Gateway executable knows about itself. */
export interface VersionBuild {
  version: string;
  commit: string;
  shortCommit: string;
  buildTime: string;
  /** Built from a checkout with uncommitted changes, which no commit identifies. */
  modified: boolean;
  os: string;
  arch: string;
  goVersion: string;
}

/** The published build this Gateway can move to. */
export interface VersionRelease {
  tag: string;
  name: string;
  commit: string;
  shortCommit: string;
  publishedAt: string;
  pageUrl: string;
  assetName: string;
  assetUrl: string;
  assetSize: number;
}

export type UpdateStage = "idle" | "downloading" | "installing" | "restarting" | "failed";

export interface UpdateStatus {
  stage: UpdateStage;
  percent: number;
  downloaded: number;
  total: number;
  target: string;
  error: string;
  /** The failure was reaching GitHub, which a proxy can fix. */
  network: boolean;
}

export interface VersionInfo {
  current: VersionBuild;
  latest: VersionRelease | null;
  updateAvailable: boolean;
  canUpdate: boolean;
  /** "unsupported-platform", "read-only" or "no-executable" when it cannot update itself. */
  blocked: string;
  /** A failed lookup, which leaves the current version worth showing anyway. */
  checkError: string;
  /** The lookup failed because GitHub could not be reached. */
  checkNetwork: boolean;
  update: UpdateStatus;
  releaseUrl: string;
  installCommand: string;
}
