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

import * as AgentBackend from "@/backend/AgentBackend";
import type {AgentEgress, AgentRecord, EgressDetail} from "@/types";

const refreshMs = 10000;

/** What the egress watch has on one agent, read again while the page is open
 *  since the destinations change as the agent runs. */
export function useAgentEgress(agentId: string, owner: string, enabled: boolean) {
  const [egress, setEgress] = React.useState<AgentEgress | null>(null);

  React.useEffect(() => {
    if (!enabled || agentId === "") {
      setEgress(null);
      return;
    }

    let cancelled = false;
    const load = () => {
      AgentBackend.getAgentEgress(agentId, owner)
        .then(res => {
          if (!cancelled && res.status === "ok") {
            setEgress(res.data ?? null);
          }
        })
        .catch(() => undefined);
    };
    load();
    const timer = window.setInterval(load, refreshMs);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [agentId, owner, enabled]);

  return egress;
}

/** The body of an egress record, null for one this page cannot read. */
export function egressDetailOf(record: AgentRecord): EgressDetail | null {
  if (!record.detail) {
    return null;
  }
  try {
    return JSON.parse(record.detail) as EgressDetail;
  } catch {
    return null;
  }
}

/** Whether there is anything to show at all: a platform the watch does not run
 *  on still reports what an agent left on disk. */
export function hasEgress(egress: AgentEgress | null): egress is AgentEgress {
  return (
    egress !== null &&
    (egress.supported || egress.snapshots.length > 0 || egress.findings.length > 0)
  );
}
