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

import * as MiscBackend from "@/backend/MiscBackend";
import * as Setting from "@/Setting";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Switch} from "@/components/ui/switch";
import type {AutostartState} from "@/types";

/**
 * Whether Gateway comes up when this account logs in. The switch writes the
 * same login entry as the tray icon's "Start at Login", and applies as it is
 * switched rather than on Save: this one is the machine's state, not a row in
 * the database the rest of the page edits.
 */
export function StartupPanel() {
  const [state, setState] = React.useState<AutostartState | null>(null);
  const [busy, setBusy] = React.useState(false);

  React.useEffect(() => {
    MiscBackend.getAutostart()
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

  const toggle = (enabled: boolean) => {
    setBusy(true);
    MiscBackend.updateAutostart(enabled)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setState(res.data);
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setBusy(false));
  };

  return (
    <Card className="gap-4 py-5">
      <CardHeader className="px-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <CardTitle className="text-[15px]">{i18next.t("setting:Startup")}</CardTitle>
            <CardDescription>{i18next.t("setting:Startup description")}</CardDescription>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-muted-foreground text-xs">
              {i18next.t(state.enabled ? "setting:Autostart is on" : "setting:Autostart is off")}
            </span>
            <Switch
              checked={state.enabled}
              disabled={busy || (!state.supported && !state.enabled)}
              aria-label={i18next.t("setting:Start at login")}
              onCheckedChange={toggle}
            />
          </div>
        </div>
      </CardHeader>

      {state.supported ? null : (
        <CardContent className="px-5">
          <p className="text-muted-foreground text-xs">{i18next.t("setting:No launcher to start")}</p>
        </CardContent>
      )}
    </Card>
  );
}
