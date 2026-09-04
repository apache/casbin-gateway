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
import {ChevronUp, EyeOff, FileCode2, Folder, FolderSearch, HardDrive, Home} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
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
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Input} from "@/components/ui/input";
import type {Agent, BrowseListing} from "@/types";

/**
 * The picker for an agent no layout and no PATH entry describes. The host does
 * the listing: a browser cannot read a real path out of a file input, and the
 * machine being looked at is the one Gateway runs on.
 */
export function AgentPathDialog({
  agentId,
  name,
  trigger,
  onAdded,
}: {
  agentId: string;
  name: string;
  /** The control that opens it, for a caller whose row has its own shape. */
  trigger?: React.ReactNode;
  onAdded?: () => void;
}) {
  const [open, setOpen] = React.useState(false);
  const [listing, setListing] = React.useState<BrowseListing>();
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [chosen, setChosen] = React.useState("");
  const [saving, setSaving] = React.useState(false);

  const browse = React.useCallback((path: string) => {
    setLoading(true);
    setError("");
    AgentBackend.browseLocalPath(path)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setListing(res.data);
        } else {
          setError(res.msg ?? "");
        }
      })
      .catch(() => setError(i18next.t("agent:This folder could not be read")))
      .then(() => setLoading(false));
  }, []);

  React.useEffect(() => {
    if (open) {
      setChosen("");
      browse("");
    }
  }, [open, browse]);

  // The path is passed in: a double-click fires before the click that chose it
  // has rendered.
  function save(path = chosen) {
    setSaving(true);
    AgentBackend.addAgentPath(agentId, path)
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg ?? "");
          return;
        }
        Setting.showMessage("success", i18next.t("agent:Agent program added"));
        setOpen(false);
        onAdded?.();
      })
      .catch(error => Setting.showMessage("error", String(error)))
      .then(() => setSaving(false));
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger ?? (
          <Button size="sm" variant="outline">
            <FolderSearch />
            {i18next.t("agent:Locate")}
          </Button>
        )}
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agent:Locate {agent}").replace("{agent}", name)}</DialogTitle>
          <DialogDescription>{i18next.t("agent:Locate hint")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-2 text-sm">
          <div className="flex flex-wrap items-center gap-1.5">
            <Button
              size="sm"
              variant="outline"
              disabled={!listing?.parent || loading}
              onClick={() => browse(listing?.parent ?? "")}
            >
              <ChevronUp />
              {i18next.t("agent:Up")}
            </Button>
            <Button size="sm" variant="ghost" disabled={loading} onClick={() => browse("")}>
              <Home />
              {i18next.t("agent:Home folder")}
            </Button>
            {(listing?.roots ?? []).map(root => (
              <Button
                key={root}
                size="sm"
                variant="ghost"
                disabled={loading}
                onClick={() => browse(root)}
              >
                <HardDrive />
                {root}
              </Button>
            ))}
          </div>

          <code className="text-muted-foreground block truncate text-xs">{listing?.path ?? ""}</code>

          <div className="h-64 overflow-y-auto rounded-md border">
            {(listing?.entries ?? []).map(entry => (
              <button
                key={entry.path}
                type="button"
                disabled={!entry.dir && !entry.executable}
                className={`hover:bg-muted flex w-full items-center gap-2 px-2 py-1 text-left disabled:opacity-40 ${
                  chosen === entry.path ? "bg-muted" : ""
                }`}
                onClick={() => (entry.dir ? browse(entry.path) : setChosen(entry.path))}
                onDoubleClick={() => (!entry.dir && entry.executable ? save(entry.path) : undefined)}
              >
                {entry.dir ? (
                  <Folder className="text-muted-foreground size-4 shrink-0" />
                ) : (
                  <FileCode2 className="text-muted-foreground size-4 shrink-0" />
                )}
                <span className="min-w-0 flex-1 truncate text-xs">{entry.name}</span>
              </button>
            ))}
            {!loading && (listing?.entries.length ?? 0) === 0 ? (
              <p className="text-muted-foreground p-3 text-xs">
                {error || i18next.t("agent:This folder is empty")}
              </p>
            ) : null}
          </div>

          {error && listing?.entries.length ? (
            <p className="text-destructive text-xs">{error}</p>
          ) : null}

          {/* A program nobody wants to walk to is pasted instead. */}
          <Input
            value={chosen}
            placeholder={i18next.t("agent:Program path placeholder")}
            onChange={event => setChosen(event.target.value)}
          />
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>
            {i18next.t("general:Cancel")}
          </Button>
          <Button disabled={!chosen.trim()} loading={saving} onClick={() => save()}>
            {i18next.t("agent:Use this program")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/** The way out for a chosen program: it forgets the row, and deletes nothing. */
export function AgentPathForgetButton({
  agent,
  onRemoved,
}: {
  agent: Agent;
  onRemoved?: () => void;
}) {
  function forget() {
    return AgentBackend.removeAgentPath(agent.agentId, agent.path)
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg ?? "");
          return;
        }
        Setting.showMessage("success", i18next.t("agent:Agent program forgotten"));
        onRemoved?.();
      })
      .catch(error => Setting.showMessage("error", String(error)));
  }

  return (
    <ConfirmDialog
      title={i18next.t("agent:Forget {agent}?").replace("{agent}", agent.name)}
      description={i18next.t("agent:Forget hint")}
      confirmText={i18next.t("agent:Forget")}
      onConfirm={forget}
    >
      <Button size="sm" variant="outline">
        <EyeOff />
        {i18next.t("agent:Forget")}
      </Button>
    </ConfirmDialog>
  );
}
