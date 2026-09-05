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
import {Check, Globe, PlugZap, X} from "lucide-react";
import i18next from "i18next";

import * as SettingBackend from "@/backend/SettingBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import type {ProxyCheck} from "@/types";

/**
 * The proxy every outbound request goes through, with a button that tries it.
 * The test uses the address as typed rather than as stored, so a proxy can be
 * checked before the page is saved.
 */
export function ProxyField({value, onChange}: {value: string; onChange: (value: string) => void}) {
  const [testing, setTesting] = React.useState(false);
  const [result, setResult] = React.useState<ProxyCheck | null>(null);

  const test = () => {
    setTesting(true);
    setResult(null);
    SettingBackend.testOutboundProxy(value)
      .then(res => {
        if (res.status === "ok" && res.data) {
          setResult(res.data);
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(failure => Setting.showMessage("error", failure.message || String(failure)))
      .then(() => setTesting(false));
  };

  return (
    <Field className="md:col-span-2" label={i18next.t("setting:Outbound SOCKS5 proxy")} htmlFor="setting-httpProxy">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          id="setting-httpProxy"
          value={value}
          placeholder="127.0.0.1:1080"
          className="max-w-xs"
          onChange={event => onChange(event.target.value)}
        />
        <Button variant="outline" loading={testing} onClick={test}>
          <PlugZap />
          {i18next.t("setting:Test the proxy")}
        </Button>
      </div>
      <p className="text-muted-foreground text-xs">{i18next.t("setting:Outbound SOCKS5 proxy hint")}</p>
      {result === null ? null : <ProxyResult result={result} />}
    </Field>
  );
}

function ProxyResult({result}: {result: ProxyCheck}) {
  if (!result.dialed) {
    return (
      <MessageAlert
        variant="destructive"
        title={i18next.t("setting:{proxy} did not accept a connection").replace("{proxy}", result.address)}
        description={result.dialError}
      />
    );
  }

  const targets = result.targets ?? [];
  const restricted = targets.find(target => target.restricted);
  const exit = [result.exitIp, result.exitCountry].filter(part => part).join(" · ");

  return (
    <div className="flex flex-col gap-2 rounded-lg border bg-card px-4 py-3 text-sm">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="success">
          <Check />
          {i18next.t("setting:Answered in {millis} ms").replace("{millis}", String(result.dialMillis))}
        </Badge>
        {restricted === undefined ? null : (
          <Badge variant={restricted.ok ? "success" : "danger"}>
            {restricted.ok ? <Check /> : <X />}
            {restricted.ok
              ? i18next.t("setting:Blocked sites are reachable")
              : i18next.t("setting:Blocked sites are not reachable")}
          </Badge>
        )}
        {exit === "" ? null : (
          <Badge variant="info">
            <Globe />
            {i18next.t("setting:Traffic leaves from {exit}").replace("{exit}", exit)}
          </Badge>
        )}
      </div>

      <div className="grid gap-1">
        {targets.map(target => (
          <div key={target.url} className="flex flex-wrap items-baseline gap-x-2 text-xs">
            {target.ok ? <Check className="size-3.5 text-success" /> : <X className="size-3.5 text-destructive" />}
            <span className="font-medium">{target.name}</span>
            <span className="text-muted-foreground break-all">{target.url}</span>
            <span className="text-muted-foreground ml-auto shrink-0">
              {target.ok ? `${target.status} · ${target.millis} ms` : target.error}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
