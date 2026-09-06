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
import {Check, RefreshCw, X} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {TagsInput} from "@/components/ui/tags-input";
import {modelPresets, modelsPlaceholder, servesAnyModel} from "@/lib/providers";
import type {Provider} from "@/types";

/**
 * The Models field of both provider forms: the names can be typed, or asked of
 * the upstream itself, which every OpenAI- and Anthropic-compatible API answers
 * at its models endpoint. What comes back is offered as chips to pick from
 * rather than written straight into the field: an aggregator lists hundreds of
 * models, most of which this provider is not meant to serve.
 */
export function ProviderModelsField({
  provider,
  hint,
  className,
  onChange,
}: {
  provider: Provider;
  hint?: React.ReactNode;
  className?: string;
  onChange: (models: string[]) => void;
}) {
  const [fetching, setFetching] = React.useState(false);
  const [fetched, setFetched] = React.useState<string[] | null>(null);

  // The list belongs to one upstream, so it stops being an answer as soon as
  // the form points somewhere else.
  React.useEffect(() => {
    setFetched(null);
  }, [provider.baseUrl, provider.type]);

  const models = provider.models ?? [];
  // A provider that forwards the caller's login sends no key upstream, so there
  // is nothing to list models with, and an empty list already means "any model".
  const canFetch = !servesAnyModel(provider) && provider.baseUrl !== "";

  const fetchModels = () => {
    setFetching(true);
    ProviderBackend.getProviderModels(provider)
      .then(res => {
        setFetching(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to fetch models")}: ${res.msg}`);
          return;
        }
        const list = res.data ?? [];
        setFetched(list);
        Setting.showMessage(
          "success",
          i18next.t("provider:Fetched {count} models").replace("{count}", `${list.length}`),
        );
      })
      .catch(error => {
        setFetching(false);
        Setting.showMessage("error", `${i18next.t("provider:Failed to fetch models")}: ${error}`);
      });
  };

  const toggle = (model: string) => {
    onChange(models.includes(model) ? models.filter(item => item !== model) : [...models, model]);
  };

  const selectAll = () => {
    onChange([...models, ...(fetched ?? []).filter(model => !models.includes(model))]);
  };

  // Only the fetched names are dropped, so anything typed by hand survives.
  const clearFetched = () => {
    onChange(models.filter(model => !(fetched ?? []).includes(model)));
  };

  return (
    <Field
      label={
        <span className="flex items-center gap-2">
          {i18next.t("provider:Models")}
          {canFetch ? (
            <Button type="button" size="xs" variant="outline" loading={fetching} onClick={fetchModels}>
              <RefreshCw />
              {i18next.t("provider:Fetch models")}
            </Button>
          ) : null}
        </span>
      }
      hint={hint}
      className={className}
    >
      <TagsInput
        value={models}
        placeholder={modelsPlaceholder(provider.type)}
        suggestions={fetched ?? modelPresets(provider.type)}
        onChange={onChange}
      />
      {/* An empty list only means "any model" for a provider that forwards the
          caller's login. Anywhere else it means the router will never pick this
          provider, which is worth saying before the form is submitted. */}
      {models.length === 0 && !servesAnyModel(provider) ? (
        <p className="text-warning text-xs">{i18next.t("provider:No models hint")}</p>
      ) : null}
      {fetched === null ? null : (
        <div className="grid gap-2 rounded-md border p-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-muted-foreground text-xs">
              {i18next.t("provider:Models from upstream")} ({fetched.length})
            </span>
            <div className="flex items-center gap-1">
              <Button type="button" size="xs" variant="ghost" onClick={selectAll}>
                {i18next.t("provider:Select all")}
              </Button>
              <Button type="button" size="xs" variant="ghost" onClick={clearFetched}>
                {i18next.t("provider:Clear")}
              </Button>
              <Button
                type="button"
                size="icon-xs"
                variant="ghost"
                aria-label={i18next.t("general:Close")}
                onClick={() => setFetched(null)}
              >
                <X />
              </Button>
            </div>
          </div>
          <div className="scrollbar-thin flex max-h-40 flex-wrap gap-1 overflow-y-auto">
            {fetched.map(model => {
              const selected = models.includes(model);
              return (
                <button key={model} type="button" onClick={() => toggle(model)}>
                  <Badge variant={selected ? "success" : "muted"} className="cursor-pointer">
                    {selected ? <Check /> : null}
                    {model}
                  </Badge>
                </button>
              );
            })}
          </div>
        </div>
      )}
    </Field>
  );
}
