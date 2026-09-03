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

import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Switch} from "@/components/ui/switch";
import type {ImportChange, ImportReport, Snapshot, SnapshotCounts, SnapshotScope} from "@/types";

/** The sections a snapshot is made of, in the order they are worth reading. */
export const SNAPSHOT_SECTIONS = [
  {key: "providers", label: "setting:Providers"},
  {key: "agents", label: "setting:Agent routing"},
  {key: "probeCases", label: "setting:Probe cases"},
  {key: "llmPrices", label: "setting:Model prices"},
  {key: "setting", label: "setting:Settings"},
] as const;

export type SnapshotSection = (typeof SNAPSHOT_SECTIONS)[number]["key"];

export const FULL_SCOPE: SnapshotScope = {
  providers: true,
  agents: true,
  probeCases: true,
  llmPrices: true,
  setting: true,
  secrets: true,
};

export function sectionLabel(key: string) {
  const section = SNAPSHOT_SECTIONS.find(entry => entry.key === key);
  return section ? i18next.t(section.label) : key;
}

/** Whether a scope would carry anything at all. */
export function isScopeEmpty(scope: SnapshotScope) {
  return SNAPSHOT_SECTIONS.every(section => !scope[section.key]);
}

/**
 * The section toggles both the export and the import are driven by. The import
 * passes `available` so a section the file does not carry is shown as off and
 * cannot be turned on: what is not in the file cannot be imported from it.
 */
export function ScopePicker({
  scope,
  onChange,
  available,
  disabled = false,
  secretsLabel,
  secretsHint,
}: {
  scope: SnapshotScope;
  onChange: (scope: SnapshotScope) => void;
  available?: SnapshotCounts;
  disabled?: boolean;
  secretsLabel: string;
  secretsHint: string;
}) {
  const countOf = (key: SnapshotSection) => {
    if (available === undefined) {
      return undefined;
    }
    return key === "agents" ? available.agents + available.agentInstances : available[key];
  };

  const row = (
    key: SnapshotSection | "secrets",
    label: React.ReactNode,
    hint: React.ReactNode,
    off: boolean,
  ) => (
    <div key={key} className="flex items-start justify-between gap-3 py-2">
      <div className="min-w-0">
        <div className="text-sm">{label}</div>
        {hint ? <div className="text-muted-foreground text-xs">{hint}</div> : null}
      </div>
      <Switch
        className="mt-0.5 shrink-0"
        checked={scope[key] && !off}
        disabled={disabled || off}
        aria-label={typeof label === "string" ? label : key}
        onCheckedChange={checked => onChange({...scope, [key]: checked})}
      />
    </div>
  );

  return (
    <div className="divide-y">
      {SNAPSHOT_SECTIONS.map(section => {
        const count = countOf(section.key);
        const off = count === 0;
        return row(
          section.key,
          i18next.t(section.label),
          count === undefined
            ? null
            : off
              ? i18next.t("setting:Not in this file")
              : i18next.t("setting:{count} in this file").replace("{count}", String(count)),
          off,
        );
      })}
      {row("secrets", secretsLabel, secretsHint, false)}
    </div>
  );
}

/** How much a snapshot file holds, counted in the browser: a file that arrived
 *  from somewhere else has no counts of its own. */
export function countsOf(snapshot: Snapshot | null): SnapshotCounts {
  const length = (section: string) => (snapshot?.[section] as unknown[] | undefined)?.length ?? 0;
  return {
    providers: length("providers"),
    agents: length("agents"),
    agentInstances: length("agentInstances"),
    probeCases: length("probeCases"),
    llmPrices: length("llmPrices"),
    setting: snapshot?.setting === undefined || snapshot.setting === null ? 0 : 1,
  };
}

/** What one snapshot holds, as the row of badges a listing has room for. */
export function CountsBadges({counts}: {counts: SnapshotCounts}) {
  const entries = SNAPSHOT_SECTIONS.map(section => ({
    key: section.key,
    label: i18next.t(section.label),
    count: section.key === "agents" ? counts.agents + counts.agentInstances : counts[section.key],
  })).filter(entry => entry.count > 0);

  if (entries.length === 0) {
    return <span className="text-muted-foreground text-xs">{i18next.t("setting:Nothing in it")}</span>;
  }

  return (
    <div className="flex flex-wrap gap-1">
      {entries.map(entry => (
        <Badge key={entry.key} variant="muted" className="font-normal">
          {entry.label} {entry.count}
        </Badge>
      ))}
    </div>
  );
}

const ACTION_TONES: Record<ImportChange["action"], BadgeVariant> = {
  add: "success",
  update: "info",
  delete: "warning",
  skip: "muted",
  fail: "danger",
};

function actionLabel(action: ImportChange["action"]) {
  switch (action) {
  case "add":
    return i18next.t("setting:Added");
  case "update":
    return i18next.t("setting:Updated");
  case "delete":
    return i18next.t("setting:Deleted");
  case "skip":
    return i18next.t("setting:Skipped");
  default:
    return i18next.t("setting:Failed");
  }
}

/**
 * What an import did, or what a dry run says it would do. The counts are the
 * summary; the rows underneath are what makes a "replace" reviewable, since
 * that is the mode that deletes.
 */
export function ImportReportView({report}: {report: ImportReport}) {
  const totals: {action: ImportChange["action"]; count: number}[] = [
    {action: "add", count: report.added},
    {action: "update", count: report.updated},
    {action: "delete", count: report.deleted},
    {action: "skip", count: report.skipped},
    {action: "fail", count: report.failed},
  ];

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap gap-1.5">
        {totals
          .filter(total => total.count > 0)
          .map(total => (
            <Badge key={total.action} variant={ACTION_TONES[total.action]} className="font-normal">
              {actionLabel(total.action)} {total.count}
            </Badge>
          ))}
        {report.changes.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("setting:Nothing would change")}</span>
        ) : null}
      </div>

      {report.changes.length === 0 ? null : (
        <div className="max-h-56 overflow-y-auto rounded-md border">
          {report.changes.map((change, index) => (
            <div
              key={`${change.section}-${change.id}-${index}`}
              className="flex items-start gap-2 border-b px-3 py-1.5 text-xs last:border-b-0"
            >
              <Badge variant={ACTION_TONES[change.action]} className="shrink-0 font-normal">
                {actionLabel(change.action)}
              </Badge>
              <span className="text-muted-foreground shrink-0">{sectionLabel(change.section)}</span>
              <span className="min-w-0 flex-1 truncate font-mono">{change.id}</span>
              {change.detail ? <span className="text-muted-foreground min-w-0 flex-1">{change.detail}</span> : null}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** Saves a snapshot as a file, which is the whole of what "export" means here. */
export function downloadSnapshot(snapshot: Snapshot, name: string) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(snapshot, null, 2)], {type: "application/json"}));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}
