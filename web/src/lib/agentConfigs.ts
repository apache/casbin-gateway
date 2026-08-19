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

export interface AgentConfigValues {
  endpoint: string;
  values: Record<string, string>;
}

export interface AgentConfigPreset extends AgentConfigValues {
  id: string;
  name: string;
  description: string;
  connectionHint: string;
  tokenUrl?: string;
  icon: "provider" | "custom";
}

export interface AgentConfigDefinition {
  title: string;
  fields: {key: string; label: string; description: string}[];
  sectionTitle: string;
  sectionDescription: string;
  defaultPreset: string;
  presets: AgentConfigPreset[];
}

const claudeCode: AgentConfigDefinition = {
  title: "Configure Claude Code API",
  sectionTitle: "Model configuration",
  sectionDescription: "Map Claude Code model roles to models supported by the provider",
  defaultPreset: "deepseek",
  fields: [
    {key: "model", label: "Default model", description: "ANTHROPIC_MODEL"},
    {key: "haikuModel", label: "Haiku model", description: "ANTHROPIC_DEFAULT_HAIKU_MODEL"},
    {key: "sonnetModel", label: "Sonnet model", description: "ANTHROPIC_DEFAULT_SONNET_MODEL"},
    {key: "opusModel", label: "Opus model", description: "ANTHROPIC_DEFAULT_OPUS_MODEL"},
  ],
  presets: [
    {
      id: "deepseek",
      name: "DeepSeek",
      description: "Official Anthropic-compatible endpoint with recommended models",
      connectionHint: "DeepSeek endpoint and recommended models are prefilled; only the API token is required",
      tokenUrl: "https://platform.deepseek.com/api_keys",
      icon: "provider",
      endpoint: "https://api.deepseek.com/anthropic",
      values: {
        model: "deepseek-v4-pro",
        haikuModel: "deepseek-v4-flash",
        sonnetModel: "deepseek-v4-pro",
        opusModel: "deepseek-v4-pro",
      },
    },
    {
      id: "custom",
      name: "Custom",
      description: "Enter your own endpoint and model names",
      connectionHint: "Use an Anthropic-compatible API endpoint",
      icon: "custom",
      endpoint: "",
      values: {},
    },
  ],
};

const definitions: Record<string, AgentConfigDefinition> = {
  "claude-code": claudeCode,
};

export function getAgentConfigDefinition(agentId: string) {
  return definitions[agentId];
}
