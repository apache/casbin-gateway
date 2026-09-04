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
import {AlertTriangle, ArrowUpCircle, Download, ExternalLink, MonitorCog} from "lucide-react";
import i18next from "i18next";
import type {VariantProps} from "class-variance-authority";

import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Button, type buttonVariants} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {Agent, AgentInstallJob, AgentInstallPlan} from "@/types";

export {InstallJobProgress, InstallLogDialog, InstallOutput} from "@/components/AgentInstallJob";

/**
 * One click that changes what is on disk. Whatever the agent was installed with
 * - a package manager, the uninstaller it registered with Windows, its own
 * updater, or the vendor's install command - the command is spelled out first
 * and confirmed before it runs.
 *
 * An agent nothing here can drive keeps a click of its own: the vendor's page,
 * which is where that install has to be done by hand. A button that only greys
 * out tells a person nothing they can act on.
 */
export function AgentActionButton({
  title,
  label,
  runningLabel,
  icon,
  plan,
  job,
  busy,
  variant = "outline",
  confirmVariant = "default",
  hint,
  fallbackUrl,
  fallbackLabel,
  onConfirm,
}: {
  title: React.ReactNode;
  label: string;
  runningLabel: string;
  icon: React.ReactNode;
  plan?: AgentInstallPlan;
  /** The running or last job of this agent, when there was one. */
  job?: AgentInstallJob;
  busy: boolean;
  variant?: VariantProps<typeof buttonVariants>["variant"];
  confirmVariant?: VariantProps<typeof buttonVariants>["variant"];
  /** What this action does, in a sentence, above the command itself. */
  hint?: React.ReactNode;
  /** The vendor's page, for when Gateway has no command for this agent. */
  fallbackUrl?: string;
  /** What that page is called, since it is not the action it stands in for. */
  fallbackLabel?: string;
  onConfirm: () => void;
}) {
  const running = job?.running === true;

  // Gateway has no command for this one, but the vendor's own page still does
  // what the button was for, so it goes there rather than greying out.
  if (!plan?.available && fallbackUrl) {
    return (
      <SimpleTooltip title={plan?.detail}>
        <Button
          size="sm"
          variant={variant}
          onClick={() => window.open(fallbackUrl, "_blank", "noreferrer")}
        >
          <ExternalLink />
          {fallbackLabel ?? i18next.t("agent:Vendor page")}
        </Button>
      </SimpleTooltip>
    );
  }

  const button = (
    <Button size="sm" variant={variant} disabled={!plan?.available} loading={busy || running}>
      {icon}
      {running ? runningLabel : label}
    </Button>
  );

  // Nothing at all can reach this installation, so the button says why instead
  // of opening a dialog with no command in it.
  if (!plan?.available) {
    return (
      <SimpleTooltip title={plan?.detail}>
        <span>{button}</span>
      </SimpleTooltip>
    );
  }

  return (
    <ConfirmDialog
      title={title}
      description={
        <span className="space-y-2">
          {hint ? <span className="block">{hint}</span> : null}
          <code className="bg-muted block rounded p-2 text-xs break-all">{plan.command}</code>
          {plan.warning ? (
            <span className="text-warning flex items-start gap-1.5">
              <AlertTriangle className="mt-0.5 size-3.5 shrink-0" />
              {plan.warning}
            </span>
          ) : null}
          {plan.interactive ? (
            <span className="text-muted-foreground flex items-start gap-1.5">
              <MonitorCog className="mt-0.5 size-3.5 shrink-0" />
              {i18next.t("agent:Interactive hint")}
            </span>
          ) : null}
        </span>
      }
      confirmText={label}
      variant={confirmVariant}
      onConfirm={onConfirm}
    >
      {button}
    </ConfirmDialog>
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
  job?: AgentInstallJob;
  busy: boolean;
  onConfirm: () => void;
}) {
  return (
    <AgentActionButton
      title={i18next.t("agent:Upgrade {agent}?").replace("{agent}", agent.name)}
      label={i18next.t("agent:Upgrade")}
      runningLabel={i18next.t("agent:Upgrading")}
      icon={<ArrowUpCircle />}
      plan={agent.upgrade}
      job={job}
      busy={busy}
      hint={i18next.t("agent:Upgrade hint")}
      fallbackUrl={agent.upgrade?.installUrl}
      fallbackLabel={i18next.t("agent:Update manually")}
      onConfirm={onConfirm}
    />
  );
}

/**
 * The click that installs an agent this machine does not have. It is one click:
 * an install adds a program rather than replacing or removing one, so there is
 * nothing to undo and nothing to warn about - unless the only way in is the
 * vendor's own script, which is confirmed like any other command that runs
 * something downloaded.
 */
export function AgentInstallButton({
  name,
  plan,
  installUrl,
  job,
  busy,
  label,
  onInstall,
}: {
  name: string;
  plan?: AgentInstallPlan;
  /** The vendor's own page, where an agent nothing here installs is got. */
  installUrl?: string;
  job?: AgentInstallJob;
  busy: boolean;
  /** What the button says, for the places that call a second try a retry. */
  label?: string;
  onInstall: () => void;
}) {
  const running = job?.running === true;
  const text = label ?? i18next.t("agent:Install");

  if (plan?.available && !plan.warning) {
    return (
      <SimpleTooltip title={plan.command}>
        <span>
          <Button size="sm" disabled={running} loading={busy || running} onClick={onInstall}>
            <Download />
            {running ? i18next.t("agent:Installing") : text}
          </Button>
        </span>
      </SimpleTooltip>
    );
  }

  return (
    <AgentActionButton
      title={i18next.t("agent:Install {agent}?").replace("{agent}", name)}
      label={text}
      runningLabel={i18next.t("agent:Installing")}
      icon={<Download />}
      plan={plan}
      job={job}
      busy={busy}
      variant="default"
      hint={i18next.t("agent:Install hint")}
      fallbackUrl={installUrl}
      onConfirm={onInstall}
    />
  );
}
