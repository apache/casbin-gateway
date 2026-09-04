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

import {Bot, ExternalLink, FolderSearch} from "lucide-react";
import i18next from "i18next";

import {AgentIcon} from "@/components/AgentIcon";
import {AgentPathDialog} from "@/components/AgentPathDialog";
import {InstallJobProgress} from "@/components/AgentInstallJob";
import {AgentInstallButton} from "@/components/ToolUpgradeConfirmDialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {AgentCatalogEntry, AgentInstallJob} from "@/types";

/**
 * One agent this machine does not have, with the click that installs it. The
 * install runs on the host with the package manager that is already there, so
 * the row shows which one it will use and what that manager is printing; an
 * agent no manager here publishes keeps the link to its vendor, which is then
 * all there is.
 *
 * That link is a link and comes last in every card: an install runs a command
 * here, the vendor's page leaves for a website, and an orange button each told
 * them apart by nothing but an icon.
 *
 * An agent put somewhere no layout describes lands here too, so the card also
 * offers the picker that points Gateway at its program.
 */
export function ToolInstallRow({
  entry,
  job,
  busy,
  onInstall,
  onLocated,
}: {
  entry: AgentCatalogEntry;
  /** The running or last install of this agent, when there was one. */
  job?: AgentInstallJob;
  busy: boolean;
  onInstall: (agentId: string) => void;
  /** Called once a program is picked, which is when a rescan will list it. */
  onLocated?: () => void;
}) {
  const plan = entry.install;
  const failed = job !== undefined && !job.running && !job.ok;

  return (
    <div className="bg-card hover:border-foreground/25 space-y-2 rounded-xl border p-3 shadow-xs transition-colors">
      <div className="flex items-center gap-2.5">
        {/* The icons are keyed by vendor name, which every entry has. */}
        <AgentIcon
          agent={entry.name}
          size={20}
          fallback={<Bot className="text-muted-foreground size-5" />}
        />
        <span className="min-w-0 flex-1 truncate text-sm font-medium">{entry.name}</span>

        {plan.available ? (
          <span className="shrink-0">
            <AgentInstallButton
              name={entry.name}
              plan={plan}
              job={job}
              busy={busy}
              label={failed ? i18next.t("agent:Retry") : undefined}
              onInstall={() => onInstall(entry.agentId)}
            />
          </span>
        ) : null}

        {entry.installUrl ? (
          <SimpleTooltip title={i18next.t("agent:Install page")}>
            <a
              href={entry.installUrl}
              target="_blank"
              rel="noreferrer"
              className="text-muted-foreground hover:text-primary shrink-0"
              aria-label={i18next.t("agent:Install page")}
            >
              <ExternalLink className="size-4" />
            </a>
          </SimpleTooltip>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {plan.available ? (
          <Badge variant="muted" className="text-[11px]">{plan.manager}</Badge>
        ) : (
          <span className="text-muted-foreground text-[11px]">{plan.detail}</span>
        )}
        {plan.available ? (
          <code className="text-muted-foreground/80 min-w-0 truncate text-[11px]">{plan.command}</code>
        ) : null}

        <span className="ml-auto shrink-0">
          <AgentPathDialog
            agentId={entry.agentId}
            name={entry.name}
            onAdded={onLocated}
            trigger={
              <Button size="xs" variant="ghost" className="text-muted-foreground">
                <FolderSearch />
                {i18next.t("agent:Locate")}
              </Button>
            }
          />
        </span>
      </div>

      <InstallJobProgress job={job} />
    </div>
  );
}
