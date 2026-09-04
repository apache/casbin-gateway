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

/** Which colour set the theme draws from, light or dark; see index.css. */
export type Palette = "amber" | "terminal" | "indigo" | "neutral";

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
  /** True asks no endpoint: the balance is `initial`, drawn down by recorded
   * spend through this provider since `since`. */
  manual: boolean;
  initial: number;
  /** Where the drawdown starts counting. The server sets it; an empty value
   * asks the server to start over from now. */
  since: string;
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

/**
 * One environment variable that an agent reads before its own configuration
 * file, so that what Gateway wrote into that file never takes effect.
 */
export interface EnvConflict {
  key: string;
  /** Masked when the variable carries a credential. */
  value: string;
  /** Where it is set, with the file in path when it is set in one. */
  source: string;
  path: string;
  /** The command that clears it, empty when the file has to be edited. */
  fix: string;
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
  /** The environment variables that override what Gateway writes here. */
  envConflicts: EnvConflict[];
}

/** The cloud account an agent is signed in to, read from its own state. */
export interface AgentAccount {
  email?: string;
  name?: string;
  plan?: string;
}

export interface Agent {
  agentId: string;
  name: string;
  version: string;
  installMethod: string;
  owner: string;
  path: string;
  /** The account signed in to this agent, absent when none can be read. */
  account?: AgentAccount;
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
  /** The command that would remove it with the manager that installed it. */
  uninstall?: AgentInstallPlan;
  /** Whether a second copy of this agent can run beside the first. */
  supportsInstances?: boolean;
}

/** One switch of the Permissions page: something an agent may be allowed to do,
 *  and the tool names the agents call it by. The name is "<group>/<item>", which
 *  is also the casbin object it is checked as. */
export interface ToolItem {
  name: string;
  group: string;
  /** Set for an item the page has no wording of its own for, such as one of the
   *  agent's MCP servers. */
  label?: string;
  tools: string[];
}

/** One group of switches, which can also be set in one go. */
export interface ToolGroup {
  name: string;
  items: ToolItem[];
}

/** What one agent is allowed to ask the gateway for. Mirrors
 *  object.AgentPermission on the server. */
export interface AgentPermission {
  owner: string;
  name: string;
  createdTime?: string;
  updatedTime?: string;
  /** Off leaves the agent unrestricted, which is what it is until this is on. */
  enabled: boolean;
  /** "all", "allow" for only the listed, or "deny" for everything but them. */
  modelMode: string;
  models: string[];
  providerMode: string;
  providers: string[];
  /** One entry per tool group, false where the group is switched off. */
  tools: {[group: string]: boolean};
  /** Extra casbin policy lines, written by hand on the advanced view. */
  rules: string[];
}

/** The permissions of one agent with what the card needs to draw them: the tool
 *  groups, and the casbin model and policy they compile to. */
export interface AgentPermissionInfo {
  permission: AgentPermission;
  groups: ToolGroup[];
  model: string;
  policy: string[];
}

/**
 * One extra copy of an agent, kept apart from the others by a state directory
 * of its own, so that each is signed in to a different account and the two can
 * work at the same time.
 */
export interface AgentInstance {
  /** The stored id, "<agentId>/<instance>", which every call names it by. */
  name: string;
  agentId: string;
  instance: string;
  displayName: string;
  /** The private state directory this copy is started with. */
  dataDir: string;
  path: string;
  hostUser: string;
  createdTime: string;
  /** The account this copy is signed in to, absent until someone signs in. */
  account?: AgentAccount;
  desktop: boolean;
  running: boolean;
  pids: number[];
  canStart: boolean;
  detail?: string;
  /** Whether Gateway can route this agent's own sign-in links on this host. */
  canCapture?: boolean;
  /** Whether the next sign-in link will open this copy. */
  capturing?: boolean;
}

