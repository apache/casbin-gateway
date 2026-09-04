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
import {Plug, Search} from "lucide-react";
import i18next from "i18next";

import {ProviderIcon} from "@/components/ProviderIcon";
import {Field} from "@/components/shared/form-dialog";
import {PasswordInput} from "@/components/shared/password-input";
import {Input} from "@/components/ui/input";
import {
  mcpCategories,
  mcpPresets,
  presetSummary,
  type McpCategory,
  type McpInputKind,
  type McpPreset,
} from "@/lib/mcp";

export function categoryTitle(category: McpCategory) {
  return {
    essential: i18next.t("agentConfig:Everyday servers"),
    dev: i18next.t("agentConfig:Development"),
    data: i18next.t("agentConfig:Databases and data"),
    web: i18next.t("agentConfig:Search and the web"),
    work: i18next.t("agentConfig:Team tools"),
  }[category];
}

function PresetCard({preset, onPick}: {preset: McpPreset; onPick: (preset: McpPreset) => void}) {
  return (
    <button
      type="button"
      onClick={() => onPick(preset)}
      className="hover:border-primary hover:bg-accent/40 flex flex-col items-start gap-1 rounded-lg border p-3 text-left transition-colors"
    >
      <span className="flex w-full min-w-0 items-center gap-2 text-sm font-medium">
        <ProviderIcon baseUrl={preset.website} size={16} fallback={<Plug className="size-4 shrink-0" />} />
        <span className="truncate">{preset.label}</span>
      </span>
      <span className="text-muted-foreground w-full truncate font-mono text-xs">{presetSummary(preset)}</span>
    </button>
  );
}

/**
 * The catalogue half of adding a server: the servers people actually install,
 * offered by name so that a working install is a click and a key rather than a
 * command line copied out of a README.
 */
export function McpPresetPicker({onPick}: {onPick: (preset: McpPreset) => void}) {
  const [query, setQuery] = React.useState("");

  const needle = query.trim().toLowerCase();
  const matches = (preset: McpPreset) =>
    needle === "" ||
    preset.label.toLowerCase().includes(needle) ||
    preset.key.includes(needle) ||
    presetSummary(preset).toLowerCase().includes(needle);

  const groups = mcpCategories
    .map(category => ({
      category: category,
      presets: mcpPresets.filter(preset => preset.category === category).filter(matches),
    }))
    .filter(group => group.presets.length > 0);

  return (
    <div className="grid gap-4">
      <div className="relative">
        <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
        <Input
          value={query}
          onChange={event => setQuery(event.target.value)}
          placeholder={i18next.t("agentConfig:Search servers")}
          className="pl-9"
        />
      </div>

      {groups.map(group => (
        <div key={group.category} className="grid gap-2">
          <span className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
            {categoryTitle(group.category)}
          </span>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {group.presets.map(preset => (
              <PresetCard key={preset.key} preset={preset} onPick={onPick} />
            ))}
          </div>
        </div>
      ))}

      {groups.length === 0 ? (
        <p className="text-muted-foreground py-4 text-center text-sm">
          {i18next.t("agentConfig:No server matches")}
        </p>
      ) : null}
    </div>
  );
}

function inputLabel(kind: McpInputKind) {
  return {
    key: i18next.t("agentConfig:API key"),
    token: i18next.t("agentConfig:Access token"),
    path: i18next.t("agentConfig:Path"),
    url: i18next.t("agentConfig:Connection URL"),
    id: i18next.t("agentConfig:Workspace ID"),
  }[kind];
}

/** The one or two values a picked preset still needs, and nothing else. */
export function McpPresetInputs({
  preset,
  values,
  disabled,
  onChange,
}: {
  preset: McpPreset;
  values: Record<string, string>;
  disabled?: boolean;
  onChange: (key: string, value: string) => void;
}) {
  return (
    <>
      {(preset.inputs ?? []).map(input => {
        const secret = input.kind === "key" || input.kind === "token";
        const Control = secret ? PasswordInput : Input;
        return (
          <Field
            key={input.key}
            label={inputLabel(input.kind)}
            htmlFor={`mcp-input-${input.key}`}
            required
            hint={
              secret ? (
                <a href={preset.website} target="_blank" rel="noreferrer" className="hover:underline">
                  {i18next.t("agentConfig:Where to get it")}
                </a>
              ) : null
            }
          >
            <Control
              id={`mcp-input-${input.key}`}
              value={values[input.key] ?? ""}
              placeholder={input.placeholder}
              disabled={disabled}
              onChange={event => onChange(input.key, event.target.value)}
            />
          </Field>
        );
      })}
    </>
  );
}
