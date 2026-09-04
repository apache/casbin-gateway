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

import * as Setting from "@/Setting";
import type {Provider} from "@/types";

/** The "owner/name" id an agent's routing names a provider by. */
export function providerIdOf(provider: Pick<Provider, "owner" | "name">) {
  return `${provider.owner}/${provider.name}`;
}

/** Mirrors object.ProviderProtocol on the server. */
export function providerProtocol(type: string) {
  return type === "anthropic" ? "anthropic" : "openai";
}

/** Mirrors object.ProviderAuthProvider and object.ProviderAuthClient on the server. */
export const authProvider = "provider";
export const authClient = "client";

/** Mirrors object.ServesResponsesApi: only OpenAI itself serves that API. */
export function servesResponsesApi(provider: {type?: string} | undefined) {
  return provider?.type === "openai";
}

/** Mirrors object.UsesClientAuth: whose credentials reach the upstream. */
export function usesClientAuth(provider: {authMode?: string} | undefined) {
  return provider?.authMode === authClient;
}

/** The env vars a client of one wire format reads its endpoint and key from. */
const protocolEnv: Record<string, {baseUrl: string; token: string}> = {
  anthropic: {baseUrl: "ANTHROPIC_BASE_URL", token: "ANTHROPIC_AUTH_TOKEN"},
  openai: {baseUrl: "OPENAI_BASE_URL", token: "OPENAI_API_KEY"},
};

/**
 * What a client sends the relay while its own key stays here. Gateway issues it
 * on first start; this is only the fallback for a page that has not been told
 * the real one yet.
 */
const placeholderToken = "casbin-gateway";

export const shells = ["bash", "PowerShell"] as const;
export type Shell = (typeof shells)[number];

/**
 * The lines that point a client of one wire format at a base URL. A client-auth
 * provider forwards the client's own credentials, so the token is left out
 * there: setting it would replace the sign-in the client already has.
 */
export function envSnippet(
  protocol: string,
  baseUrl: string,
  shell: Shell,
  includeToken = true,
  token = placeholderToken,
) {
  const env = protocolEnv[protocol] ?? protocolEnv.openai;
  const variables: [string, string][] = [[env.baseUrl, baseUrl]];
  if (includeToken) {
    variables.push([env.token, token || placeholderToken]);
  }

  return variables
    .map(([name, value]) =>
      shell === "PowerShell" ? `$env:${name} = "${value}"` : `export ${name}="${value}"`,
    )
    .join("\n");
}

/**
 * The base URL for requests routed by model name, i.e. not through an agent.
 * An OpenAI client appends /chat/completions to it and an Anthropic one appends
 * /v1/messages, which is why the /v1 belongs to only one of them.
 */
export function gatewayBaseUrl(protocol: string) {
  const origin = Setting.ServerUrl || window.location.origin;
  return protocol === "anthropic" ? origin : `${origin}/v1`;
}

/** The shell a host is driven from, guessed from an absolute path on it. */
export function shellForPath(path: string): Shell {
  return /^[a-zA-Z]:[\\/]/.test(path) ? "PowerShell" : "bash";
}

/** The shell the browser's own machine is driven from. */
export function localShell(): Shell {
  return navigator.userAgent.includes("Windows") ? "PowerShell" : "bash";
}

/**
 * The identifier stored for a provider, derived from the name the user sees so
 * that it stays readable. The server appends a number when it is already taken,
 * which is why nothing random is needed here.
 */
export function providerSlug(label: string) {
  const slug = label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 60);

  return slug === "" ? "provider" : slug;
}

/**
 * Who a vendor is, which is the thing worth knowing when picking one: the maker
 * of the models, a platform that hosts models it did not make, or a site that
 * resells many vendors behind one key.
 */
export const vendorCategories = ["official", "platform", "aggregator"] as const;
export type VendorCategory = (typeof vendorCategories)[number];

/** A vendor the provider forms can fill themselves in from. */
export interface ProviderPreset {
  label: string;
  type: string;
  baseUrl: string;
  models: string[];
  category: VendorCategory;
  /** Where the vendor hands out API keys, so the form can point at it. */
  website: string;
}

