export type AnthropicAuthType = "bearer" | "x-api-key";

export interface AnthropicChannelPreset {
  id: string;
  name: string;
  website: string;
  apiKeyUrl: string;
  baseUrl: string;
  authType: AnthropicAuthType;
  defaultModel: string;
  haikuModel: string;
  sonnetModel: string;
  opusModel: string;
}

// This is the single source used by both create and edit forms. Values follow
// the local cc-switch Claude presets; promotional query parameters are omitted.
export const anthropicChannelPresets: AnthropicChannelPreset[] = [
  {
    id: "custom",
    name: "Custom",
    website: "",
    apiKeyUrl: "",
    baseUrl: "",
    authType: "bearer",
    defaultModel: "",
    haikuModel: "",
    sonnetModel: "",
    opusModel: "",
  },
  {
    id: "anthropic",
    name: "Anthropic Official",
    website: "https://www.anthropic.com/claude-code",
    apiKeyUrl: "https://console.anthropic.com/settings/keys",
    baseUrl: "https://api.anthropic.com",
    authType: "x-api-key",
    defaultModel: "claude-sonnet-5",
    haikuModel: "claude-haiku-4-5-20251001",
    sonnetModel: "claude-sonnet-5",
    opusModel: "claude-opus-5",
  },
  {
    id: "deepseek",
    name: "DeepSeek",
    website: "https://platform.deepseek.com",
    apiKeyUrl: "https://platform.deepseek.com/api_keys",
    baseUrl: "https://api.deepseek.com/anthropic",
    authType: "bearer",
    defaultModel: "deepseek-v4-pro",
    haikuModel: "deepseek-v4-flash",
    sonnetModel: "deepseek-v4-pro",
    opusModel: "deepseek-v4-pro",
  },
  {
    id: "kimi-for-coding",
    name: "Kimi For Coding",
    website: "https://www.kimi.com/code/",
    apiKeyUrl: "https://www.kimi.com/code/",
    baseUrl: "https://api.kimi.com/coding/",
    authType: "bearer",
    defaultModel: "kimi-for-coding",
    haikuModel: "kimi-for-coding",
    sonnetModel: "kimi-for-coding",
    opusModel: "kimi-for-coding",
  },
  {
    id: "minimax",
    name: "MiniMax",
    website: "https://platform.minimaxi.com",
    apiKeyUrl: "https://platform.minimaxi.com/subscribe/coding-plan",
    baseUrl: "https://api.minimaxi.com/anthropic",
    authType: "bearer",
    defaultModel: "MiniMax-M2.7",
    haikuModel: "MiniMax-M2.7",
    sonnetModel: "MiniMax-M2.7",
    opusModel: "MiniMax-M2.7",
  },
  {
    id: "zhipu",
    name: "Zhipu GLM",
    website: "https://open.bigmodel.cn",
    apiKeyUrl: "https://bigmodel.cn/usercenter/apikeys",
    baseUrl: "https://open.bigmodel.cn/api/anthropic",
    authType: "x-api-key",
    defaultModel: "glm-5.3",
    haikuModel: "glm-4.7",
    sonnetModel: "glm-5.3",
    opusModel: "glm-5.3",
  },
  {
    id: "openrouter",
    name: "OpenRouter",
    website: "https://openrouter.ai",
    apiKeyUrl: "https://openrouter.ai/keys",
    baseUrl: "https://openrouter.ai/api",
    authType: "bearer",
    defaultModel: "anthropic/claude-sonnet-5",
    haikuModel: "anthropic/claude-haiku-4.5",
    sonnetModel: "anthropic/claude-sonnet-5",
    opusModel: "anthropic/claude-opus-5",
  },
  {
    id: "longcat",
    name: "LongCat",
    website: "https://longcat.chat/platform",
    apiKeyUrl: "https://longcat.chat/platform/api_keys",
    baseUrl: "https://api.longcat.chat/anthropic",
    authType: "bearer",
    defaultModel: "LongCat-2.0",
    haikuModel: "LongCat-2.0",
    sonnetModel: "LongCat-2.0",
    opusModel: "LongCat-2.0",
  },
];

export function anthropicPreset(id: string) {
  return anthropicChannelPresets.find(preset => preset.id === id);
}
