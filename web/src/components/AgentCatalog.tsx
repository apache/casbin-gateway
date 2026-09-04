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

import i18next from "i18next";

import {ToolInstallRow} from "@/components/ToolInstallRow";
import {useAgentCatalog, type useAgentInstall} from "@/lib/agents";
import type {Agent} from "@/types";

/**
 * The agents Gateway supports that this machine has none of, each with the
 * click that installs it here. Without them a fresh machine says only that
 * nothing was found, which is the one thing it cannot act on.
 */
export function AgentCatalog({
  agents,
  enabled = true,
  installer,
  onLocated,
}: {
  agents: Agent[];
  enabled?: boolean;
  /** Called once a program is picked, which is when a rescan drops its row. */
  onLocated?: () => void;
  /**
   * The page's own installer. It is passed in rather than made here: the page
   * upgrades the agents it already has through the same one, and two of them
   * would report the same finished job twice.
   */
  installer: ReturnType<typeof useAgentInstall>;
}) {
  const missing = useAgentCatalog(agents, enabled);
  const {jobs, busyId, install} = installer;

  if (missing.length === 0) {
    return null;
  }

  return (
    <section className="space-y-3">
      <div>
        <h2 className="text-sm font-medium">{i18next.t("agent:Not installed")}</h2>
        <p className="text-muted-foreground text-xs">{i18next.t("agent:Not installed hint")}</p>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {missing.map(item => (
          <ToolInstallRow
            key={item.agentId}
            entry={item}
            job={jobs[item.agentId]}
            busy={busyId === item.agentId}
            onInstall={install}
            onLocated={onLocated}
          />
        ))}
      </div>
    </section>
  );
}