/**
 * Every base URL below was checked against the live API rather than assumed.
 * Models are listed only where a short list is the whole catalogue: a platform
 * serving hundreds is left empty for the Fetch models button, which asks the
 * upstream itself and so never goes stale.
 */
export const providerPresets: ProviderPreset[] = [
  {
    label: "OpenAI",
    type: "openai",
    baseUrl: "https://api.openai.com/v1",
    models: ["gpt-5.5", "gpt-5", "gpt-5-mini", "o3", "o4-mini"],
    category: "official",
    website: "https://platform.openai.com/api-keys",
  },
  {
    label: "Anthropic",
    type: "anthropic",
    baseUrl: "https://api.anthropic.com",
    models: ["claude-opus-5", "claude-sonnet-5", "claude-haiku-4-5"],
    category: "official",
    website: "https://console.anthropic.com/settings/keys",
  },
  {
    label: "Google Gemini",
    type: "custom",
    baseUrl: "https://generativelanguage.googleapis.com/v1beta/openai",
    models: [],
    category: "official",
    website: "https://aistudio.google.com/apikey",
  },
  {
    label: "xAI",
    type: "custom",
    baseUrl: "https://api.x.ai/v1",
    models: [],
    category: "official",
    website: "https://console.x.ai",
  },
  {
    label: "Mistral",
    type: "custom",
    baseUrl: "https://api.mistral.ai/v1",
    models: [],
    category: "official",
    website: "https://console.mistral.ai/api-keys",
  },
  {
    label: "Cohere",
    type: "custom",
    baseUrl: "https://api.cohere.ai/compatibility/v1",
    models: [],
    category: "official",
    website: "https://dashboard.cohere.com/api-keys",
  },
  {
    label: "Perplexity",
    type: "custom",
    baseUrl: "https://api.perplexity.ai",
    models: [],
    category: "official",
    website: "https://www.perplexity.ai/settings/api",
  },
  {
    label: "DeepSeek",
    type: "custom",
    baseUrl: "https://api.deepseek.com/v1",
    models: ["deepseek-chat", "deepseek-reasoner"],
    category: "official",
    website: "https://platform.deepseek.com/api_keys",
  },
  {
    label: "Moonshot",
    type: "custom",
    baseUrl: "https://api.moonshot.cn/v1",
    models: [],
    category: "official",
    website: "https://platform.moonshot.cn/console/api-keys",
  },
  {
    label: "Zhipu GLM",
    type: "custom",
    baseUrl: "https://open.bigmodel.cn/api/paas/v4",
    models: [],
    category: "official",
    website: "https://open.bigmodel.cn/usercenter/apikeys",
  },
  {
    label: "Qwen",
    type: "custom",
    baseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1",
    models: ["qwen-max", "qwen-plus"],
    category: "official",
    website: "https://bailian.console.aliyun.com",
  },
  {
    label: "MiniMax",
    type: "custom",
    baseUrl: "https://api.minimaxi.com/v1",
    models: [],
    category: "official",
    website: "https://platform.minimaxi.com",
  },
  {
    label: "Baichuan",
    type: "custom",
    baseUrl: "https://api.baichuan-ai.com/v1",
    models: [],
    category: "official",
    website: "https://platform.baichuan-ai.com/console/apikey",
  },
  {
    label: "StepFun",
    type: "custom",
    baseUrl: "https://api.stepfun.com/v1",
    models: [],
    category: "official",
    website: "https://platform.stepfun.com/interface-key",
  },
  {
    label: "Volcengine Ark",
    type: "custom",
    baseUrl: "https://ark.cn-beijing.volces.com/api/v3",
    models: [],
    category: "official",
    website: "https://console.volcengine.com/ark",
  },
  {
    label: "Tencent Hunyuan",
    type: "custom",
    baseUrl: "https://api.hunyuan.cloud.tencent.com/v1",
    models: [],
    category: "official",
    website: "https://console.cloud.tencent.com/hunyuan/api-key",
  },
  {
    label: "Baidu ERNIE",
    type: "custom",
    baseUrl: "https://qianfan.baidubce.com/v2",
    models: [],
    category: "official",
    website: "https://console.bce.baidu.com/iam/#/iam/apikey/list",
  },
  {
    label: "01.AI",
    type: "custom",
    baseUrl: "https://api.lingyiwanwu.com/v1",
    models: [],
    category: "official",
    website: "https://platform.lingyiwanwu.com/apikeys",
  },
  {
    label: "AI21 Labs",
    type: "custom",
    baseUrl: "https://api.ai21.com/studio/v1",
    models: ["jamba-large", "jamba-mini"],
    category: "official",
    website: "https://studio.ai21.com/account/api-key",
  },
  {
    label: "Reka",
    type: "custom",
    baseUrl: "https://api.reka.ai/v1",
    models: ["reka-core", "reka-flash", "reka-edge"],
    category: "official",
    website: "https://platform.reka.ai",
  },
  {
    label: "SiliconFlow",
    type: "custom",
    baseUrl: "https://api.siliconflow.cn/v1",
    models: [],
    category: "platform",
    website: "https://cloud.siliconflow.cn/account/ak",
  },
  {
    label: "Groq",
    type: "custom",
    baseUrl: "https://api.groq.com/openai/v1",
    models: [],
    category: "platform",
    website: "https://console.groq.com/keys",
  },
  {
    label: "Together AI",
    type: "custom",
    baseUrl: "https://api.together.xyz/v1",
    models: [],
    category: "platform",
    website: "https://api.together.xyz/settings/api-keys",
  },
  {
    label: "Fireworks AI",
    type: "custom",
    baseUrl: "https://api.fireworks.ai/inference/v1",
    models: [],
    category: "platform",
    website: "https://fireworks.ai",
  },
  {
    label: "Novita AI",
    type: "custom",
    baseUrl: "https://api.novita.ai/v3/openai",
    models: [],
    category: "platform",
    website: "https://novita.ai/settings/key-management",
  },
  {
    label: "DeepInfra",
    type: "custom",
    baseUrl: "https://api.deepinfra.com/v1/openai",
    models: [],
    category: "platform",
    website: "https://deepinfra.com/dash/api_keys",
  },
  {
    label: "ModelScope",
    type: "custom",
    baseUrl: "https://api-inference.modelscope.cn/v1",
    models: [],
    category: "platform",
    website: "https://modelscope.cn/my/myaccesstoken",
  },
  {
    label: "PPIO",
    type: "custom",
    baseUrl: "https://api.ppinfra.com/v3/openai",
    models: [],
    category: "platform",
    website: "https://ppinfra.com/settings/key-management",
  },
  {
    label: "Cerebras",
    type: "custom",
    baseUrl: "https://api.cerebras.ai/v1",
    models: [],
    category: "platform",
    website: "https://cloud.cerebras.ai/platform",
  },
  {
    label: "SambaNova",
    type: "custom",
    baseUrl: "https://api.sambanova.ai/v1",
    models: [],
    category: "platform",
    website: "https://cloud.sambanova.ai/apis",
  },
  {
    label: "Hyperbolic",
    type: "custom",
    baseUrl: "https://api.hyperbolic.xyz/v1",
    models: [],
    category: "platform",
    website: "https://app.hyperbolic.xyz/settings",
  },
  {
    label: "Nebius AI Studio",
    type: "custom",
    baseUrl: "https://api.studio.nebius.ai/v1",
    models: [],
    category: "platform",
    website: "https://studio.nebius.ai",
  },
  {
    label: "Lambda",
    type: "custom",
    baseUrl: "https://api.lambda.ai/v1",
    models: [],
    category: "platform",
    website: "https://cloud.lambda.ai/api-keys",
  },
  {
    label: "Baseten",
    type: "custom",
    baseUrl: "https://inference.baseten.co/v1",
    models: [],
    category: "platform",
    website: "https://app.baseten.co/settings/api_keys",
  },
  {
    label: "NVIDIA NIM",
    type: "custom",
    baseUrl: "https://integrate.api.nvidia.com/v1",
    models: [],
    category: "platform",
    website: "https://build.nvidia.com",
  },
  {
    label: "Ollama",
    type: "custom",
    baseUrl: "http://localhost:11434/v1",
    models: [],
    category: "platform",
    website: "https://ollama.com",
  },
  {
    label: "LM Studio",
    type: "custom",
    baseUrl: "http://localhost:1234/v1",
    models: [],
    category: "platform",
    website: "https://lmstudio.ai",
  },
  {
    label: "vLLM",
    type: "custom",
    baseUrl: "http://localhost:8000/v1",
    models: [],
    category: "platform",
    website: "https://docs.vllm.ai",
  },
  {
    label: "llama.cpp",
    type: "custom",
    baseUrl: "http://localhost:8080/v1",
    models: [],
    category: "platform",
    website: "https://github.com/ggml-org/llama.cpp",
  },
  {
    label: "OpenRouter",
    type: "custom",
    baseUrl: "https://openrouter.ai/api/v1",
    models: [],
    category: "aggregator",
    website: "https://openrouter.ai/keys",
  },
  {
    label: "AiHubMix",
    type: "custom",
    baseUrl: "https://aihubmix.com/v1",
    models: [],
    category: "aggregator",
    website: "https://aihubmix.com/token",
  },
  {
    label: "302.AI",
    type: "custom",
    baseUrl: "https://api.302.ai/v1",
    models: [],
    category: "aggregator",
    website: "https://dash.302.ai",
  },
  {
    label: "Vercel AI Gateway",
    type: "custom",
    baseUrl: "https://ai-gateway.vercel.sh/v1",
    models: [],
    category: "aggregator",
    website: "https://vercel.com/docs/ai-gateway",
  },
  {
    label: "LiteLLM",
    type: "custom",
    baseUrl: "http://localhost:4000",
    models: [],
    category: "aggregator",
    website: "https://docs.litellm.ai",
  },
];

