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

import {Bot, Download} from "lucide-react";
import i18next from "i18next";

import {AgentIcon} from "@/components/AgentIcon";
import {useAgentCatalog} from "@/lib/agents";
import type {Agent} from "@/types";

/**
 * The agents Gateway supports that this machine has none of, each linking to
 * the vendor's own install page. Without them a fresh machine says only that
 * nothing was found, which is the one thing it cannot act on.
 */
export function AgentCatalog({agents, enabled = true}: {agents: Agent[]; enabled?: boolean}) {
  const missing = useAgentCatalog(agents, enabled);

  if (missing.length === 0) {
    return null;
  }

  return (
    <section className="space-y-3">
      <div>
        <h2 className="font-medium">{i18next.t("agent:Not installed")}</h2>
        <p className="text-muted-foreground text-sm">{i18next.t("agent:Not installed hint")}</p>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
        {missing.map(item => {
          const inside = (
            <>
              {/* The icons are keyed by vendor name, which every entry has. */}
              <AgentIcon
                agent={item.name}
                size={22}
                fallback={<Bot className="text-muted-foreground size-5" />}
              />
              <span className="min-w-0 truncate text-sm font-medium">{item.name}</span>
              {item.installUrl ? (
                <span className="text-muted-foreground ml-auto inline-flex shrink-0 items-center gap-1 text-xs">
                  <Download className="size-3.5" />
                  {i18next.t("agent:Install")}
                </span>
              ) : null}
            </>
          );

          return item.installUrl ? (
            <a
              key={item.agentId}
              href={item.installUrl}
              target="_blank"
              rel="noreferrer"
              className="hover:border-primary hover:text-primary flex items-center gap-2.5 rounded-lg border p-3 transition-colors"
            >
              {inside}
            </a>
          ) : (
            <div
              key={item.agentId}
              className="text-muted-foreground flex items-center gap-2.5 rounded-lg border p-3"
            >
              {inside}
            </div>
          );
        })}
      </div>
    </section>
  );
}
