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

import {ArrowUpCircle} from "lucide-react";
import i18next from "i18next";

import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {Agent, AgentInstallJob} from "@/types";

/** The tail of what a package manager printed, while it works and after. */
export function InstallOutput({job}: {job?: AgentInstallJob}) {
  if (!job || job.output === "") {
    return null;
  }

  return (
    <pre className="bg-muted text-muted-foreground max-h-32 overflow-auto rounded p-2 text-xs whitespace-pre-wrap">
      {job.output}
    </pre>
  );
}

/**
 * The guard in front of an upgrade. Unlike a provider switch this replaces the
 * program on disk while the agent may be running, so the command that will run
 * is shown before it does and the click is confirmed.
 */
export function ToolUpgradeConfirmDialog({
  agent,
  job,
  busy,
  onConfirm,
}: {
  agent: Agent;
  /** The running or last upgrade of this agent, when there was one. */
  job?: AgentInstallJob;
  busy: boolean;
  onConfirm: () => void;
}) {
  const plan = agent.upgrade;
  const running = job?.running === true;

  const button = (
    <Button size="sm" variant="outline" disabled={!plan?.available || running} loading={busy || running}>
      <ArrowUpCircle />
      {i18next.t(running ? "agent:Upgrading" : "agent:Upgrade")}
    </Button>
  );

  // Nothing to confirm for an agent whose install method Gateway cannot drive:
  // the button says why instead of opening a dialog with no command in it.
  if (!plan?.available) {
    return (
      <SimpleTooltip title={plan?.detail}>
        <span>{button}</span>
      </SimpleTooltip>
    );
  }

  return (
    <ConfirmDialog
      title={i18next.t("agent:Upgrade {agent}?").replace("{agent}", agent.name)}
      description={
        <span className="space-y-2">
          <span className="block">{i18next.t("agent:Upgrade hint")}</span>
          <code className="bg-muted block rounded p-2 text-xs break-all">{plan.command}</code>
        </span>
      }
      confirmText={i18next.t("agent:Upgrade")}
      variant="default"
      onConfirm={onConfirm}
    >
      {button}
    </ConfirmDialog>
  );
}
