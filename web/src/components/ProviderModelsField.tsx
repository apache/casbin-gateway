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
import {Check, RefreshCw, Search, X} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
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
  const [query, setQuery] = React.useState("");
  const [fetching, setFetching] = React.useState(false);
  const [fetched, setFetched] = React.useState<string[] | null>(null);

  const currentRequest = React.useRef<symbol | null>(null);
  React.useEffect(() => {
    setFetched(null);
    setQuery("");
    setFetching(false);
    return () => { currentRequest.current = null; };
  }, [provider.owner, provider.name, provider.type, provider.protocol, provider.baseUrl, provider.apiKey,
    provider.authMode, provider.loginId, provider.subscriptionVendor, provider.subscriptionAccount]);

  const models = provider.models ?? [];
  // A provider that forwards the caller's login sends no key upstream, so there
  // is nothing to list models with, and an empty list already means "any model".
  const canFetch = !servesAnyModel(provider) && provider.baseUrl !== "";

  const fetchModels = () => {
    const request = Symbol();
    currentRequest.current = request;
    setFetching(true);
    ProviderBackend.getProviderModels(provider)
      .then(res => {
        if (currentRequest.current !== request) {
          return;
        }
        setFetching(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to fetch models")}: ${res.msg}`);
          return;
        }
        const list = [...new Set(res.data ?? [])];
        setFetched(list);
        Setting.showMessage(
          "success",
          i18next.t("provider:Fetched {count} models").replace("{count}", `${list.length}`),
        );
      })
      .catch(error => {
        if (currentRequest.current !== request) {
          return;
        }
        setFetching(false);
        Setting.showMessage("error", `${i18next.t("provider:Failed to fetch models")}: ${error}`);
      });
  };

  const all = fetched ?? [];
  const needle = query.trim().toLowerCase();
  const visible = all.filter(model => needle === "" || model.toLowerCase().includes(needle));
  const shown = needle === "" ? `${all.length}` : `${visible.length} / ${all.length}`;

  const toggle = (model: string) => {
    onChange(models.includes(model) ? models.filter(item => item !== model) : [...models, model]);
  };

  // Bulk moves take what the search leaves in view, so a name that is not on the
  // screen is never selected or dropped by surprise.
  const selectAll = () => {
    onChange([...models, ...visible.filter(model => !models.includes(model))]);
  };

  // Only fetched names go, so anything typed by hand keeps its place.
  const clearFetched = () => {
    onChange(models.filter(model => !visible.includes(model)));
  };

  const closeList = () => {
    setFetched(null);
    setQuery("");
  };

  return (
    <Field
      label={
        <span className="flex items-center gap-2">
          {i18next.t("provider:Models")}
          {canFetch ? (
            <Button type="button" size="xs" variant="outline" aria-label={i18next.t("provider:Fetch models")} loading={fetching} onClick={fetchModels}>
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
              {i18next.t("provider:Models from upstream")} ({shown})
            </span>
            <div className="flex items-center gap-1">
              <Button type="button" size="xs" variant="ghost" onClick={selectAll} disabled={visible.length === 0}>
                {i18next.t(needle ? "provider:Select matching" : "provider:Select all")}
              </Button>
              <Button type="button" size="xs" variant="ghost" onClick={clearFetched} disabled={visible.length === 0}>
                {i18next.t(needle ? "provider:Clear matching" : "provider:Clear")}
              </Button>
              <Button
                type="button"
                size="icon-xs"
                variant="ghost"
                aria-label={i18next.t("general:Close")}
                onClick={closeList}
              >
                <X />
              </Button>
            </div>
          </div>
          <div className="relative">
            <Search className="text-muted-foreground pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2" />
            <Input
              value={query}
              onChange={event => setQuery(event.target.value)}
              // The picker sits inside the provider form, and Enter here would
              // otherwise submit it.
              onKeyDown={event => {
                if (event.key === "Enter") {
                  event.preventDefault();
                }
              }}
              aria-label={i18next.t("provider:Search models")}
              placeholder={i18next.t("provider:Search models")}
              className="h-8 pl-8 text-xs"
            />
          </div>
          <div className="scrollbar-thin flex max-h-40 flex-wrap gap-1 overflow-y-auto">
            {visible.map(model => {
              const selected = models.includes(model);
              return (
                <button key={model} type="button" aria-pressed={selected} onClick={() => toggle(model)}>
                  <Badge variant={selected ? "success" : "muted"} className="cursor-pointer">
                    {selected ? <Check /> : null}
                    {model}
                  </Badge>
                </button>
              );
            })}
            {needle === "" || visible.length > 0 ? null : (
              <p className="text-muted-foreground text-xs">{i18next.t("provider:No model matches")}</p>
            )}
          </div>
        </div>
      )}
    </Field>
  );
}
