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
import {Search} from "lucide-react";
import i18next from "i18next";

import * as LlmPriceBackend from "@/backend/LlmPriceBackend";
import {Badge} from "@/components/ui/badge";
import {Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle} from "@/components/ui/dialog";
import {Input} from "@/components/ui/input";
import {Loading} from "@/components/shared/loading";
import {MessageAlert} from "@/components/ui/alert";
import {EmptyState} from "@/components/shared/empty-state";
import {formatRate} from "@/lib/usage";
import type {ModelsDevModel} from "@/types";

/** Long enough that typing a model name is one request rather than eight. */
const SEARCH_DELAY = 300;

/**
 * Picks a model out of the models.dev catalogue and hands back its rates, so a
 * price is copied from the catalogue rather than typed in off a vendor's page.
 * The search runs on the server: the catalogue is a few megabytes and only the
 * matches are worth sending here.
 */
export function ModelsDevPicker({
  open,
  onOpenChange,
  onPick,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (model: ModelsDevModel) => void;
}) {
  const [draft, setDraft] = React.useState("");
  const [models, setModels] = React.useState<ModelsDevModel[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    if (!open) {
      return undefined;
    }

    let live = true;
    setLoading(true);
    const timer = setTimeout(() => {
      LlmPriceBackend.searchModelsDevModels(draft)
        .then(res => {
          if (!live) {
            return;
          }
          if (res.status === "ok") {
            setModels(res.data ?? []);
            setError("");
          } else {
            setError(res.msg || i18next.t("general:Failed to get data"));
          }
        })
        .catch(failure => live && setError(failure.message || String(failure)))
        .then(() => live && setLoading(false));
    }, SEARCH_DELAY);

    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [open, draft]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("usage:Import from models.dev")}</DialogTitle>
          <DialogDescription>{i18next.t("usage:Import from models.dev description")}</DialogDescription>
        </DialogHeader>

        <div className="relative">
          <Search className="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
          <Input
            autoFocus
            className="pl-9"
            placeholder={i18next.t("usage:Search the catalogue")}
            value={draft}
            onChange={event => setDraft(event.target.value)}
          />
        </div>

        {error ? <MessageAlert title={error} /> : null}

        <div className="max-h-[24rem] overflow-y-auto">
          {loading ? (
            <Loading />
          ) : models.length === 0 ? (
            <EmptyState icon={Search} title={i18next.t("usage:No model matches")} />
          ) : (
            <div className="flex flex-col gap-1">
              {models.map(model => (
                <button
                  key={model.model}
                  type="button"
                  onClick={() => {
                    onPick(model);
                    onOpenChange(false);
                  }}
                  className="hover:bg-accent flex flex-col gap-1 rounded-md border p-2.5 text-left transition-colors"
                >
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="truncate font-mono text-xs">{model.model}</span>
                    <span className="text-muted-foreground truncate text-xs">{model.displayName}</span>
                    {model.releaseDate ? <Badge variant="muted">{model.releaseDate}</Badge> : null}
                  </div>
                  <div className="text-muted-foreground flex flex-wrap gap-3 text-xs tabular-nums">
                    <span>
                      {i18next.t("usage:Input")} {formatRate(model.input)}
                    </span>
                    <span>
                      {i18next.t("llm:Output")} {formatRate(model.output)}
                    </span>
                    <span>
                      {i18next.t("llm:Cache write")} {formatRate(model.cacheWrite)}
                    </span>
                    <span>
                      {i18next.t("llm:Cache read")} {formatRate(model.cacheRead)}
                    </span>
                    <span className="ml-auto">
                      {i18next
                        .t(model.providers === 1 ? "usage:One provider lists it" : "usage:{count} providers list it")
                        .replace("{count}", String(model.providers))}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
