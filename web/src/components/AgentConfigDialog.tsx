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
import {ExternalLink, Settings2, Sparkles} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {PasswordInput} from "@/components/ui/password-input";
import {Spinner} from "@/components/ui/spinner";
import {
  getAgentConfigDefinition,
  type AgentConfigDefinition,
  type AgentConfigPreset,
  type AgentConfigValues,
} from "@/lib/agentConfigs";
import {cn} from "@/lib/utils";
import type {Agent} from "@/types";

export function AgentConfigDialog({
  agent,
  onClose,
  onSaved,
}: {
  agent: Agent;
  onClose: () => void;
  onSaved: () => void;
}) {
  const definition = getAgentConfigDefinition(agent.agentId);
  if (!definition) {
    return null;
  }

  return <AgentConfigDialogContent agent={agent} definition={definition} onClose={onClose} onSaved={onSaved} />;
}

function AgentConfigDialogContent({
  agent,
  definition,
  onClose,
  onSaved,
}: {
  agent: Agent;
  definition: AgentConfigDefinition;
  onClose: () => void;
  onSaved: () => void;
}) {
  const editing = agent.configStatus?.configured === true;

  const initialPreset = agent.configStatus?.restorable
    ? definition.presets.find(preset => preset.endpoint && preset.endpoint === agent.configStatus?.endpoint) ??
      definition.presets.find(preset => preset.id === "custom")!
    : definition.presets.find(preset => preset.id === definition.defaultPreset)!;
  const [presetId, setPresetId] = React.useState(initialPreset.id);
  const [config, setConfig] = React.useState<AgentConfigValues>(() =>
    agent.configStatus?.restorable
      ? {
        endpoint: agent.configStatus.endpoint ?? "",
        values: Object.fromEntries(
          definition.fields.map(field => [field.key, agent.configStatus?.values?.[field.key] ?? ""]),
        ),
      }
      : presetValues(initialPreset, definition.fields),
  );
  const [token, setToken] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const preset = definition.presets.find(item => item.id === presetId) ?? initialPreset;

  const selectPreset = (selected: AgentConfigPreset) => {
    setPresetId(selected.id);
    setConfig(presetValues(selected, definition.fields));
  };

  const save = () => {
    if (!config.endpoint.trim()) {
      Setting.showMessage("error", i18next.t("agent:API endpoint is required"));
      return;
    }
    if (!editing && !token.trim()) {
      Setting.showMessage("error", i18next.t("agent:API token is required"));
      return;
    }

    setSaving(true);
    AgentBackend.configureAgentApi(
      {agentId: agent.agentId, path: agent.path, owner: agent.owner},
      {
        endpoint: config.endpoint.trim(),
        token: token.trim(),
        values: Object.fromEntries(Object.entries(config.values).map(([key, value]) => [key, value.trim()])),
      },
    )
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("agent:API configuration updated"));
          onSaved();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agent:Failed to update API configuration"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setSaving(false));
  };

  return (
    <Dialog open onOpenChange={open => !open && onClose()}>
      <DialogContent className="max-h-[90vh] grid-rows-[auto_minmax(0,1fr)_auto] sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{i18next.t(`agent:${definition.title}`)}</DialogTitle>
          <DialogDescription>
            {i18next.t("agent:Choose a provider preset, then review and save the configuration")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-6 overflow-y-auto pr-1">
          <section className="space-y-3">
            <Label>{i18next.t("agent:Provider preset")}</Label>
            <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3">
              {definition.presets.map(item => (
                <ProviderButton
                  key={item.id}
                  preset={item}
                  selected={presetId === item.id}
                  onClick={() => selectPreset(item)}
                />
              ))}
            </div>
          </section>

          <section className="space-y-4 rounded-lg border p-4">
            <div>
              <h3 className="font-medium">{i18next.t("agent:Connection")}</h3>
              <p className="mt-1 text-sm text-muted-foreground">{i18next.t(`agent:${preset.connectionHint}`)}</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="agent-config-endpoint">{i18next.t("agent:API Endpoint")}</Label>
              <Input
                id="agent-config-endpoint"
                value={config.endpoint}
                placeholder="https://api.example.com"
                onChange={event => setConfig(current => ({...current, endpoint: event.target.value}))}
              />
            </div>
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3">
                <Label htmlFor="agent-config-token">{i18next.t("agent:API Token")}</Label>
                {preset.tokenUrl ? (
                  <a
                    href={preset.tokenUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-primary hover:underline"
                  >
                    {i18next.t("agent:Get API token")}
                    <ExternalLink className="h-3 w-3" />
                  </a>
                ) : null}
              </div>
              <PasswordInput
                id="agent-config-token"
                value={token}
                placeholder={i18next.t(editing ? "agent:Leave blank to keep current token" : "agent:Enter API token")}
                onChange={event => setToken(event.target.value)}
              />
            </div>
          </section>

          {definition.fields.length > 0 ? (
            <section className="space-y-4 rounded-lg border p-4">
              <div>
                <h3 className="font-medium">{i18next.t(`agent:${definition.sectionTitle}`)}</h3>
                <p className="mt-1 text-sm text-muted-foreground">
                  {i18next.t(`agent:${definition.sectionDescription}`)}
                </p>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                {definition.fields.map(field => (
                  <div key={field.key} className="space-y-2">
                    <Label htmlFor={`agent-config-${field.key}`}>{i18next.t(`agent:${field.label}`)}</Label>
                    <Input
                      id={`agent-config-${field.key}`}
                      value={config.values[field.key] ?? ""}
                      placeholder={field.description}
                      onChange={event =>
                        setConfig(current => ({
                          ...current,
                          values: {...current.values, [field.key]: event.target.value},
                        }))
                      }
                    />
                    <p className="text-xs text-muted-foreground">{field.description}</p>
                  </div>
                ))}
              </div>
            </section>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            {i18next.t("general:Cancel")}
          </Button>
          <Button onClick={save} disabled={saving}>
            {saving ? <Spinner /> : null}
            {i18next.t("general:Save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function presetValues(
  preset: AgentConfigPreset,
  fields: {key: string}[],
): AgentConfigValues {
  return {
    endpoint: preset.endpoint,
    values: Object.fromEntries(fields.map(field => [field.key, preset.values[field.key] ?? ""])),
  };
}

function ProviderButton({
  preset,
  selected,
  onClick,
}: {
  preset: AgentConfigPreset;
  selected: boolean;
  onClick: () => void;
}) {
  const icon =
    preset.icon === "custom" ? <Settings2 className="h-5 w-5" /> : <Sparkles className="h-5 w-5" />;

  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      className={cn(
        "flex items-start gap-3 rounded-lg border p-4 text-left transition-colors",
        selected ? "border-primary bg-primary/5 ring-1 ring-primary" : "hover:border-primary/50 hover:bg-muted/40",
      )}
    >
      <span className={cn("rounded-md p-2", selected ? "bg-primary text-primary-foreground" : "bg-muted")}>
        {icon}
      </span>
      <span>
        <span className="block font-medium">{i18next.t(`agent:${preset.name}`)}</span>
        <span className="mt-1 block text-sm text-muted-foreground">
          {i18next.t(`agent:${preset.description}`)}
        </span>
      </span>
    </button>
  );
}
