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

import {CircleX, LogIn, Pencil, Trash2} from "lucide-react";
import i18next from "i18next";

import {ProviderIcon} from "@/components/ProviderIcon";
import {QuotaBadge} from "@/components/ProviderQuota";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {agentSpeaks} from "@/lib/agents";
import {providerIdOf, providerProtocol, servesAnyModel, usesClientAuth, usesSubscription} from "@/lib/providers";
import {cn} from "@/lib/utils";
import type {Agent, Provider, ProviderHealth, ProviderQuota} from "@/types";

/** How many model badges fit before the rest become a count. */
const visibleModels = 3;

/** The one mark that says whether the proxy is happy with this provider. */
function HealthDot({health}: {health: ProviderHealth | undefined}) {
  if (!health) {
    return (
      <SimpleTooltip title={i18next.t("provider:Not used yet")}>
        <span className="bg-muted-foreground/40 size-2 shrink-0 rounded-full" />
      </SimpleTooltip>
    );
  }

  const healthy = health.healthy && health.consecutive === 0;
  return (
    <SimpleTooltip
      title={
        health.healthy
          ? `${health.successes} / ${health.successes + health.failures}`
          : `${health.lastError} · ${i18next.t("provider:Retried at")} ${health.retryTime}`
      }
    >
      <span
        className={cn(
          "size-2 shrink-0 rounded-full",
          healthy ? "bg-success" : health.healthy ? "bg-muted-foreground" : "bg-warning",
        )}
      />
    </SimpleTooltip>
  );
}

/**
 * One provider, as a card rather than a row: the name, where it points, what it
 * serves, and the agents it answers for. Switching an agent over is what people
 * come to this page to do, so it is one click from here.
 */
export function ProviderGridCard({
  provider,
  agents,
  health,
  quota,
  busy,
  onEdit,
  onDelete,
  onBind,
}: {
  provider: Provider;
  agents: Agent[];
  health: ProviderHealth | undefined;
  quota: ProviderQuota | undefined;
  busy: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onBind: (agent: Agent) => void;
}) {
  const id = providerIdOf(provider);
  const protocol = providerProtocol(provider.type);
  const models = provider.models ?? [];
  const users = agents.filter(agent => agent.provider === id);
  // An agent bound directly to a provider reads its answers itself, so only one
  // serving the API that agent speaks is offered; through the gateway they all
  // are, since the proxy translates.
  const others = agents.filter(agent => agent.provider !== id && agentSpeaks(agent, protocol));

  return (
    <Card className="gap-0 py-0">
      <CardContent className="flex h-full flex-col gap-3 p-4">
        <div className="flex items-start gap-2">
          <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} alt={provider.name} size={22} />
          <div className="min-w-0 flex-1">
            <div className="flex min-w-0 items-center gap-2">
              <button
                type="button"
                onClick={onEdit}
                className="min-w-0 truncate text-left font-medium hover:underline"
              >
                {provider.displayName || provider.name}
              </button>
              <HealthDot health={health} />
            </div>
            <SimpleTooltip title={provider.baseUrl}>
              <p className="text-muted-foreground truncate font-mono text-xs">{provider.baseUrl || "-"}</p>
            </SimpleTooltip>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant={protocol === "anthropic" ? "info" : "success"}>{protocol}</Badge>
          {usesClientAuth(provider) ? (
            <Badge variant="muted">{i18next.t("provider:Caller's own login")}</Badge>
          ) : null}
          {/* A held sign-in is named after the account it spends: that, and not
              the base URL, is what tells two of them apart. */}
          {usesSubscription(provider) ? (
            <Badge variant="muted">
              <LogIn />
              {provider.subscriptionAccount || i18next.t("provider:Held sign-in")}
            </Badge>
          ) : null}
          {provider.status === "enabled" ? null : (
            <Badge variant="muted">
              <CircleX />
              {i18next.t("provider:Disabled")}
            </Badge>
          )}
          {/* A card has no column to fill, so a balance nobody can read is left out. */}
          {quota && (quota.error !== "" || quota.supported) ? <QuotaBadge quota={quota} /> : null}
        </div>

        <div className="flex flex-wrap gap-1">
          {models.length === 0 ? (
            <span className="text-muted-foreground text-xs">
              {servesAnyModel(provider) ? i18next.t("provider:Any model") : "-"}
            </span>
          ) : (
            <>
              {models.slice(0, visibleModels).map(model => (
                <Badge key={model} variant="muted">
                  {model}
                </Badge>
              ))}
              {models.length > visibleModels ? (
                <SimpleTooltip title={models.slice(visibleModels).join(", ")}>
                  <Badge variant="muted">{`+${models.length - visibleModels}`}</Badge>
                </SimpleTooltip>
              ) : null}
            </>
          )}
        </div>

        <p className="text-muted-foreground mt-auto truncate text-xs">
          {users.length === 0
            ? i18next.t("provider:Used by no agent")
            : `${i18next.t("provider:Used by")} ${users.map(agent => agent.name).join(", ")}`}
        </p>

        <div className="flex flex-wrap items-center gap-2 border-t pt-3">
          {others.length === 0 ? null : (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button size="sm" variant="outline" disabled={busy}>
                  {i18next.t("provider:Switch an agent over")}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start">
                <DropdownMenuLabel>{i18next.t("provider:Switch hint")}</DropdownMenuLabel>
                {others.map(agent => (
                  <DropdownMenuItem key={`${agent.agentId}/${agent.path}`} onSelect={() => onBind(agent)}>
                    {agent.name}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
          <div className="ml-auto flex gap-2">
            <SimpleTooltip title={i18next.t("general:Edit")}>
              <Button size="icon-sm" variant="ghost" aria-label={i18next.t("general:Edit")} onClick={onEdit}>
                <Pencil />
              </Button>
            </SimpleTooltip>
            <ConfirmDialog
              title={i18next
                .t("general:Sure to delete {name} ?")
                .replace("{name}", provider.displayName || provider.name)}
              confirmText={i18next.t("general:Delete")}
              onConfirm={onDelete}
            >
              <Button
                size="icon-sm"
                variant="ghost"
                className="text-destructive"
                aria-label={i18next.t("general:Delete")}
              >
                <Trash2 />
              </Button>
            </ConfirmDialog>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
