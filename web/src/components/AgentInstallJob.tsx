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
import {CheckCircle2, Copy, MonitorCog, ScrollText, XCircle} from "lucide-react";
import copy from "copy-to-clipboard";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import type {AgentInstallJob} from "@/types";

/** What a job is doing, in the words the buttons use for it. */
export function installActionLabel(action: string, running: boolean) {
  switch (action) {
  case "uninstall":
    return i18next.t(running ? "agent:Uninstalling" : "agent:Uninstall");
  case "downgrade":
    return i18next.t(running ? "agent:Switching version" : "agent:Switch version");
  case "upgrade":
    return i18next.t(running ? "agent:Upgrading" : "agent:Upgrade");
  default:
    return i18next.t(running ? "agent:Installing" : "agent:Install");
  }
}

/**
 * The step a job is on. A package manager prints its progress rather than
 * reporting it, so the last line it wrote is the most honest answer there is to
 * "what is it doing now".
 */
function currentStep(job: AgentInstallJob) {
  const lines = job.output.split("\n").map(line => line.trim()).filter(line => line !== "");
  return lines.length > 0 ? lines[lines.length - 1] : "";
}

/**
 * How far along a job is, when its own output says. winget prints a percentage
 * and npm does not, so this is a real number when there is one and nothing at
 * all when there is not - never a guess that fills up on a timer.
 */
function reportedPercent(job: AgentInstallJob) {
  const matches = job.output.match(/(\d{1,3})\s?%/g);
  if (!matches || matches.length === 0) {
    return undefined;
  }
  const last = Number(matches[matches.length - 1].replace(/[^\d]/g, ""));
  return last >= 0 && last <= 100 ? last : undefined;
}

/** How long the job has been going, or how long it took. */
function elapsedOf(job: AgentInstallJob, now: number) {
  const started = Date.parse(job.startTime);
  const ended = job.endTime ? Date.parse(job.endTime) : now;
  if (!Number.isFinite(started) || !Number.isFinite(ended)) {
    return "";
  }
  const seconds = Math.max(0, Math.round((ended - started) / 1000));
  return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}

/** A clock that only runs while something is being timed. */
function useElapsed(active: boolean) {
  const [now, setNow] = React.useState(() => Date.now());

  React.useEffect(() => {
    if (!active) {
      return;
    }
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [active]);

  return now;
}

/**
 * The bar under a running install. A percentage is shown when the manager
 * reports one; otherwise the bar sweeps, which says that work is happening
 * without claiming to know how much is left.
 */
function InstallBar({percent}: {percent?: number}) {
  return (
    <div className="bg-muted relative h-1.5 w-full overflow-hidden rounded-full">
      {percent === undefined ? (
        <span
          className="bg-primary absolute inset-y-0 w-2/5 rounded-full"
          style={{animation: "install-sweep 1.4s ease-in-out infinite"}}
        />
      ) : (
        <span
          className="bg-primary absolute inset-y-0 left-0 rounded-full transition-all"
          style={{width: `${percent}%`}}
        />
      )}
    </div>
  );
}

/**
 * What one install, upgrade or removal is doing, in a row: the step it is on, a
 * bar while it runs, and the way into the console output it is writing. It
 * stays after the job ends, so a failure can be read rather than guessed at.
 */
export function InstallJobProgress({job, className = ""}: {job?: AgentInstallJob; className?: string}) {
  const now = useElapsed(job?.running === true);
  if (!job) {
    return null;
  }

  const step = currentStep(job);
  const elapsed = elapsedOf(job, now);

  return (
    <div className={`min-w-0 space-y-1.5 ${className}`}>
      <div className="flex items-center gap-2 text-xs">
        <InstallJobBadge job={job} />
        <span className="text-muted-foreground tabular-nums">{elapsed}</span>
        <InstallLogDialog job={job}>
          <Button size="sm" variant="ghost" className="ml-auto h-6 px-2 text-xs">
            <ScrollText className="size-3" />
            {i18next.t("agent:Log")}
          </Button>
        </InstallLogDialog>
      </div>

      {job.running ? <InstallBar percent={reportedPercent(job)} /> : null}

      {job.interactive && job.running ? (
        <p className="text-warning flex items-center gap-1.5 text-xs">
          <MonitorCog className="size-3 shrink-0" />
          {i18next.t("agent:Waiting at the machine")}
        </p>
      ) : null}

      {step || job.error ? (
        <p className="text-muted-foreground truncate font-mono text-xs" title={job.error || step}>
          {job.error || step}
        </p>
      ) : null}
    </div>
  );
}

/** Where a job stands: running, done, or failed. */
export function InstallJobBadge({job}: {job: AgentInstallJob}) {
  if (job.running) {
    return (
      <Badge variant="info">
        <span className="bg-info size-1.5 animate-pulse rounded-full" />
        {installActionLabel(job.action, true)}
      </Badge>
    );
  }
  return (
    <Badge variant={job.ok ? "success" : "destructive"}>
      {job.ok ? <CheckCircle2 className="size-3" /> : <XCircle className="size-3" />}
      {i18next.t(job.ok ? "agent:Finished" : "agent:Failed")}
    </Badge>
  );
}

/**
 * Everything one package manager printed, live while it prints it. An install
 * that fails says why on its own last lines, so this is where that goes rather
 * than a message that only says it failed.
 */
export function InstallLogDialog({
  job,
  children,
}: {
  job?: AgentInstallJob;
  children: React.ReactNode;
}) {
  const [open, setOpen] = React.useState(false);
  const bottom = React.useRef<HTMLDivElement>(null);
  const output = job?.output ?? "";

  // A running install keeps writing, and the end is the part worth reading.
  React.useEffect(() => {
    if (open) {
      bottom.current?.scrollIntoView({block: "end"});
    }
  }, [open, output]);

  if (!job) {
    return null;
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>{children}</DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {installActionLabel(job.action, false)} · {job.name}
            <InstallJobBadge job={job} />
          </DialogTitle>
          <DialogDescription>{i18next.t("agent:Install log hint")}</DialogDescription>
        </DialogHeader>

        <code className="bg-muted block rounded p-2 text-xs break-all">{job.command}</code>

        <pre className="bg-muted/60 scrollbar-thin max-h-80 overflow-auto rounded p-3 text-xs whitespace-pre-wrap">
          {output || i18next.t("agent:No output yet")}
          <div ref={bottom} />
        </pre>

        {job.error ? <p className="text-destructive text-xs">{job.error}</p> : null}

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => {
              copy(`${job.command}\n\n${output}`);
              Setting.showMessage("success", i18next.t("general:Copied to clipboard"));
            }}
          >
            <Copy />
            {i18next.t("general:Copy")}
          </Button>
          <Button onClick={() => setOpen(false)}>{i18next.t("general:Close")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** The tail of what a package manager printed, for the places with room for it. */
export function InstallOutput({job}: {job?: AgentInstallJob}) {
  if (!job) {
    return null;
  }
  return <InstallJobProgress job={job} />;
}