/**
 * What installing or upgrading one agent would run here. The command is built
 * on the server from what Gateway knows about the agent, so the page shows it
 * and asks for it by agent, never by command.
 */
export interface AgentInstallPlan {
  agentId: string;
  /** "install", "upgrade", "downgrade" or "uninstall". */
  action: string;
  /** What would do it: a package manager (npm, winget, homebrew, msstore), the
   *  app's registered uninstaller, a Store package, the agent's own updater,
   *  the vendor's install command, or the files on disk. */
  manager?: string;
  command?: string;
  /** The release a pinned install asks for, absent for the current one. */
  version?: string;
  available: boolean;
  /** Why there is no command, when there is none. */
  detail?: string;
  /** The command opens a window of its own and waits for whoever is at the
   *  machine: an uninstaller with no silent mode, or a consent prompt. */
  interactive?: boolean;
  /** What this command does that a package manager would not. */
  warning?: string;
  installUrl?: string;
}

/** What one agent's package manager publishes, for the version picker. */
export interface AgentVersionCatalog {
  agentId: string;
  manager?: string;
  package?: string;
  /** The release the manager calls current. */
  latest?: string;
  /** Every published release, newest first. */
  versions: string[];
  /** The command a version change runs, with the release left as "{version}".
   *  Absent for a manager that only installs the one version it indexes. */
  commandTemplate?: string;
  /** Why there is nothing to list, when there is nothing. */
  detail?: string;
  checkedAt?: string;
}

