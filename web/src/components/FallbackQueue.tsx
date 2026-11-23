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

import {ArrowDown, ArrowUp, X} from "lucide-react";
import i18next from "i18next";

import {ProviderIcon} from "@/components/ProviderIcon";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {providerIdOf} from "@/lib/providers";
import type {Provider, ProviderHealth} from "@/types";

export function FallbackQueue({
  fallbacks,
  providers,
  candidates,
  health,
  throughGateway,
  busy,
  onChange,
}: {
  fallbacks: string[];
  providers: Provider[];
  candidates: Provider[];
  health: ProviderHealth[];
  throughGateway: boolean;
  busy: boolean;
  onChange: (fallbacks: string[]) => void;
}) {
  const providersById = new Map(providers.map(provider => [providerIdOf(provider), provider]));
  const candidateIds = new Set(candidates.map(providerIdOf));
  const queued = new Set(fallbacks);
  const remaining = candidates.filter(provider => !queued.has(providerIdOf(provider)));
  const hasUnavailable = fallbacks.some(id => !candidateIds.has(id));
  const editingDisabled = busy || hasUnavailable;
  const title = i18next.t("agent:Fallback queue");
  const addLabel = i18next.t("agent:Add a fallback provider");

  const move = (index: number, offset: number) => {
    const next = [...fallbacks];
    const [id] = next.splice(index, 1);
    next.splice(index + offset, 0, id);
    onChange(next);
  };

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">{title}</h3>
      <p className="text-sm text-muted-foreground">
        {i18next.t(throughGateway ? "agent:Fallback queue hint" : "agent:Direct fallback queue hint")}
      </p>
      {fallbacks.length > 0 ? (
        <ol aria-label={title} className="space-y-1">
          {fallbacks.map((id, index) => {
            const provider = providersById.get(id);
            const label = provider?.displayName || provider?.name || id;
            const lastHealth = health.find(item => item.provider === id);
            const status = !provider ? "agent:Deleted fallback provider"
              : !candidateIds.has(id) ? "agent:Incompatible fallback provider"
                : provider.status !== "enabled" ? "provider:Disabled"
                  : lastHealth && !lastHealth.healthy ? "agent:Cooling down" : "";

            return (
              <li key={id} className="flex items-center gap-2 rounded-md border p-1.5">
                <span aria-hidden="true" className="w-5 shrink-0 text-center text-xs tabular-nums text-muted-foreground">
                  {index + 1}
                </span>
                {provider ? <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} size={16} /> : null}
                <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2">
                  <span className="max-w-full truncate text-sm" title={label}>{label}</span>
                  {status ? <Badge variant="warning">{i18next.t(status)}</Badge> : null}
                </div>
                <div className="flex shrink-0 items-center">
                  <SimpleTooltip title={i18next.t("llm:Move up")}>
                    <Button
                      type="button"
                      size="icon-sm"
                      variant="ghost"
                      aria-label={i18next.t("agent:Move {provider} up").replace("{provider}", label)}
                      disabled={editingDisabled || index === 0}
                      onClick={() => move(index, -1)}
                    >
                      <ArrowUp />
                    </Button>
                  </SimpleTooltip>
                  <SimpleTooltip title={i18next.t("llm:Move down")}>
                    <Button
                      type="button"
                      size="icon-sm"
                      variant="ghost"
                      aria-label={i18next.t("agent:Move {provider} down").replace("{provider}", label)}
                      disabled={editingDisabled || index === fallbacks.length - 1}
                      onClick={() => move(index, 1)}
                    >
                      <ArrowDown />
                    </Button>
                  </SimpleTooltip>
                  <SimpleTooltip title={i18next.t("agent:Remove fallback")}>
                    <Button
                      type="button"
                      size="icon-sm"
                      variant="ghost"
                      aria-label={i18next.t("agent:Remove {provider} from the fallback queue").replace("{provider}", label)}
                      disabled={editingDisabled}
                      onClick={() => onChange(fallbacks.filter(item => item !== id))}
                    >
                      <X />
                    </Button>
                  </SimpleTooltip>
                </div>
              </li>
            );
          })}
        </ol>
      ) : null}
      {hasUnavailable ? (
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={busy}
          onClick={() => onChange(fallbacks.filter(id => candidateIds.has(id)))}
        >
          {i18next.t("agent:Remove unavailable fallbacks")}
        </Button>
      ) : remaining.length > 0 ? (
        <Select value="" disabled={busy} onValueChange={id => onChange([...fallbacks, id])}>
          <SelectTrigger aria-label={addLabel} className="w-full max-w-xs" size="sm">
            <SelectValue placeholder={addLabel} />
          </SelectTrigger>
          <SelectContent>
            {remaining.map(provider => (
              <SelectItem key={providerIdOf(provider)} value={providerIdOf(provider)}>
                <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} size={16} />
                {provider.displayName || provider.name}
                {provider.status !== "enabled" ? (
                  <span className="text-xs text-muted-foreground">{i18next.t("provider:Disabled")}</span>
                ) : null}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : candidates.length === 0 ? (
        <p className="text-sm text-muted-foreground">{i18next.t("agent:No other provider speaks this API")}</p>
      ) : null}
    </div>
  );
}
