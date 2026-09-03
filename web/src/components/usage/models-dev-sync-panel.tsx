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
import {CloudDownload} from "lucide-react";
import i18next from "i18next";

import * as LlmPriceBackend from "@/backend/LlmPriceBackend";
import * as Setting from "@/Setting";
import {CodeText} from "@/components/shared/misc";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Switch} from "@/components/ui/switch";
import type {ModelsDevSync, ModelsDevSyncState} from "@/types";

/** How often an automatic sync runs. The catalogue changes when a vendor
 *  publishes a price, so even the shortest of these is often. */
const INTERVALS = [6, 24, 168];

function intervalLabel(hours: number) {
  if (hours === 24) {
    return i18next.t("usage:Once a day");
  }
  if (hours === 168) {
    return i18next.t("usage:Once a week");
  }
  return i18next.t("usage:Every {count} hours").replace("{count}", String(hours));
}

/** What one sync did, in the order it is worth reading: what is still unpriced
 *  first, because those are the models the Usage page cannot cost. */
function SyncReport({result}: {result: ModelsDevSync}) {
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
    />
  );
}

/**
 * The models.dev sync: whether it runs on its own, how often, and what the last
 * run did. A sync only ever reprices the models this machine has been seen
 * running, and never one that was edited by hand.
 */
export function ModelsDevSyncPanel({onSynced}: {onSynced: () => void}) {
  const [state, setState] = React.useState<ModelsDevSyncState | null>(null);
  const [syncing, setSyncing] = React.useState(false);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    LlmPriceBackend.getModelsDevSync()
      .then(res => {
        if (res.status === "ok") {
          setState(res.data);
        }
      })
      .catch(() => undefined);
  }, []);

  if (state === null) {
    return null;
  }

  const auto = state.mode === "auto";

  const schedule = (mode: ModelsDevSyncState["mode"], intervalHours: number) => {
    setSaving(true);
    LlmPriceBackend.updateModelsDevSync(mode, intervalHours)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setState(res.data);
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${failure}`))
      .then(() => setSaving(false));
  };

  // The state carries the report, so the panel reads the same whether the run
  // it is showing was asked for here or by the schedule.
  const syncNow = () => {
    setSyncing(true);
    LlmPriceBackend.syncModelsDevPrices()
      .then(res => {
        if (res.status !== "ok") {
          Setting.showMessage("error", res.msg || i18next.t("general:Failed to get data"));
        }
        onSynced();
        return LlmPriceBackend.getModelsDevSync().then(latest => {
          if (latest.status === "ok") {
            setState(latest.data);
          }
        });
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setSyncing(false));
  };

  const fact = (label: string, value: React.ReactNode) => (
    <div className="flex flex-col gap-0.5">
      <span className="text-muted-foreground text-xs">{i18next.t(label)}</span>
      <span className="text-sm">{value}</span>
    </div>
  );

  return (
    <Card className="gap-4 py-5">
      <CardHeader className="px-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle className="text-[15px]">{i18next.t("usage:Automatic sync")}</CardTitle>
            <CardDescription>{i18next.t("usage:Automatic sync description")}</CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {i18next.t(auto ? "usage:Sync is on" : "usage:Sync is off")}
            </span>
            <Switch
              checked={auto}
              disabled={saving}
              aria-label={i18next.t("usage:Automatic sync")}
              onCheckedChange={checked => schedule(checked ? "auto" : "off", state.intervalHours)}
            />
          </div>
        </div>
      </CardHeader>

      <CardContent className="flex flex-col gap-3 px-5">
        <div className="grid gap-3 sm:grid-cols-3">
          {fact(
            "usage:Last sync",
            state.syncedTime === ""
              ? i18next.t("usage:Never synced")
              : Setting.getFormattedDate(state.syncedTime),
          )}
          {fact(
            "usage:Next sync",
            auto
              ? state.nextTime === ""
                ? i18next.t("usage:Due now")
                : Setting.getFormattedDate(state.nextTime)
              : i18next.t("usage:Nothing is scheduled"),
          )}
          <div className="flex flex-col gap-1">
            <span className="text-muted-foreground text-xs">{i18next.t("usage:How often")}</span>
            <SimpleSelect
              size="sm"
              disabled={!auto || saving}
              value={String(state.intervalHours)}
              onChange={value => schedule(state.mode, Number(value))}
              options={INTERVALS.map(hours => ({label: intervalLabel(hours), value: String(hours)}))}
            />
          </div>
        </div>

        {state.error === "" ? null : (
          <MessageAlert title={i18next.t("usage:The last sync failed")} description={state.error} />
        )}

        {state.result === null ? null : <SyncReport result={state.result} />}

        <div className="flex justify-end">
          <Button variant="outline" onClick={syncNow} loading={syncing || state.running}>
            <CloudDownload />
            {i18next.t("usage:Sync from models.dev")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
