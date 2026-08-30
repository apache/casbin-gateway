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
import {Link} from "react-router-dom";
import {ArrowUpCircle, Check, CircleAlert, Copy, ExternalLink, Loader2, RefreshCw} from "lucide-react";
import copy from "copy-to-clipboard";
import i18next from "i18next";

import * as MiscBackend from "@/backend/MiscBackend";
import type {UpdateStatus, VersionBuild, VersionInfo, VersionRelease} from "@/types";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Progress} from "@/components/ui/progress";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {formatBytes} from "@/lib/agent-configs";

/**
 * Where the update has got to as far as this page is concerned. It is not the
 * backend's stage: from "restarting" onwards the backend is gone, and what is
 * left to follow is whether the new one has answered yet.
 */
type Phase = "idle" | "starting" | "running" | "waiting" | "done";

/** How long the restarted Gateway is given to answer before giving up on it. */
const restartTimeout = 120 * 1000;
const runningPollInterval = 800;
const waitingPollInterval = 1500;

function buildLabel(build: VersionBuild) {
  if (build.shortCommit === "") {
    return build.version;
  }

  const suffix = build.modified ? `${build.shortCommit}+` : build.shortCommit;
  return `${build.version} · ${suffix}`;
}

function releaseLabel(release: VersionRelease) {
  return release.shortCommit === "" ? release.tag : `${release.tag} · ${release.shortCommit}`;
}

function pad(value: number) {
  return String(value).padStart(2, "0");
}

/** "2026-08-23 17:22", in the reader's own time zone. */
function formatMoment(value: string) {
  const moment = new Date(value);
  if (isNaN(moment.getTime())) {
    return "";
  }

  const day = formatDay(value);
  return `${day} ${pad(moment.getHours())}:${pad(moment.getMinutes())}`;
}

/** The day alone, which is what says at a glance how old a build is. */
function formatDay(value: string) {
  const moment = new Date(value);
  if (isNaN(moment.getTime())) {
    return "";
  }

  return `${moment.getFullYear()}-${pad(moment.getMonth() + 1)}-${pad(moment.getDate())}`;
}

function blockedHint(blocked: string) {
  switch (blocked) {
  case "unsupported-platform":
    return i18next.t("general:No build is published for this platform, so the update has to be done by hand");
  case "read-only":
    return i18next.t("general:This executable sits where it cannot be replaced, so the update has to be done by hand");
  default:
    return i18next.t("general:This Gateway cannot find its own executable, so the update has to be done by hand");
  }
}

function stageLabel(status: UpdateStatus | null, phase: Phase) {
  if (phase === "waiting") {
    return i18next.t("general:Restarting into the new version");
  }
  if (phase === "done") {
    return i18next.t("general:Update complete, reloading");
  }
  switch (status?.stage) {
  case "installing":
    return i18next.t("general:Installing");
  case "restarting":
    return i18next.t("general:Restarting into the new version");
  default:
    return i18next.t("general:Downloading");
  }
}

/** A labelled row of the version table, so the two versions line up. */
function InfoRow({label, value, note}: {label: string; value: React.ReactNode; note?: string}) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-1.5">
      <span className="text-muted-foreground shrink-0 text-sm">{label}</span>
      <span className="min-w-0 text-right">
        <span className="block truncate font-mono text-sm font-medium">{value}</span>
        {note ? <span className="text-muted-foreground block text-xs">{note}</span> : null}
      </span>
    </div>
  );
}

function CommandLine({command}: {command: string}) {
  const [copied, setCopied] = React.useState(false);

  return (
    <div className="bg-muted/60 flex items-center gap-2 rounded-md border p-2">
      <code className="scrollbar-thin min-w-0 flex-1 overflow-x-auto text-xs whitespace-nowrap">{command}</code>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={i18next.t("general:Copy")}
        onClick={() => {
          copy(command);
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1500);
        }}
      >
        {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </Button>
    </div>
  );
}

/**
 * The version of this Gateway, the version it could be, and the one button that
 * closes the gap. It sits in the header rather than out of the way, and carries
 * the build date: a commit hash alone says nothing about how old a build is.
 */
