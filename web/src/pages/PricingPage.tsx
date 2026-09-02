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
import {CircleDollarSign, CloudDownload, Pencil, Plus, RotateCcw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as LlmPriceBackend from "@/backend/LlmPriceBackend";
import * as Setting from "@/Setting";
import {PriceEditDialog} from "@/components/usage/price-edit-dialog";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {CodeText, UnauthorizedResult} from "@/components/shared/misc";
import {MessageAlert} from "@/components/ui/alert";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {formatRate} from "@/lib/usage";
import type {Account, LlmPriceEntry, LlmPriceSource, LlmPriceView, ModelsDevSync} from "@/types";

const SOURCE_TONE: Record<LlmPriceSource, BadgeVariant> = {
  "built-in": "muted",
  file: "info",
  "models.dev": "success",
  manual: "warning",
};

const SOURCE_LABEL: Record<LlmPriceSource, string> = {
  "built-in": "usage:Built in",
  file: "usage:Pricing file",
  "models.dev": "usage:models.dev",
  manual: "usage:Edited by hand",
};

/** What one sync did, in the order it is worth reading: what is still unpriced
 *  first, because those are the models the Usage page cannot cost. */
function SyncReport({result, onDismiss}: {result: ModelsDevSync; onDismiss: () => void}) {
  const line = (label: string, models: string[]) =>
    models.length === 0 ? null : (
      <div className="flex min-w-0 flex-wrap items-baseline gap-1.5">
        <span className="shrink-0 text-xs font-medium">{i18next.t(label)}</span>
        {models.map(model => (
          <CodeText key={model}>{model}</CodeText>
        ))}
      </div>
    );

  return (
    <MessageAlert
      variant={result.missing.length > 0 ? "warning" : "success"}
      title={i18next
        .t("usage:Priced {updated} of {considered} models")
        .replace("{updated}", String(result.updated.length))
        .replace("{considered}", String(result.considered.length))}
      description={
        <div className="flex flex-col gap-2">
          {line("usage:Still unpriced", result.missing)}
          {line("usage:Left as edited", result.skipped)}
          {line("usage:Updated", result.updated)}
          <span className="text-xs">
            {i18next
              .t("usage:Read from a catalogue of {count} models")
              .replace("{count}", result.catalogue.toLocaleString())}
          </span>
        </div>
      }
      action={
        <Button size="sm" variant="ghost" onClick={onDismiss}>
          {i18next.t("general:Close")}
        </Button>
      }
    />
  );
}

export default function PricingPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [prices, setPrices] = React.useState<LlmPriceView[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [syncing, setSyncing] = React.useState(false);
  const [sync, setSync] = React.useState<ModelsDevSync | null>(null);
  const [editing, setEditing] = React.useState<LlmPriceView | null>(null);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  const load = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    LlmPriceBackend.getLlmPrices()
      .then(res => {
        if (res.status === "ok") {
          setPrices(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)))
      .then(() => setLoading(false));
  }, [isAdmin]);

  React.useEffect(() => {
    load();
  }, [load]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const save = (entry: Partial<LlmPriceEntry>) => {
    setSaving(true);
    LlmPriceBackend.updateLlmPrice(entry)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          setDialogOpen(false);
          load();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${failure}`))
      .then(() => setSaving(false));
  };

  const reset = (model: string) =>
    LlmPriceBackend.deleteLlmPrice(model)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          load();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${failure}`));

  const runSync = () => {
    setSyncing(true);
    setSync(null);
    LlmPriceBackend.syncModelsDevPrices()
      .then(res => {
        if (res.status === "ok" && res.data) {
          setSync(res.data);
          load();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setSyncing(false));
  };

  const open = (price: LlmPriceView | null) => {
    setEditing(price);
    setDialogOpen(true);
  };

  const rate = (key: "input" | "output" | "cacheWrite" | "cacheRead" | "cacheWrite1h"): Column<LlmPriceView> => ({
    key: key,
    title: i18next.t(
      {
        input: "usage:Input",
        output: "llm:Output",
        cacheWrite: "llm:Cache write",
        cacheRead: "llm:Cache read",
        cacheWrite1h: "usage:Cache write 1h",
      }[key],
    ),
    align: "right",
    width: "110px",
    sorter: (left, right) => left[key] - right[key],
    render: (_value, price) => (
      <span className={price[key] === 0 ? "text-muted-foreground tabular-nums" : "tabular-nums"}>
        {formatRate(price[key])}
      </span>
    ),
  });

  const columns: Column<LlmPriceView>[] = [
    {
      key: "model",
      title: i18next.t("agent:Model"),
      sorter: (left, right) => left.model.localeCompare(right.model),
      render: (_value, price) => (
        <div className="flex min-w-0 flex-col">
          <span className="truncate font-mono text-xs">{price.model}</span>
          {price.displayName ? (
            <span className="text-muted-foreground truncate text-xs">{price.displayName}</span>
          ) : null}
        </div>
      ),
    },
    rate("input"),
    rate("output"),
    rate("cacheWrite"),
    rate("cacheRead"),
    rate("cacheWrite1h"),
    {
      key: "source",
      title: i18next.t("usage:Source"),
      width: "130px",
      render: (_value, price) => (
        <Badge variant={SOURCE_TONE[price.source] ?? "muted"}>{i18next.t(SOURCE_LABEL[price.source] ?? price.source)}</Badge>
      ),
    },
    {
      key: "actions",
      title: "",
      width: "96px",
      align: "right",
      render: (_value, price) => (
        <div className="flex items-center justify-end gap-1">
          <SimpleTooltip title={i18next.t("general:Edit")}>
            <Button size="icon" variant="ghost" onClick={() => open(price)}>
              <Pencil />
            </Button>
          </SimpleTooltip>
          {price.source === "built-in" || price.source === "file" ? null : (
            <ConfirmDialog
              title={
                price.overridden
                  ? i18next.t("usage:Put the built-in price back?")
                  : i18next.t("usage:Delete this price?")
              }
              description={
                price.overridden
                  ? i18next.t("usage:Put the built-in price back detail")
                  : i18next.t("usage:Delete this price detail")
              }
              onConfirm={() => reset(price.model)}
            >
              <Button size="icon" variant="ghost">
                {price.overridden ? <RotateCcw /> : <Trash2 />}
              </Button>
            </ConfirmDialog>
          )}
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("usage:Model pricing")}
        description={i18next.t("usage:Model pricing description")}
        actions={
          <>
            <Button variant="outline" onClick={runSync} loading={syncing}>
              <CloudDownload />
              {i18next.t("usage:Sync from models.dev")}
            </Button>
            <Button onClick={() => open(null)}>
              <Plus />
              {i18next.t("usage:Add a price")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert title={error} /> : null}
      {sync ? <SyncReport result={sync} onDismiss={() => setSync(null)} /> : null}

      <DataTable
        searchable
        searchPlaceholder={i18next.t("agent:Model")}
        columns={columns}
        dataSource={prices}
        rowKey={price => price.model}
        loading={loading}
        pageSize={25}
        emptyIcon={CircleDollarSign}
        emptyText={i18next.t("usage:No prices yet")}
      />

      <PriceEditDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        price={editing}
        onSave={save}
        saving={saving}
      />
    </PageContainer>
  );
}
