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
import {Bot, Cloud, Folder, Globe, Settings2} from "lucide-react";

import {AgentIcon} from "@/components/AgentIcon";
import {ProviderIcon} from "@/components/ProviderIcon";
import type {SelectOption} from "@/components/shared/simple-select";
import {providerPresets} from "@/lib/providers";

/** A dropdown item that names a brand is read by its mark first. */
function brandOption(value: string, text: string, mark: React.ReactNode): SelectOption {
  return {
    value,
    text,
    label: (
      <span className="flex items-center gap-2 truncate">
        {mark}
        {text}
      </span>
    ),
  };
}

/** An agent, named by the id its records carry or by its own display name. */
export function agentOption(agent: string, label?: string): SelectOption {
  return brandOption(
    agent,
    label ?? agent,
    <AgentIcon agent={agent} size={16} fallback={<Bot className="size-4 opacity-50" />} />,
  );
}

/** A vendor, named by the site its mark is served from. */
export function vendorOption(value: string, label: string, site: string): SelectOption {
  return brandOption(value, label, <ProviderIcon icon={site} size={16} />);
}

/** The wire format a provider serves, which is a vendor's API either way. */
export const protocolOptions: SelectOption[] = [
  vendorOption("openai", "OpenAI", "openai.com"),
  vendorOption("anthropic", "Anthropic", "anthropic.com"),
];

/** The Type field of both provider forms: two vendors and everything else. */
export const providerTypeOptions: SelectOption[] = [
  ...protocolOptions,
  brandOption("custom", "Custom", <Settings2 className="size-4 opacity-50" />),
];

/**
 * A storage cloud sync writes to. Only S3 is a brand: a folder is a folder, and
 * WebDAV is a protocol nobody draws a logo for, so those two are shown as what
 * they are.
 */
const storageMarks: Record<string, React.ReactNode> = {
  folder: <Folder className="size-4 opacity-50" />,
  s3: <ProviderIcon icon="aws.amazon.com" size={16} />,
  webdav: <Globe className="size-4 opacity-50" />,
};

export function storageOption(kind: string, label: string): SelectOption {
  return brandOption(kind, label, storageMarks[kind] ?? <Cloud className="size-4 opacity-50" />);
}

/** The base URLs a type's vendors answer on, each behind its own mark. The
 *  mark comes from the vendor's own site: an API host often has no icon. */
export function baseUrlOptions(type: string): SelectOption[] {
  return providerPresets
    .filter(preset => preset.type === type)
    .map(preset =>
      brandOption(
        preset.baseUrl,
        preset.baseUrl,
        <ProviderIcon icon={preset.website} baseUrl={preset.baseUrl} size={16} />,
      ),
    );
}