export function VersionPanel({
  isAdmin,
  signedIn,
}: {
  isAdmin: boolean;
  /** The version is behind the session, so asking before there is one answers nothing. */
  signedIn: boolean;
}) {
  const [info, setInfo] = React.useState<VersionInfo | null>(null);
  const [checking, setChecking] = React.useState(false);
  const [open, setOpen] = React.useState(false);
  const [phase, setPhase] = React.useState<Phase>("idle");
  const [status, setStatus] = React.useState<UpdateStatus | null>(null);
  const [failure, setFailure] = React.useState("");
  // A failure that is the network, not the update: a proxy is what fixes it.
  const [failureNetwork, setFailureNetwork] = React.useState(false);

  const load = React.useCallback((refresh: boolean) => {
    setChecking(refresh);
    return MiscBackend.getVersion(refresh)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setInfo(res.data);
        }
      })
      // A version nobody can read is not worth a toast on every page load.
      .catch(() => undefined)
      .finally(() => setChecking(false));
  }, []);

  React.useEffect(() => {
    if (signedIn) {
      load(false);
    }
  }, [load, signedIn]);

  // While the update runs, the backend is still there to be asked how far it
  // has got. It stops answering the moment it restarts, which is not a failure
  // but the next phase.
  React.useEffect(() => {
    if (phase !== "running") {
      return;
    }

    let stopped = false;
    const tick = () => {
      MiscBackend.getUpdateStatus()
        .then(res => {
          if (stopped || res.status !== "ok" || !res.data) {
            return;
          }
          setStatus(res.data);
          if (res.data.stage === "failed") {
            setFailure(res.data.error);
            setFailureNetwork(res.data.network);
            setPhase("idle");
          } else if (res.data.stage === "restarting") {
            setPhase("waiting");
          }
        })
        .catch(() => {
          if (!stopped) {
            setPhase("waiting");
          }
        });
    };

    const timer = window.setInterval(tick, runningPollInterval);
    tick();
    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [phase]);

  // The restarted Gateway is a new process, so its update status is "idle" —
  // which is exactly what tells the two of them apart across the restart.
  React.useEffect(() => {
    if (phase !== "waiting") {
      return;
    }

    let stopped = false;
    const startedAt = Date.now();
    const timer = window.setInterval(() => {
      if (Date.now() - startedAt > restartTimeout) {
        stopped = true;
        window.clearInterval(timer);
        setFailure(i18next.t("general:The new version did not start, check the Gateway log"));
        setFailureNetwork(false);
        setPhase("idle");
        return;
      }

      MiscBackend.getUpdateStatus()
        .then(res => {
          if (stopped || res.status !== "ok" || !res.data) {
            return;
          }
          if (res.data.stage === "failed") {
            stopped = true;
            window.clearInterval(timer);
            setFailure(res.data.error);
            setFailureNetwork(res.data.network);
            setPhase("idle");
            return;
          }
          if (res.data.stage === "idle") {
            stopped = true;
            window.clearInterval(timer);
            setPhase("done");
            window.setTimeout(() => window.location.reload(), 1200);
          }
        })
        // The gap where nothing answers is the restart itself.
        .catch(() => undefined);
    }, waitingPollInterval);

    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [phase]);

  const startUpdate = () => {
    setFailure("");
    setFailureNetwork(false);
    setStatus(null);
    setPhase("starting");
    MiscBackend.updateGateway()
      .then(res => {
        if (res.status !== "ok") {
          setFailure(res.msg);
          setPhase("idle");
          return;
        }
        setStatus(res.data ?? null);
        setPhase("running");
      })
      .catch(error => {
        setFailure(error?.message ?? String(error));
        setPhase("idle");
      });
  };

  const busy = phase !== "idle";
  const updateAvailable = info?.updateAvailable === true;
  const current = info?.current;
  const day = current ? formatDay(current.buildTime) : "";
  // "nightly · 7696f22 · 2026-08-23": which build, and how old it is.
  const label = current ? [buildLabel(current), day].filter(part => part !== "").join(" · ") : "";

  // An empty chip in the header while the version is on its way, or for a
  // reader with no session to ask with, is worse than no chip at all.
  if (info === null) {
    return null;
  }

  const trigger = (
    <SimpleTooltip
      title={updateAvailable && info.latest ? `${i18next.t("general:Update available")}: ${releaseLabel(info.latest)}` : label}
    >
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={cn(
          // No transition-colors: switching theme leaves an element that has one
          // stuck on the palette it was rendered in until the page reloads.
          "flex shrink-0 items-center gap-1.5 rounded-md border px-2 py-1 font-mono text-xs",
          updateAvailable
            ? "border-primary/40 bg-primary/10 text-primary hover:bg-primary/20"
            : "text-muted-foreground hover:bg-accent hover:text-foreground border-transparent",
        )}
      >
        {updateAvailable ? <ArrowUpCircle className="size-3.5 shrink-0" /> : null}
        {/* Narrow screens keep the date, which is the half that ages. */}
        <span className="hidden lg:inline">{label}</span>
        <span className="lg:hidden">{day === "" ? label : day}</span>
        {updateAvailable ? (
          <span className="bg-primary text-primary-foreground shrink-0 rounded-full px-1.5 text-[10px] font-medium">
            {i18next.t("general:New")}
          </span>
        ) : null}
      </button>
    </SimpleTooltip>
  );

  return (
    <>
      {trigger}

      <Dialog
        open={open}
        onOpenChange={next => {
          // Closing mid-update would leave nothing watching for the restart,
          // and the page is about to reload itself anyway.
          if (!busy) {
            setOpen(next);
          }
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{i18next.t("general:Version")}</DialogTitle>
            <DialogDescription>
              {i18next.t("general:Casbin Gateway installs updates from the nightly build of master")}
            </DialogDescription>
          </DialogHeader>

          <div className="divide-y">
            {current ? (
              <InfoRow
                label={i18next.t("general:Current version")}
                value={buildLabel(current)}
                note={[
                  current.buildTime ? `${i18next.t("general:Built")} ${formatMoment(current.buildTime)}` : "",
                  `${current.os}/${current.arch}`,
                ]
                  .filter(part => part !== "")
                  .join(" · ")}
              />
            ) : (
              <InfoRow label={i18next.t("general:Current version")} value="…" />
            )}

            {info?.latest ? (
              <InfoRow
                label={i18next.t("general:Latest version")}
                value={releaseLabel(info.latest)}
                note={
                  info.latest.publishedAt
                    ? `${i18next.t("general:Published")} ${formatMoment(info.latest.publishedAt)}`
                    : undefined
                }
              />
            ) : null}
          </div>

          {info?.checkError ? (
            <p className="text-muted-foreground flex items-start gap-2 text-xs">
              <CircleAlert className="text-warning mt-0.5 size-3.5 shrink-0" />
              {i18next.t("general:Could not check for updates")}: {info.checkError}
            </p>
          ) : null}

          {failure ? (
            <p className="text-destructive flex items-start gap-2 text-xs">
              <CircleAlert className="mt-0.5 size-3.5 shrink-0" />
              {failure}
            </p>
          ) : null}

          {failureNetwork || (failure === "" && info.checkNetwork) ? (
            <p className="text-muted-foreground text-xs">
              {i18next.t("general:GitHub could not be reached, if this machine needs a proxy to reach the internet, set one and try again")}{" "}
              <Link to="/settings" onClick={() => setOpen(false)} className="text-primary underline underline-offset-2">
                {i18next.t("general:Set a proxy")}
              </Link>
            </p>
          ) : null}

          {busy ? (
            <div className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span className="flex items-center gap-2">
                  {phase === "done" ? (
                    <Check className="text-success size-4" />
                  ) : (
                    <Loader2 className="size-4 animate-spin" />
                  )}
                  {stageLabel(status, phase)}
                </span>
                {status && status.stage === "downloading" && status.total > 0 ? (
                  <span className="text-muted-foreground text-xs">
                    {formatBytes(status.downloaded)} / {formatBytes(status.total)}
                  </span>
                ) : null}
              </div>
              <Progress
                value={phase === "done" ? 100 : (status?.percent ?? 0)}
                tone={phase === "done" ? "success" : "default"}
                className={cn(phase === "waiting" && "animate-pulse")}
              />
              <p className="text-muted-foreground text-xs">
                {i18next.t("general:Leave this page open, it reloads itself when the new version is up")}
              </p>
            </div>
          ) : updateAvailable && info?.canUpdate && isAdmin ? (
            <Button onClick={startUpdate} className="w-full">
              <ArrowUpCircle className="size-4" />
              {i18next.t("general:Update now")}
            </Button>
          ) : updateAvailable && info && !info.canUpdate ? (
            <div className="space-y-2">
              <p className="text-muted-foreground text-xs">{blockedHint(info.blocked)}</p>
              <CommandLine command={info.installCommand} />
            </div>
          ) : info !== null && !updateAvailable && info.checkError === "" ? (
            <p className="text-muted-foreground flex items-center gap-2 text-sm">
              <Check className="text-success size-4" />
              {i18next.t("general:This is the latest version")}
            </p>
          ) : null}

          <div className="flex items-center justify-between gap-2 pt-1">
            <Button variant="ghost" size="sm" disabled={busy || checking} onClick={() => load(true)}>
              <RefreshCw className={cn("size-4", checking && "animate-spin")} />
              {i18next.t("general:Check for updates")}
            </Button>
            {info?.releaseUrl ? (
              <Button variant="ghost" size="sm" asChild>
                <a href={info.releaseUrl} target="_blank" rel="noreferrer">
                  {i18next.t("general:Release notes")}
                  <ExternalLink className="size-3.5" />
                </a>
              </Button>
            ) : null}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
