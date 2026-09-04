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
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {Loading} from "@/components/shared/loading";
import {Markdown} from "@/components/shared/markdown";
import {CodeText} from "@/components/shared/misc";
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
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {Textarea} from "@/components/ui/textarea";
import {formatBytes, formatModified} from "@/lib/agent-configs";
import {cn} from "@/lib/utils";
import type {AgentConfigItem} from "@/types";

/** The instruction file on screen, and the agent it belongs to. */
export interface PromptTarget {
  item: AgentConfigItem;
  agentName: string;
}

/** Which pane is on screen when there is only room for one of them. */
type Pane = "edit" | "preview";

/**
 * Edits the instructions one agent reads before every session. The file is
 * loaded when the dialog opens rather than taken from the listing, so a save
 * writes over what is on disk now and not over a stale scan.
 *
 * Instructions are Markdown, so the rendered half is kept beside the source and
 * follows every keystroke: the headings and lists an agent reads are the point
 * of the file, and they are hard to judge from the source alone.
 */
export function PromptDialog({
  target,
  onOpenChange,
  onSaved,
}: {
  target: PromptTarget | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void;
}) {
  const [content, setContent] = React.useState("");
  const [loaded, setLoaded] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [pane, setPane] = React.useState<Pane>("edit");
  const [error, setError] = React.useState("");

  const item = target?.item;

  React.useEffect(() => {
    if (!item) {
      return;
    }

    let current = true;
    setContent("");
    setLoaded("");
    setPane("edit");
    setError("");
    setLoading(true);
    AgentConfigBackend.getAgentConfigItem(item.agentId, item.owner, item.kind, item.name)
      .then(res => {
        if (!current) {
          return;
        }
        if (res.status === "ok") {
          setContent(res.data?.content ?? "");
          setLoaded(res.data?.content ?? "");
        } else {
          setError(res.msg || i18next.t("agentConfig:Failed to read this item"));
        }
      })
      .catch(err => current && setError(err.message || String(err)))
      .then(() => current && setLoading(false));

    return () => {
      current = false;
    };
  }, [item]);

  const save = () => {
    if (!item) {
      return;
    }

    setSaving(true);
    setError("");
    AgentConfigBackend.saveAgentConfigPrompt(item.agentId, item.owner, content)
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to save these instructions"));
          return;
        }
        Setting.showMessage("success", i18next.t("agentConfig:Instructions saved"));
        onSaved();
        onOpenChange(false);
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setSaving(false));
  };

  return (
    <Dialog open={Boolean(target)} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-3 overflow-hidden sm:max-w-5xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <span className="truncate">{item?.name}</span>
            <Badge variant="muted">{target?.agentName}</Badge>
          </DialogTitle>
          <DialogDescription>{i18next.t("agentConfig:Prompt editor hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col gap-3">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
            <span className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="text-muted-foreground">{i18next.t("general:Path")}</span>
              <CodeText copyable>{item?.path}</CodeText>
              {item?.missing ? (
                <Badge variant="muted">{i18next.t("agentConfig:Not written yet")}</Badge>
              ) : (
                <span className="text-muted-foreground">
                  {formatBytes(item?.bytes)}
                  {formatModified(item?.modified) ? ` · ${formatModified(item?.modified)}` : ""}
                </span>
              )}
            </span>

            {/* Both panes fit side by side on a wide screen, so the switch is
                only there for the screens where they do not. */}
            <Tabs value={pane} onValueChange={value => setPane(value as Pane)} className="md:hidden">
              <TabsList>
                <TabsTrigger value="edit">{i18next.t("general:Edit")}</TabsTrigger>
                <TabsTrigger value="preview">{i18next.t("agentConfig:Preview")}</TabsTrigger>
              </TabsList>
            </Tabs>
          </div>

          {error ? <MessageAlert description={error} /> : null}
          {loading ? (
            <Loading />
          ) : (
            <div className="grid min-h-0 flex-1 gap-3 md:grid-cols-2">
              <Textarea
                value={content}
                onChange={event => setContent(event.target.value)}
                spellCheck={false}
                className={cn(
                  "scrollbar-thin h-full min-h-[45vh] resize-none font-mono text-xs",
                  pane === "preview" && "hidden md:block",
                )}
                placeholder={i18next.t("agentConfig:Prompt placeholder")}
              />
              <div
                className={cn(
                  "scrollbar-thin bg-muted/20 h-full min-h-[45vh] overflow-y-auto rounded-md border p-3",
                  pane === "edit" && "hidden md:block",
                )}
              >
                {content.trim() === "" ? (
                  <p className="text-muted-foreground text-sm">
                    {i18next.t("agentConfig:Nothing to preview yet")}
                  </p>
                ) : (
                  <Markdown content={content} />
                )}
              </div>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {i18next.t("general:Cancel")}
          </Button>
          <Button onClick={save} disabled={loading || saving || content === loaded}>
            {i18next.t("general:Save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
