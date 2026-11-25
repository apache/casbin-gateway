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

import {useEffect, useState} from "react";
import {ArrowDown, ArrowUp, GripVertical, X} from "lucide-react";
import i18next from "i18next";

import {ProviderIcon} from "@/components/ProviderIcon";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {providerIdOf} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {Provider, ProviderHealth} from "@/types";

const fallbackDragType = "application/x-casbin-fallback";

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
  const [dragged, setDragged] = useState("");
  const [target, setTarget] = useState("");
  const providersById = new Map(providers.map(provider => [providerIdOf(provider), provider]));
  const candidateIds = new Set(candidates.map(providerIdOf));
  const queued = new Set(fallbacks);
  const remaining = candidates.filter(provider => !queued.has(providerIdOf(provider)));
  const hasUnavailable = fallbacks.some(id => !candidateIds.has(id));
  const editingDisabled = busy || hasUnavailable;
  const title = i18next.t("agent:Fallback queue");
  const addLabel = i18next.t("agent:Add a fallback provider");

  const move = (from: number, to: number) => {
    if (editingDisabled || from < 0 || to < 0 || to >= fallbacks.length || from === to) {
      return;
    }
    const next = [...fallbacks];
    const [id] = next.splice(from, 1);
    next.splice(to, 0, id);
    onChange(next);
  };

  const canDrag = !editingDisabled && fallbacks.length > 1;
  const from = fallbacks.indexOf(dragged);
  const endDrag = () => {
    setDragged("");
    setTarget("");
  };

  useEffect(() => {
    if (dragged && (!canDrag || from < 0)) {
      endDrag();
    }
  }, [canDrag, dragged, from]);

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">{title}</h3>
      <p className="text-sm text-muted-foreground">
        {i18next.t(throughGateway ? "agent:Fallback queue hint" : "agent:Direct fallback queue hint")}
      </p>
      {fallbacks.length > 0 ? (
        <ol
          aria-label={title}
          className="space-y-1"
          onDragLeave={event => {
            if (!(event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget))) {
              setTarget("");
            }
          }}
        >
          {fallbacks.map((id, index) => {
            const provider = providersById.get(id);
            const label = provider?.displayName || provider?.name || id;
            const dropTarget = canDrag && target === id && from >= 0;
            const lastHealth = health.find(item => item.provider === id);
            const status = !provider ? "agent:Deleted fallback provider"
              : !candidateIds.has(id) ? "agent:Incompatible fallback provider"
                : provider.status !== "enabled" ? "provider:Disabled"
                  : lastHealth && !lastHealth.healthy ? "agent:Cooling down" : "";

            return (
              <li
                key={id}
                className={cn(
                  "flex items-center gap-2 rounded-md border p-1.5",
                  dropTarget && from < index && "border-b-primary",
                  dropTarget && from > index && "border-t-primary",
                )}
                onDragOver={event => {
                  if (canDrag && from >= 0 && event.dataTransfer.types.includes(fallbackDragType)) {
                    event.preventDefault();
                    event.dataTransfer.dropEffect = "move";
                    setTarget(id);
                  }
                }}
                onDrop={event => {
                  if (from >= 0 && event.dataTransfer.getData(fallbackDragType) === dragged) {
                    event.preventDefault();
                    move(from, index);
                    endDrag();
                  }
                }}
              >
                {canDrag && (
                  <span
                    draggable
                    aria-hidden="true"
                    title={i18next.t("agent:Drag to reorder")}
                    className="cursor-grab select-none p-1 text-muted-foreground active:cursor-grabbing"
                    onDragStart={event => {
                      event.dataTransfer.effectAllowed = "move";
                      event.dataTransfer.setData(fallbackDragType, id);
                      setDragged(id);
                    }}
                    onDragEnd={endDrag}
                  >
                    <GripVertical className="size-4" />
                  </span>
                )}
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
                      onClick={() => move(index, index - 1)}
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
                      onClick={() => move(index, index + 1)}
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