/** One installation measured against what its package manager publishes. */
export interface AgentUpdate {
  agentId: string;
  path: string;
  owner: string;
  manager?: string;
  current?: string;
  latest?: string;
  /** A newer release is waiting. False while either version is unknown. */
  available: boolean;
  detail?: string;
  checkedAt?: string;
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
  /** The release a pinned install asked for, absent for the current one. */
  version?: string;
  /** The job put a window on screen and is waiting for whoever is at the
   *  machine, rather than working on its own. */
  interactive?: boolean;
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
  id: number;
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

/** Whether the catalogue is read on a schedule. */
export type ModelsDevSyncMode = "auto" | "off";

/** The schedule an automatic sync is on, and what the last run did. Held in
 *  memory by the process, so it is empty again after a restart. */
export interface ModelsDevSyncState {
  mode: ModelsDevSyncMode;
  intervalHours: number;
  running: boolean;
  syncedTime: string;
  /** When the schedule runs again, empty when nothing is scheduled. */
  nextTime: string;
  error: string;
  result: ModelsDevSync | null;
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

/** The checks a channel audit runs, and how sure each one is of its answer. */
export type LlmAuditKey = "cache" | "errors" | "latency" | "pricing";
export type LlmAuditLevel = "ok" | "warn" | "alert" | "unknown";

/**
 * One measurement of a provider. The server sends the number and the level and
 * leaves the wording here, so a check reads as a fact with the sample it was
 * measured over rather than as a verdict.
 */
export interface LlmAuditCheck {
  key: LlmAuditKey;
  level: LlmAuditLevel;
  /** A share of `sample` for every check but latency, which is P95 over P50. */
  value: number;
  sample: number;
}

/** What the kept records say about one provider that served them. */
export interface LlmProviderAudit {
  provider: string;
  requests: number;
  failed: number;
  /** Attempts this provider lost, which another one then answered. */
  failedOver: number;
  retried: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  totalTokens: number;
  cost: number;
  /** Answered requests a cache could have been reported on at all. */
  cacheable: number;
  cacheHits: number;
  latencyP50Ms: number;
  latencyP95Ms: number;
  unpriced: number;
  unpricedModels: string[];
  models: string[];
  firstTime: string;
  lastTime: string;
  checks: LlmAuditCheck[];
}

export interface LlmAuditReport {
  scanned: number;
  /** The window held more records than one audit reads. */
  truncated: boolean;
  providers: LlmProviderAudit[];
}

/** How probes are started. An automatic probe spends a little of the credit. */
export type ProviderProbeMode = "auto" | "manual" | "off";

/** The questions an active probe asks, none of which the records can answer. */
export type ProbeKey =
  | "identity"
  | "vendor"
  | "stream"
  | "cache"
  | "tools"
  | "billing"
  | "knowledge"
  | "selfid"
  | "hidden"
  | "feature"
  | "repeat";

/**
 * One probe measurement. `facts` is what actually came back, as data rather
 * than as a sentence — the model name that answered, the headers present, the
 * events missing, the counts that disagreed — so the page can word it in the
 * reader's language and still quote the value the level was drawn from.
 */
export interface ProbeCheck {
  key: ProbeKey;
  /** The test case that produced this, and what it was called when it ran. */
  case: string;
  title: string;
  /** What this check was worth in the score of that run. */
  weight: number;
  level: LlmAuditLevel;
  facts: string[];
  value: number;
}

/** How far an upstream got through the suite, as a letter. */
export type ProbeGrade = "A" | "B" | "C" | "D" | "F" | "unknown";

/** The knobs of the check engine a case runs on. Fields another engine owns are
 * ignored, so one shape edits every case in the suite. */
export interface ProbeCaseParams {
  system: string;
  prompt: string;
  maxTokens: number;
  toolName: string;
  /** A JSON schema the forced tool call has to fill. */
  schema: string;
  events: string[];
  fillerChars: number;
  gapMs: number;
  headers: string[];
  minHeaders: number;
  driftTolerance: number;
  warnHigh: number;
  alertHigh: number;
  warnLow: number;
  alertLow: number;
  /** The answers that pass, the ones that fail, and how they are compared. */
  expect: string[];
  forbid: string[];
  match: "" | "contains" | "exact" | "regex";
  /** JSON merged into the request body, and the answer fields it has to
   * produce, as dotted paths with an optional "=pattern". */
  extra: string;
  require: string[];
  /** How many times an identical request goes out. */
  samples: number;
  /** Vendors whose models this case applies to. Empty asks it of every model. */
  vendors: string[];
}

/**
 * One test in the suite. `question` is what it asks the upstream and `method`
 * how the answer is judged: the two are published so a score can be argued
 * with, and both are editable, as is the weight the score is built from.
 */
export interface ProbeCase {
  name: string;
  createdTime: string;
  updatedTime: string;
  displayName: string;
  check: ProbeKey;
  /** Empty runs the case against both upstream APIs. */
  protocol: "" | "anthropic" | "openai";
  enabled: boolean;
  weight: number;
  sort: number;
  /** Shipped with Gateway, so restoring the defaults brings it back. */
  builtIn: boolean;
  question: string;
  method: string;
  params: ProbeCaseParams;
  /** A built-in case whose words were rewritten here, which is what stops a
   * shipped translation from standing in front of what someone typed. */
  edited: boolean;
}

/** One run of the probe suite against one provider. */
export interface ProviderProbe {
  id: number;
  provider: string;
  createdTime: string;
  /** Why it ran: "added", "edited", "unused", "scheduled" or "manual". */
  trigger: string;
  protocol: string;
  model: string;
  /** The model name the upstream answered with, which may not be the one asked. */
  upstreamModel: string;
  ok: boolean;
  error: string;
  /** The weighted share of the measurable cases that answered the way the
   * vendor's own API documents, out of 100, and the letter it falls in. */
  score: number;
  grade: ProbeGrade;
  ttftMs: number;
  durationMs: number;
  /** What the probe itself spent. */
  requests: number;
  promptTokens: number;
  completionTokens: number;
  cacheReadTokens: number;
  cacheWriteTokens: number;
  cost: number;
  priced: boolean;
  vendorHeaders: string[];
  checks: ProbeCheck[];
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
  /** The recent per-day breakdown, oldest first. Filled for agents only, and
   *  is what their cards draw the trend from. */
  days?: AgentUsageStat[];
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