/** Where a new provider gets its credentials: one card of the picker. */
export interface ProviderSource {
  /** Stable id. The two cards that are not a vendor are titled by the picker. */
  key: string;
  /** The vendor's own name, empty when the picker titles the card itself. */
  label: string;
  /** Absent on the two cards that are not a vendor, which lead the picker. */
  category?: VendorCategory;
  /** Where the vendor hands out API keys. */
  website?: string;
  /** What the form starts from once the card is picked. */
  provider: Partial<Provider>;
}

export const subscriptionSource = "subscription";
export const customSource = "custom";

/**
 * The sign-in comes first: it is the only source that needs nothing filled in,
 * and the one people with a subscription and no API key are here for. Anthropic
 * is the vendor of the clients that mode works with; Codex signs in against an
 * API Gateway does not relay. Custom follows it, because the two of them are
 * the answers for a vendor that is not on the list at all.
 */
export const providerSources: ProviderSource[] = [
  {
    key: subscriptionSource,
    label: "",
    provider: {
      type: "anthropic",
      baseUrl: providerPresets.find(preset => preset.type === "anthropic")?.baseUrl ?? "",
      models: [],
      apiKey: "",
      authMode: authClient,
    },
  },
  {
    key: customSource,
    label: "",
    provider: {type: "custom", baseUrl: "", models: [], authMode: authProvider},
  },
  ...providerPresets.map(preset => ({
    key: preset.label.toLowerCase().replace(/[^a-z0-9]+/g, "-"),
    label: preset.label,
    category: preset.category,
    website: preset.website,
    provider: {
      type: preset.type,
      baseUrl: preset.baseUrl,
      models: preset.models,
      authMode: authProvider,
    },
  })),
];

/** The vendor a source came from, for the fields a picked card does not carry. */
export function presetOfSource(source: ProviderSource) {
  return providerPresets.find(preset => preset.label === source.label);
}

/** The base URLs and models offered for a provider type, from the vendors of it. */
export function modelPresets(type: string) {
  return providerPresets.filter(preset => preset.type === type).flatMap(preset => preset.models);
}

export function baseUrlPlaceholder(type: string) {
  return providerProtocol(type) === "anthropic" ? "https://api.anthropic.com" : "https://api.openai.com/v1";
}

export function modelsPlaceholder(type: string) {
  return providerProtocol(type) === "anthropic" ? "claude-opus-5, claude-sonnet-5" : "gpt-5, gpt-5-mini";
}
