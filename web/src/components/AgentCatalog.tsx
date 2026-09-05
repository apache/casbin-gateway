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

import {Fold} from "@/components/shared/fold";
import {ToolInstallRow} from "@/components/ToolInstallRow";
import {useAgentCatalog, type useAgentInstall} from "@/lib/agents";
import type {Agent} from "@/types";

/**
 * The agents Gateway supports that this machine has none of, each with the
 * click that installs it here. Without them a fresh machine says only that
 * nothing was found, which is the one thing it cannot act on - but on a machine
 * that already has agents it is a dozen cards about software nobody asked for,
 * so it folds down to the line that names it.
 */
export function AgentCatalog({
  agents,
  enabled = true,
  defaultOpen = true,
  installer,
  onLocated,
}: {
  agents: Agent[];
  enabled?: boolean;
  /** False where the page has more to say than this, which is the home screen. */
  defaultOpen?: boolean;
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
    <Fold
      defaultOpen={defaultOpen}
      title={i18next
        .t("agent:{count} agents not installed")
        .replace("{count}", String(missing.length))}
      description={i18next.t("agent:Not installed hint")}
    >
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
    </Fold>
  );
}
