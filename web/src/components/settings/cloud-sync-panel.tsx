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
import {CloudDownload, FolderSymlink, PlugZap, RefreshCw} from "lucide-react";
import i18next from "i18next";

import * as CloudSyncBackend from "@/backend/CloudSyncBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {PasswordInput} from "@/components/shared/password-input";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import type {CloudSyncDirection, CloudSyncField, CloudSyncKind, CloudSyncState} from "@/types";

/** The kinds, their fields and their hints are described by the server, so a
 *  storage added there needs nothing here. The English text it sends doubles as
 *  the translation key, and stands as written when there is no translation. */
function text(key: string) {
  return key === "" ? "" : i18next.t(`setting:${key}`, {defaultValue: key});
}

/** One field of the chosen kind, drawn from what the server said it is. */
function KindField({
  field,
  value,
  disabled,
  onChange,
}: {
  field: CloudSyncField;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const id = `cloud-sync-${field.name}`;

  if (field.type === "switch") {
    return (
      <Field label={text(field.label)} hint={text(field.hint)}>
        <div className="flex h-9 items-center">
          <Switch
            id={id}
            checked={value === "true"}
            disabled={disabled}
            aria-label={text(field.label)}
            onCheckedChange={checked => onChange(checked ? "true" : "false")}
          />
        </div>
      </Field>
    );
  }

  const Control = field.type === "secret" ? PasswordInput : Input;
  return (
    <Field label={text(field.label)} htmlFor={id} required={field.required} hint={text(field.hint)}>
      <Control
        id={id}
        value={value}
        placeholder={field.placeholder}
        disabled={disabled}
        onChange={event => onChange(event.target.value)}
      />
    </Field>
  );
}

/**
 * Where a copy of the backups goes: a folder a desktop client already syncs, a
 * WebDAV share, or an S3 bucket. The snapshots are immutable files named after
 * the moment they were taken, so a sync has nothing to merge - each side ends
 * up holding what the other had, and a machine that has pulled them can restore
 * any of them from the list above.
 */
export function CloudSyncPanel({onSynced}: {onSynced: () => void}) {
  const [state, setState] = React.useState<CloudSyncState | null>(null);
  const [kind, setKind] = React.useState("");
  // Kept per kind, so trying another storage and coming back does not mean
  // typing the credentials again.
  const [options, setOptions] = React.useState<Record<string, Record<string, string>>>({});
  const [busy, setBusy] = React.useState(false);
  const [tested, setTested] = React.useState("");

  const load = React.useCallback(() => {
    CloudSyncBackend.getCloudSyncState()
      .then(res => {
        if (res.status === "ok") {
          setState(res.data);
          setKind(current => current || res.data.kind || res.data.kinds[0]?.name || "");
          setOptions(current => ({...current, [res.data.kind]: res.data.options ?? {}}));
        }
      })
      .catch(() => undefined);
  }, []);

  React.useEffect(load, [load]);

  if (state === null) {
    return null;
  }

  const auto = state.mode === "auto";
  const kinds = state.kinds ?? [];
  const current: CloudSyncKind | undefined = kinds.find(item => item.name === kind);
  const values = options[kind] ?? {};

  const setValue = (name: string, value: string) =>
    setOptions(all => ({...all, [kind]: {...(all[kind] ?? {}), [name]: value}}));

  const apply = (promise: Promise<{status: string; msg: string; data?: CloudSyncState}>, done?: () => void) => {
    setBusy(true);
    return promise
      .then(res => {
        if (res.status === "ok" && res.data) {
          const saved = res.data;
          setState(saved);
          setOptions(all => ({...all, [saved.kind]: saved.options ?? {}}));
          done?.();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setBusy(false));
  };

  const save = (mode: CloudSyncState["mode"]) => {
    setTested("");
    return apply(CloudSyncBackend.updateCloudSync(mode, kind, values), () =>
      Setting.showMessage("success", i18next.t("general:Successfully saved")),
    );
  };

  const test = () => {
    setBusy(true);
    setTested("");
    return CloudSyncBackend.testCloudSync(kind, values)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setTested(
            i18next
              .t("setting:Reached {target}, {count} files there")
              .replace("{target}", res.data.target)
              .replace("{count}", String(res.data.files?.length ?? 0)),
          );
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setBusy(false));
  };

  // A run that pulled anything changes the list of backups above, so the page
  // is told rather than left a reload behind.
  const run = (direction: CloudSyncDirection) =>
    apply(CloudSyncBackend.runCloudSync(direction), () => {
      Setting.showMessage("success", i18next.t("setting:The sync finished"));
      onSynced();
    });

  const result = state.result;
  const summary =
    result === null
      ? ""
      : i18next
        .t("setting:{up} sent, {down} received, {gone} dropped")
        .replace("{up}", String(result.uploaded?.length ?? 0))
        .replace("{down}", String(result.downloaded?.length ?? 0))
        .replace("{gone}", String(result.removed?.length ?? 0));

  return (
    <Card className="gap-4 py-5">
      <CardHeader className="px-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle className="text-[15px]">{i18next.t("setting:Cloud sync")}</CardTitle>
            <CardDescription>{i18next.t("setting:Cloud sync description")}</CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {i18next.t(auto ? "setting:Syncing is on" : "setting:Syncing is off")}
            </span>
            <Switch
              checked={auto}
              disabled={busy || kind === ""}
              aria-label={i18next.t("setting:Cloud sync")}
              onCheckedChange={checked => save(checked ? "auto" : "off")}
            />
          </div>
        </div>
      </CardHeader>

      <CardContent className="flex flex-col gap-4 px-5">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={i18next.t("setting:Storage")} hint={text(current?.description ?? "")}>
            <SimpleSelect
              value={kind}
              disabled={busy}
              onChange={value => {
                setKind(value);
                setTested("");
              }}
              options={kinds.map(item => ({label: text(item.displayName), value: item.name}))}
            />
          </Field>

          {(current?.fields ?? []).map(field => (
            <KindField
              key={field.name}
              field={field}
              value={values[field.name] ?? ""}
              disabled={busy}
              onChange={value => setValue(field.name, value)}
            />
          ))}
        </div>

        {kind === "folder" && (state.folders ?? []).length > 0 ? (
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-muted-foreground text-xs">{i18next.t("setting:Found on this machine")}</span>
            {state.folders.map(folder => (
              <Button
                key={folder.path}
                variant="outline"
                size="xs"
                disabled={busy}
                onClick={() => setValue("path", folder.suggested)}
              >
                <FolderSymlink />
                {folder.name}
              </Button>
            ))}
          </div>
        ) : null}

        <div className="grid gap-3 sm:grid-cols-2">
          <div className="flex flex-col gap-0.5">
            <span className="text-muted-foreground text-xs">{i18next.t("setting:Copies go to")}</span>
            <span className="text-sm break-all">
              {state.target === "" ? i18next.t("setting:Nowhere yet") : state.target}
            </span>
          </div>
          <div className="flex flex-col gap-0.5">
            <span className="text-muted-foreground text-xs">{i18next.t("setting:Last sync")}</span>
            <span className="flex flex-wrap items-center gap-2 text-sm">
              {state.syncedTime === "" ? i18next.t("setting:Never") : Setting.getFormattedDate(state.syncedTime)}
              {summary === "" ? null : (
                <Badge variant="muted" className="font-normal">
                  {summary}
                </Badge>
              )}
            </span>
          </div>
        </div>

        {tested === "" ? null : <MessageAlert variant="success" title={tested} />}

        {state.error === "" ? null : (
          <MessageAlert title={i18next.t("setting:The last sync failed")} description={state.error} />
        )}

        <div className="flex flex-wrap gap-2">
          <Button variant="outline" disabled={busy || kind === ""} onClick={() => save(state.mode)}>
            {i18next.t("general:Save")}
          </Button>
          <Button variant="outline" disabled={busy || kind === ""} onClick={test}>
            <PlugZap />
            {i18next.t("setting:Test connection")}
          </Button>
          <Button variant="outline" disabled={busy || state.kind === ""} loading={busy} onClick={() => run("both")}>
            <RefreshCw />
            {i18next.t("setting:Sync now")}
          </Button>
          <Button variant="ghost" disabled={busy || state.kind === ""} onClick={() => run("down")}>
            <CloudDownload />
            {i18next.t("setting:Fetch what is there")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