  providerProbeMode: ProviderProbeMode;
  providerProbeIntervalHours: number;

  modelsDevSyncMode: ModelsDevSyncMode;
  modelsDevSyncIntervalHours: number;

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

  /** Comma-separated: extra Host names Gateway answers to, and the browser
   *  origins allowed to call it from another site. */
  allowedHosts: string;
  allowedOrigins: string;

  httpProxy: string;

  backupMode: BackupMode;
  backupIntervalHours: number;
  backupRetention: number;
  backupDir: string;

  cloudSyncMode: CloudSyncMode;
  cloudSyncKind: string;
  /** The options of the chosen kind, as one JSON document: a kind added later
   *  brings its own keys, and this row does not have to learn them. */
  cloudSyncOptions: string;
}

export type BackupMode = "auto" | "off";

/** Which sections a snapshot carries, and whether the credentials inside them
 *  come with it. */
export interface SnapshotScope {
  providers: boolean;
  agents: boolean;
  probeCases: boolean;
  llmPrices: boolean;
  setting: boolean;
  secrets: boolean;
}

export interface SnapshotCounts {
  providers: number;
  agents: number;
  agentInstances: number;
  probeCases: number;
  llmPrices: number;
  setting: number;
}

/** The exported document itself. It is only ever handed straight back to the
 *  server or written to a file, so the sections stay opaque here. */
export interface Snapshot {
  version: number;
  createdTime: string;
  gateway: string;
  host: string;
  reason: string;
  scope: SnapshotScope;
  [section: string]: unknown;
}

export type ImportMode = "merge" | "overwrite" | "replace";

export interface ImportChange {
  section: string;
  id: string;
  action: "add" | "update" | "delete" | "skip" | "fail";
  detail: string;
}

/** What an import did, and what a dry run says it would do: the two are the
 *  same document, so the confirmation and the result read alike. */
export interface ImportReport {
  dryRun: boolean;
  mode: ImportMode;
  added: number;
  updated: number;
  deleted: number;
  skipped: number;
  failed: number;
  changes: ImportChange[];
  /** The snapshot taken before anything was written, empty on a dry run. */
  backup: string;
}

/** One snapshot on disk. */
export interface Backup {
  name: string;
  createdTime: string;
  reason: string;
  gateway: string;
  host: string;
  size: number;
  secrets: boolean;
  counts: SnapshotCounts;
}

/** The schedule automatic backups are on, and the files themselves. */
export interface BackupState {
  mode: BackupMode;
  intervalHours: number;
  retention: number;
  dir: string;
  takenTime: string;
  /** When the schedule runs again, empty when nothing is scheduled. */
  nextTime: string;
  latest: string;
  error: string;
  backups: Backup[];
  /** The synced folders this machine has, offered as a place to keep them. */
  folders: SyncedFolder[];
}

export type CloudSyncMode = "auto" | "off";

export type CloudSyncDirection = "both" | "up" | "down";

/** A folder on this machine that a desktop client already syncs. */
export interface SyncedFolder {
  name: string;
  path: string;
  /** The path with Gateway's own subdirectory on the end, which is what
   *  picking the folder fills the field with. */
  suggested: string;
}

/** One thing a storage kind needs to be told. The server describes its own
 *  form, so a kind added later needs no change here. */
export interface CloudSyncField {
  name: string;
  label: string;
  type: "text" | "secret" | "switch";
  placeholder: string;
  hint: string;
  required: boolean;
}

/** One storage Gateway can copy the backups to. */
export interface CloudSyncKind {
  name: string;
  displayName: string;
  description: string;
  fields: CloudSyncField[];
}

/** One file at the target. */
export interface CloudSyncFile {
  name: string;
  size: number;
  modifiedTime: string;
}

/** What one run did. */
export interface CloudSyncResult {
  uploaded: string[];
  downloaded: string[];
  removed: string[];
  skipped: number;
  errors: string[];
  remote: CloudSyncFile[];
}

/** Where the backups are copied to, and how the last run went. */
export interface CloudSyncState {
  mode: CloudSyncMode;
  kind: string;
  options: Record<string, string>;
  kinds: CloudSyncKind[];
  folders: SyncedFolder[];
  /** Where the copies land, in one line and without credentials. */
  target: string;
  running: boolean;
  syncedTime: string;
  error: string;
  result: CloudSyncResult | null;
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
   * What a skill installed as a symbolic link points at. A linked skill follows
   * its source; a copied one is the agent's own from the moment it lands.
   */
  link?: string;
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

/** What a vendor's "add this to Gateway" link carries. */
export type ImportResource = "provider" | "mcp" | "prompt" | "skill";

export interface ImportLink {
  resource: ImportResource;
  provider?: Provider;
  mcp?: McpImport;
  prompt?: PromptImport;
  skill?: SkillImport;
}

/** One or more MCP servers, as the JSON block the link carried. */
export interface McpImport {
  name: string;
  config: string;
  /** The agents the link asks for, as ids of the agents on this host. */
  targets: string[];
  /** The apps it named that Gateway does not manage. */
  unknown: string[];
}

export interface PromptImport {
  name: string;
  description: string;
  content: string;
  targets: string[];
  unknown: string[];
}

export interface SkillImport {
  name: string;
  repo: string;
  ref: string;
  subdir: string;
}

/** Where a skill source's content comes from. */
export type SkillSourceKind = "github" | "archive" | "upload" | "local";

/** One place skills can be installed from. */
export interface SkillSource {
  id: string;
  name: string;
  kind: SkillSourceKind;
  /** owner/repo for a repository, the download URL for an archive, the folder for a local source. */
  url?: string;
  /** The branch, tag or commit of a repository; empty follows its default branch. */
  ref?: string;
  /** The one folder of the source to look in, for a repository that groups its skills. */
  subdir?: string;
  /** Seeded by Gateway rather than added here. It can still be removed. */
  builtin?: boolean;
  addedAt?: number;
  /** When Gateway last took a copy, and how many skills it held then. */
  fetchedAt?: number;
  skills?: number;
}

/** One skill of a source, before it is installed anywhere. */
export interface SkillCatalogItem {
  /** The folder it would be installed under, relative to the source. */
  name: string;
  path: string;
  description?: string;
  files?: number;
  bytes?: number;
  digest?: string;
  modified?: number;
}

export interface SkillCatalog {
  source: SkillSource;
  /** The folder the skills below were read from. */
  root: string;
  skills: SkillCatalogItem[];
}

/** Where an untracked skill appears to have come from. */
export interface SkillMatch {
  sourceId: string;
  sourceName: string;
  /** The name inside the source, and the folder it is in. */
  skill: string;
  path: string;
  /** The source holds exactly this content. */
  same: boolean;
  /** Recognized by its content after being renamed here. */
  byDigest?: boolean;
}

/** One skill on disk that Gateway did not install and has no record of. */
export interface UnmanagedSkill {
  agentId: string;
  owner: string;
  name: string;
  path: string;
  description?: string;
  files?: number;
  bytes?: number;
  digest?: string;
  modified?: number;
  match?: SkillMatch;
}

/**
 * How an installed skill relates to its source: a copy the agent then owns, or
 * a link that follows Gateway's copy of the source whenever it is fetched again.
 */
export type SkillInstallMode = "copy" | "link";

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

/** Whether this machine starts Gateway at login. Host state, not a setting: it
 *  lives in the login entry the desktop launcher and the tray share. */
export interface AutostartState {
  /** False where there is no desktop launcher to start, as in a container. */
  supported: boolean;
  enabled: boolean;
  launcher: string;
}
