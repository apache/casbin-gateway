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

export interface SiteNode {
  name: string;
  version: string;
  diff: string;
  pid?: number;
  status: string;
  message: string;
  provider?: string;
}

export interface Site {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;
  tag: string;
  domain: string;
  otherDomains: string[];
  needRedirect: boolean;
  disableVerbose: boolean;
  rules: string[];
  enableAlert: boolean;
  alertInterval: number;
  alertTryTimes: number;
  alertProviders: string[];
  challenges: string[];
  host: string;
  port: number;
  hosts: string[];
  sslMode: string;
  sslCert: string;
  publicIp: string;
  node: string;
  isSelf: boolean;
  nodes: SiteNode[];
  casdoorApplication: string;
  status?: string;
}

export interface Node {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;
  tag: string;
  clientIp: string;
  upgradeMode: string;
}

export interface Cert {
  owner: string;
  name: string;
  createdTime: string;
  displayName: string;
  type: string;
  cryptoAlgorithm: string;
  expireTime: string;
  domainExpireTime: string;
  provider: string;
  account: string;
  accessKey: string;
  accessSecret: string;
  certificate: string;
  privateKey: string;
}

export interface RuleExpression {
  name: string;
  operator: string;
  value: string;
}

export interface Rule {
  owner: string;
  name: string;
  createdTime: string;
  updatedTime: string;
  type: string;
  expressions: RuleExpression[];
  action: string;
  statusCode: number;
  reason: string;
  isVerbose: boolean;
}

export interface Record {
  id: number;
  owner: string;
  name: string;
  createdTime: string;
  method: string;
  host: string;
  path: string;
  clientIp: string;
  userAgent: string;
}

export interface Channel {
  owner: string;
  name: string;
  displayName: string;
  type: string;
  status: string;
  models: string[];
  priority: number;
  baseUrl: string;
  apiKey: string;
}

export interface ChannelTestResult {
  success: boolean;
  statusCode?: number;
  message: string;
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
  /** The "owner/name" id of the channel this agent's requests are sent to. */
  channel: string;
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
}

export interface LlmRecord {
  id: number;
  createdTime: string;
  protocol: string;
  endpoint: string;
  model: string;
  channel: string;
  agent: string;
  clientIp: string;
  stream: boolean;
  status: number;
  durationMs: number;
  attempts: number;
  error: string;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
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

export interface MetricPoint {
  data: string;
  count: number;
}

export interface LlmUsageTotals {
  requests: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

export interface LlmUsage {
  totals: LlmUsageTotals;
  overTime: MetricPoint[];
  byModel: MetricPoint[];
  byChannel: MetricPoint[];
  byAgent: MetricPoint[];
}

export interface Provider {
  name: string;
  category: string;
}

export interface Application {
  name: string;
}

export interface GatewayStatus {
  gatewayEnabled: boolean;
}
