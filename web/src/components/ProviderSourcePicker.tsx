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
import {ArrowDownToLine, KeyRound, Link2, LogIn, Search, Settings2} from "lucide-react";
import i18next from "i18next";

import {ProviderIcon} from "@/components/ProviderIcon";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {
  providerSources,
  customSource,
  subscriptionSource,
  vendorCategories,
  type ProviderSource,
  type VendorCategory,
} from "@/lib/providers";

/**
 * The title and the line under it. A vendor card is named after the vendor and
 * says where its requests go; the two cards that are not a vendor say what they
 * are instead, because their base URL would not.
 */
export function sourceTitle(source: ProviderSource) {
  if (source.key === subscriptionSource) {
    return i18next.t("provider:Claude subscription");
  }
  if (source.key === customSource) {
    return i18next.t("provider:Custom vendor");
  }
  return source.label;
}

function sourceDetail(source: ProviderSource) {
  if (source.key === subscriptionSource) {
    return i18next.t("provider:Claude subscription detail");
  }
  if (source.key === customSource) {
    return i18next.t("provider:Custom vendor detail");
  }
  return source.provider.baseUrl ?? "";
}

function categoryTitle(category: VendorCategory) {
  return {
    official: i18next.t("provider:Model vendors"),
    platform: i18next.t("provider:Inference platforms"),
    aggregator: i18next.t("provider:Aggregators"),
  }[category];
}

function SourceCard({source, onPick}: {source: ProviderSource; onPick: (source: ProviderSource) => void}) {
  // Only reached when the vendor's own mark will not load.
  const Icon =
    source.key === subscriptionSource ? LogIn : source.key === customSource ? Settings2 : KeyRound;

  return (
    <button
      type="button"
      onClick={() => onPick(source)}
      className="hover:border-primary hover:bg-accent/40 flex flex-col items-start gap-1 rounded-lg border p-4 text-left transition-colors"
    >
      <span className="flex items-center gap-2 text-sm font-medium">
        <ProviderIcon
          baseUrl={source.website ?? source.provider.baseUrl}
          size={16}
          fallback={<Icon className="size-4 shrink-0" />}
        />
        {sourceTitle(source)}
      </span>
      <span className="text-muted-foreground break-all text-xs">{sourceDetail(source)}</span>
    </button>
  );
}

/**
 * The first step of creating a provider: where its credentials come from. Picking
 * a card fills in the type, base URL, models and authentication mode, so a
 * subscription needs nothing typed and a vendor needs only its key.
 *
 * The vendors are grouped by who they are — the maker of the models, a platform
 * hosting other people's, a site reselling many — because that is the thing
 * worth knowing about one, and there are too many of them to read as one list.
 */
export function ProviderSourcePicker({
  onPick,
  onLink,
  onCcSwitch,
}: {
  onPick: (source: ProviderSource) => void;
  /** A vendor's "add this provider" link, or New API connection info. */
  onLink?: (link: string) => Promise<void>;
  /** Opens the import page, where a whole CC Switch install is brought over. */
  onCcSwitch?: () => void;
}) {
  const [query, setQuery] = React.useState("");
  const [link, setLink] = React.useState("");
  const [importing, setImporting] = React.useState(false);

  const importLink = () => {
    if (onLink === undefined || link.trim() === "") {
      return;
    }
    setImporting(true);
    onLink(link.trim()).then(() => setImporting(false));
  };

  const needle = query.trim().toLowerCase();
  const matches = (source: ProviderSource) =>
    needle === "" ||
    sourceTitle(source).toLowerCase().includes(needle) ||
    (source.provider.baseUrl ?? "").toLowerCase().includes(needle);

  // The two cards that are not a vendor lead, without a heading of their own:
  // they are the answers for a vendor that is not on the list at all.
  const leading = providerSources.filter(source => source.category === undefined).filter(matches);
  const groups = vendorCategories
    .map(category => ({
      category: category,
      sources: providerSources.filter(source => source.category === category).filter(matches),
    }))
    .filter(group => group.sources.length > 0);

  return (
    <div className="grid gap-4">
      <div className="bg-background sticky top-0 z-10 pb-1">
        <div className="relative">
          <Search className="text-muted-foreground pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2" />
          <Input
            value={query}
            onChange={event => setQuery(event.target.value)}
            placeholder={i18next.t("provider:Search vendors")}
            className="pl-9"
          />
        </div>
      </div>

      {onLink === undefined ? null : (
        <div className="grid gap-1.5 rounded-lg border border-dashed p-3">
          <span className="flex items-center gap-2 text-sm font-medium">
            <Link2 className="size-4" />
            {i18next.t("provider:Paste a link or connection info")}
          </span>
          <div className="flex flex-wrap gap-2">
            <Input
              value={link}
              onChange={event => setLink(event.target.value)}
              onKeyDown={event => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  importLink();
                }
              }}
              placeholder={"ccswitch://v1/import?... or {\"_type\":\"newapi_channel_conn\",...}"}
              className="min-w-0 flex-1"
            />
            <Button type="button" variant="outline" loading={importing} onClick={importLink}>
              {i18next.t("provider:Read it")}
            </Button>
          </div>
          <span className="text-muted-foreground text-xs">
            {i18next.t("provider:Paste a link or connection info hint")}
          </span>
        </div>
      )}

      {onCcSwitch === undefined ? null : (
        <div className="grid gap-1.5 rounded-lg border border-dashed p-3">
          <span className="flex items-center gap-2 text-sm font-medium">
            <ArrowDownToLine className="size-4" />
            {i18next.t("provider:Coming from CC Switch")}
          </span>
          <span className="text-muted-foreground text-xs">
            {i18next.t("provider:Coming from CC Switch hint")}
          </span>
          <div>
            <Button type="button" variant="outline" onClick={onCcSwitch}>
              {i18next.t("provider:Bring it all over")}
            </Button>
          </div>
        </div>
      )}

      {leading.length === 0 ? null : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {leading.map(source => (
            <SourceCard key={source.key} source={source} onPick={onPick} />
          ))}
        </div>
      )}

      {groups.map(group => (
        <div key={group.category} className="grid gap-2">
          <span className="text-muted-foreground text-xs font-medium uppercase tracking-wide">
            {categoryTitle(group.category)}
          </span>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {group.sources.map(source => (
              <SourceCard key={source.key} source={source} onPick={onPick} />
            ))}
          </div>
        </div>
      ))}

      {leading.length === 0 && groups.length === 0 ? (
        <p className="text-muted-foreground py-4 text-center text-sm">
          {i18next.t("provider:No vendor matches")}
        </p>
      ) : null}
    </div>
  );
}
