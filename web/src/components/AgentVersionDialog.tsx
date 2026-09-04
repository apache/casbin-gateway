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
import {ArrowUpCircle, History, RefreshCw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {Agent, AgentUpdate, AgentVersionCatalog} from "@/types";

/**
 * The mark on an installation a newer release is waiting for. Nothing is shown
 * while the lookup is out, or for an agent whose manager publishes no list: an
 * installation is never called out of date on a version nobody could read.
 */
export function AgentUpdateBadge({update, className}: {update?: AgentUpdate; className?: string}) {
  if (!update?.available) {
    return null;
  }

  return (
    <SimpleTooltip title={`${i18next.t("agent:Latest version")}: ${update.latest}`}>
      <Badge variant="warning" className={className}>
        <ArrowUpCircle className="size-3" />
        {i18next.t("agent:New version")} {update.latest}
      </Badge>
    </SimpleTooltip>
  );
}

/**
 * The picker behind a version change. An agent is moved onto any release its
 * package manager still publishes, which is how a broken update is undone: the
 * command that will run is spelled out before the click that runs it.
 *
 * It serves an agent this machine does not have too, where the releases are the
 * ones a first install can ask for and `installedVersion` is empty.
 */
export function AgentVersionDialog({
  agentId,
  name,
  installMethod = "",
  installedVersion = "",
  update,
  busy,
  fallbackDetail,
  onSelect,
}: {
  agentId: string;
  name: string;
  /** The manager that owns the tree, empty for an agent that is not installed. */
  installMethod?: string;
  installedVersion?: string;
  /** What the manager publishes, when the page has already looked it up. */
  update?: AgentUpdate;
  busy: boolean;
  /** Why there may be nothing to pick, from whatever the page already knows. */
  fallbackDetail?: string;
  onSelect: (version: string) => void;
}) {
  const [open, setOpen] = React.useState(false);
  const [catalog, setCatalog] = React.useState<AgentVersionCatalog>();
  const [loading, setLoading] = React.useState(false);
  const [chosen, setChosen] = React.useState("");

  const load = React.useCallback(
    (forceRefresh = false) => {
      setLoading(true);
      AgentBackend.getAgentVersions(agentId, installMethod, forceRefresh)
        .then(res => setCatalog(res.status === "ok" ? res.data : undefined))
        .catch(() => setCatalog(undefined))
        .then(() => setLoading(false));
    },
    [agentId, installMethod],
  );

  // The list is asked for when the dialog opens rather than with the page: a
  // registry lookup per installed agent on every load is a lot of waiting for
  // something most visits never open.
  React.useEffect(() => {
    if (open) {
      setChosen("");
      load();
    }
  }, [open, load]);

  const versions = catalog?.versions ?? [];
  const template = catalog?.commandTemplate ?? "";
  // Without a template the manager cannot be asked for a named release, which
  // is every Homebrew cask: it installs the one version its own index carries.
  const canPick = versions.length > 0 && template !== "";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline" loading={busy}>
          <History />
          {i18next.t("agent:Versions")}
          {update?.available ? <span className="bg-warning size-1.5 rounded-full" /> : null}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{i18next.t("agent:Versions of {agent}").replace("{agent}", name)}</DialogTitle>
          <DialogDescription>
            {i18next.t(installedVersion ? "agent:Version picker hint" : "agent:Version install hint")}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3 text-sm">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-muted-foreground">{i18next.t("agent:Installed version")}</span>
            <Badge variant="secondary" className="tabular-nums">
              {installedVersion || i18next.t("agent:Not installed")}
            </Badge>
            {catalog?.latest ? (
              <>
                <span className="text-muted-foreground">{i18next.t("agent:Latest version")}</span>
                <Badge variant={update?.available ? "warning" : "muted"} className="tabular-nums">
                  {catalog.latest}
                </Badge>
              </>
            ) : null}
            <Button
              size="sm"
              variant="ghost"
              className="ml-auto"
              loading={loading}
              onClick={() => load(true)}
            >
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          </div>

          {canPick ? (
            <Select value={chosen} onValueChange={setChosen}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={i18next.t("agent:Pick a version")} />
              </SelectTrigger>
              <SelectContent>
                {versions.map(version => (
                  <SelectItem key={version} value={version}>
                    <span className="tabular-nums">{version}</span>
                    {version === catalog?.latest ? (
                      <Badge variant="muted" className="ml-2 text-[11px]">
                        {i18next.t("agent:Latest")}
                      </Badge>
                    ) : null}
                    {version === installedVersion ? (
                      <Badge variant="muted" className="ml-2 text-[11px]">
                        {i18next.t("agent:Installed")}
                      </Badge>
                    ) : null}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <p className="text-muted-foreground text-xs">
              {catalog?.detail || fallbackDetail || i18next.t("agent:No versions to list")}
            </p>
          )}

          {chosen && chosen !== installedVersion ? (
            <code className="bg-muted block rounded p-2 text-xs break-all">
              {template.replace("{version}", chosen)}
            </code>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {i18next.t("general:Cancel")}
          </Button>
          <Button
            disabled={!chosen || chosen === installedVersion}
            onClick={() => {
              setOpen(false);
              onSelect(chosen);
            }}
          >
            {i18next.t(installedVersion ? "agent:Switch version" : "agent:Install")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * The guard in front of an uninstall. The program goes; the agent's own state
 * directory, with its sign-in and its history, stays where it is, so the
 * dialog says so rather than leaving that to be discovered.
 */
export function ToolUninstallConfirmDialog({
  agent,
  busy,
  onConfirm,
}: {
  agent: Agent;
  busy: boolean;
  onConfirm: () => void;
}) {
  const plan = agent.uninstall;

  const button = (
    <Button size="sm" variant="outline" disabled={!plan?.available} loading={busy}>
      <Trash2 />
      {i18next.t("agent:Uninstall")}
    </Button>
  );

  if (!plan?.available) {
    return (
      <SimpleTooltip title={plan?.detail}>
        <span>{button}</span>
      </SimpleTooltip>
    );
  }

  return (
    <ConfirmDialog
      title={i18next.t("agent:Uninstall {agent}?").replace("{agent}", agent.name)}
      description={
        <span className="space-y-2">
          <span className="block">{i18next.t("agent:Uninstall hint")}</span>
          <code className="bg-muted block rounded p-2 text-xs break-all">{plan.command}</code>
        </span>
      }
      confirmText={i18next.t("agent:Uninstall")}
      onConfirm={onConfirm}
    >
      {button}
    </ConfirmDialog>
  );
}
