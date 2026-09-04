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

import {FolderSearch, Play, Square} from "lucide-react";
import i18next from "i18next";

import {AgentPathDialog} from "@/components/AgentPathDialog";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import type {Agent, AgentRuntime} from "@/types";

/**
 * Why an installation is not running. A missing program is the one answer the
 * user can act on, so it is said in their own language rather than passed
 * through from the host.
 */
function stoppedHint(status: AgentRuntime) {
  return status.canStart ? status.detail : i18next.t("agent:No program hint");
}

/** Whether the installation has live processes, with its pids on hover. */
export function RunBadge({status}: {status?: AgentRuntime}) {
  if (!status) {
    return <span className="text-muted-foreground">-</span>;
  }
  if (!status.running) {
    return (
      <SimpleTooltip title={stoppedHint(status)}>
        <span>
          <Badge variant="muted">{i18next.t("agent:Stopped")}</Badge>
        </span>
      </SimpleTooltip>
    );
  }
  return (
    <SimpleTooltip title={`${i18next.t("agent:Processes")}: ${status.pids.join(", ")}`}>
      <span>
        <Badge variant="success">{i18next.t("agent:Running")}</Badge>
      </span>
    </SimpleTooltip>
  );
}

/**
 * The same state as a dot, for the cards, where a filled badge shouts and there
 * is no width for a word. The button below it says which of the two it is in
 * full, so the dot is the glance rather than the only telling.
 */
export function RunDot({status}: {status?: AgentRuntime}) {
  if (!status) {
    return null;
  }
  const state = i18next.t(status.running ? "agent:Running" : "agent:Stopped");
  const detail = status.running
    ? `${i18next.t("agent:Processes")}: ${status.pids.join(", ")}`
    : stoppedHint(status);

  return (
    <SimpleTooltip title={detail ? `${state} · ${detail}` : state}>
      <span
        className={cn(
          "mt-1.5 size-1.5 shrink-0 rounded-full",
          status.running ? "bg-success ring-success/20 ring-2" : "bg-muted-foreground/40",
        )}
      />
    </SimpleTooltip>
  );
}

/**
 * The start/stop control. Starting is harmless enough to happen on the click,
 * while stopping ends work in progress and is confirmed first.
 *
 * An installation found by the state directory it left behind has no program to
 * run, and a greyed-out button is a dead end there: it offers the picker that
 * points Gateway at the program instead.
 */
export function RunButton({
  agent,
  status,
  busy,
  className,
  onLocated,
  onToggle,
}: {
  agent: Agent;
  status?: AgentRuntime;
  busy: boolean;
  /** The cards run it smaller than a table row does. */
  className?: string;
  /** Called once a program is picked, which is when a rescan can start it. */
  onLocated?: () => void;
  onToggle: (agent: Agent, running: boolean) => void;
}) {
  if (status?.running) {
    return (
      <ConfirmDialog
        title={`${i18next.t("agent:Stop")} ${agent.name}?`}
        description={i18next.t("agent:Stop hint")}
        confirmText={i18next.t("agent:Stop")}
        onConfirm={() => onToggle(agent, true)}
      >
        <Button size="sm" variant="outline" className={className} loading={busy}>
          <Square />
          {i18next.t("agent:Stop")}
        </Button>
      </ConfirmDialog>
    );
  }

  // Only once the status is in: until then nothing is known about a launcher.
  if (status !== undefined && !status.canStart) {
    return (
      <SimpleTooltip title={i18next.t("agent:No program hint")}>
        <span>
          <AgentPathDialog
            agentId={agent.agentId}
            name={agent.name}
            onAdded={onLocated}
            trigger={
              <Button size="sm" variant="outline" className={className}>
                <FolderSearch />
                {i18next.t("agent:Locate")}
              </Button>
            }
          />
        </span>
      </SimpleTooltip>
    );
  }

  return (
    <Button
      size="sm"
      variant="outline"
      className={className}
      disabled={status === undefined}
      loading={busy}
      onClick={() => onToggle(agent, false)}
    >
      <Play />
      {i18next.t("agent:Start")}
    </Button>
  );
}
