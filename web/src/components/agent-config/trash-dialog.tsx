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
import {RotateCcw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {EmptyState} from "@/components/shared/empty-state";
import {Loading} from "@/components/shared/loading";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {formatBytes, formatModified} from "@/lib/agent-configs";
import type {AgentConfigTrashEntry} from "@/types";

/**
 * What deleting removed, and the way back. A delete on this page moves the item
 * here instead of erasing it, so a mistyped click is undone by one restore.
 */
export function TrashDialog({
  open,
  onOpenChange,
  owner,
  agentNames,
  onRestored,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  owner: string;
  agentNames: Map<string, string>;
  onRestored: () => void;
}) {
  const [entries, setEntries] = React.useState<AgentConfigTrashEntry[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [busy, setBusy] = React.useState("");
  const [error, setError] = React.useState("");
  // The entries whose place has been taken since they were deleted. Restoring
  // one of those is offered a second time, as a replacement.
  const [blocked, setBlocked] = React.useState<string[]>([]);

  const load = React.useCallback(() => {
    setLoading(true);
    setError("");
    setBlocked([]);
    AgentConfigBackend.getAgentConfigTrash(owner)
      .then(res => {
        if (res.status === "ok") {
          setEntries(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("agentConfig:Failed to read the recycle bin"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [owner]);

  React.useEffect(() => {
    if (open) {
      load();
    }
  }, [open, load]);

  const restore = (entry: AgentConfigTrashEntry, replace = false) => {
    setBusy(entry.id);
    return AgentConfigBackend.restoreAgentConfigItem(owner, entry.id, replace)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", `${i18next.t("agentConfig:Restored")}: ${entry.name}`);
          load();
          onRestored();
        } else {
          // Something took its place while it was in here, and the server says
          // so rather than overwriting it. The operator reads what stands in the
          // way and is offered the restore again, as a replacement.
          if (res.msg?.includes("already")) {
            setBlocked(previous => [...previous, entry.id]);
          }
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to restore"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusy(""));
  };

  const purge = (id: string) => {
    setBusy(id || "all");
    return AgentConfigBackend.purgeAgentConfigTrash(owner, id)
      .then(res => {
        if (res.status === "ok") {
          load();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to delete"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setBusy(""));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] gap-3 sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agentConfig:Recycle bin")}</DialogTitle>
          <DialogDescription>{i18next.t("agentConfig:Recycle bin hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-2 overflow-y-auto">
          {error ? <MessageAlert description={error} /> : null}
          {loading ? <Loading /> : null}

          {!loading && entries.length === 0 ? (
            <EmptyState
              icon={Trash2}
              title={i18next.t("agentConfig:Nothing deleted")}
              description={i18next.t("agentConfig:Nothing deleted detail")}
            />
          ) : null}

          {entries.map(entry => (
            <div key={entry.id} className="flex items-center justify-between gap-3 rounded-md border px-3 py-2">
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="truncate text-sm font-medium">{entry.name}</span>
                  <Badge variant="muted">{agentNames.get(entry.agentId) ?? entry.agentId}</Badge>
                  <Badge variant="outline">
                    {i18next.t(
                      entry.kind === "skill"
                        ? "agentConfig:Skill"
                        : entry.kind === "prompt"
                          ? "agentConfig:Prompt"
                          : "agentConfig:MCP",
                    )}
                  </Badge>
                </div>
                <p className="text-muted-foreground truncate text-xs">
                  {formatModified(entry.deletedAt)}
                  {entry.bytes ? ` · ${formatBytes(entry.bytes)}` : ""}
                  {` · ${entry.path}`}
                </p>
              </div>

              <div className="flex shrink-0 items-center gap-1">
                {blocked.includes(entry.id) ? (
                  <ConfirmDialog
                    title={i18next.t("agentConfig:Restore as a replacement?")}
                    description={i18next.t("agentConfig:Restore as a replacement description")}
                    onConfirm={() => restore(entry, true)}
                    disabled={busy === entry.id}
                  >
                    <Button variant="outline" size="sm" disabled={busy === entry.id}>
                      <RotateCcw className="size-4" />
                      {i18next.t("agentConfig:Restore as a replacement")}
                    </Button>
                  </ConfirmDialog>
                ) : (
                  <SimpleTooltip title={i18next.t("agentConfig:Restore")}>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={busy === entry.id}
                      onClick={() => restore(entry)}
                    >
                      <RotateCcw className="size-4" />
                      {i18next.t("agentConfig:Restore")}
                    </Button>
                  </SimpleTooltip>
                )}
                <ConfirmDialog
                  title={i18next.t("agentConfig:Delete for good?")}
                  description={i18next.t("agentConfig:Delete for good description")}
                  onConfirm={() => purge(entry.id)}
                  disabled={busy === entry.id}
                >
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    className="text-destructive"
                    aria-label={i18next.t("agentConfig:Delete for good?")}
                    disabled={busy === entry.id}
                  >
                    <Trash2 className="size-4" />
                  </Button>
                </ConfirmDialog>
              </div>
            </div>
          ))}
        </div>

        <DialogFooter>
          {entries.length > 0 ? (
            <ConfirmDialog
              title={i18next.t("agentConfig:Empty the recycle bin?")}
              description={i18next.t("agentConfig:Delete for good description")}
              onConfirm={() => purge("")}
              disabled={busy === "all"}
            >
              <Button variant="outline" className="text-destructive" disabled={busy === "all"}>
                {i18next.t("agentConfig:Empty the recycle bin")}
              </Button>
            </ConfirmDialog>
          ) : null}
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {i18next.t("general:Close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
