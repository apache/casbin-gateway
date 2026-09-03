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
import {CircleX, RefreshCw, Wallet} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {NumberInput} from "@/components/shared/number-input";
import {PasswordInput} from "@/components/shared/password-input";
import {Section} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {formatPairs, parsePairs} from "@/lib/pairs";
import {emptyQuotaConfig, formatQuota, quotaPresets} from "@/lib/quota";
import {usesClientAuth} from "@/lib/providers";
import type {Provider, ProviderQuota, QuotaConfig} from "@/types";

/** The one line a table has room for: what is left, and why there is no number
 * when there is none. */
export function QuotaBadge({quota}: {quota: ProviderQuota | undefined}) {
  if (!quota) {
    return <span className="text-muted-foreground">{i18next.t("provider:Not asked yet")}</span>;
  }
  // The error comes first: a known vendor whose base URL will not parse is
  // unsupported and has something to say about why.
  if (quota.error !== "") {
    return (
      <SimpleTooltip title={quota.error}>
        <Badge variant="destructive">
          <CircleX />
          {i18next.t("provider:Balance unavailable")}
        </Badge>
      </SimpleTooltip>
    );
  }
  if (!quota.supported) {
    return <span className="text-muted-foreground">{i18next.t("provider:No balance endpoint")}</span>;
  }

  const amount = quota.remaining ?? quota.total ?? quota.used;
  if (amount === null || amount === undefined) {
    return <span className="text-muted-foreground">-</span>;
  }

  const detail = [
    quota.used === null ? "" : `${i18next.t("provider:Used")} ${formatQuota(quota.used, quota.unit)}`,
    quota.total === null ? "" : `${i18next.t("provider:Granted")} ${formatQuota(quota.total, quota.unit)}`,
    `${i18next.t("provider:Asked at")} ${quota.time}`,
  ]
    .filter(part => part !== "")
    .join(" · ");

  return (
    <SimpleTooltip title={detail}>
      <Badge variant={quota.remaining !== null && quota.remaining <= 0 ? "warning" : "success"}>
        {formatQuota(amount, quota.unit)}
      </Badge>
    </SimpleTooltip>
  );
}

/** Reads a scale that people would rather write as a whole number than as the
 * fraction it divides by. 0 and 1 both mean "as the vendor reports it". */
function scaleOf(config: QuotaConfig) {
  return config.scale === 0 ? 1 : config.scale;
}

/**
 * What the vendor says is left, on the page where the provider is edited: the
 * reading itself, and — for the vendors Gateway has no built-in endpoint for —
 * where to ask and which fields of the answer to read.
 */
export function ProviderQuotaSection({
  provider,
  onChange,
}: {
  provider: Provider;
  onChange: (quota: QuotaConfig | null) => void;
}) {
  const [quota, setQuota] = React.useState<ProviderQuota | null>(null);
  const [loading, setLoading] = React.useState(false);
  const id = `${provider.owner}/${provider.name}`;
  const config = provider.quota ?? null;

  const load = React.useCallback(
    (force: boolean) => {
      setLoading(true);
      ProviderBackend.refreshProviderQuotas(id, force)
        .then(res => {
          setLoading(false);
          if (res.status === "error") {
            Setting.showMessage("error", `${i18next.t("provider:Failed to read the balance")}: ${res.msg}`);
            return;
          }
          setQuota((res.data ?? [])[0] ?? null);
        })
        .catch(error => {
          setLoading(false);
          Setting.showMessage("error", `${i18next.t("provider:Failed to read the balance")}: ${error}`);
        });
    },
    [id],
  );

  React.useEffect(() => {
    if (!usesClientAuth(provider)) {
      load(false);
    }
    // The reading is asked for once, for the provider as it is stored. Asking
    // again on every keystroke would bill the vendor for typing.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id]);

  const setConfigField = <K extends keyof QuotaConfig>(key: K, value: QuotaConfig[K]) => {
    onChange({...(config ?? emptyQuotaConfig()), [key]: value});
  };

  const mode = config === null ? "auto" : config.manual ? "manual" : "custom";
  const setMode = (next: string) => {
    if (next === "auto") {
      onChange(null);
    } else {
      onChange({...(config ?? emptyQuotaConfig()), manual: next === "manual"});
    }
  };

  return (
    <Section
      columns={2}
      title={
        <span className="flex items-center gap-2">
          <Wallet className="size-4" />
          {i18next.t("provider:Balance")}
        </span>
      }
      description={i18next.t("provider:Balance description")}
    >
      <Field label={i18next.t("provider:Remaining")} className="md:col-span-2">
        <div className="flex flex-wrap items-center gap-3">
          <QuotaBadge quota={quota ?? undefined} />
          <Button type="button" size="sm" variant="outline" loading={loading} onClick={() => load(true)}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        </div>
      </Field>

      <Field
        label={i18next.t("provider:Balance source")}
        hint={i18next.t("provider:Balance source hint")}
        className="md:col-span-2"
      >
        <SimpleSelect
          className="max-w-xs"
          value={mode}
          onChange={setMode}
          options={[
            {label: i18next.t("provider:Automatic"), value: "auto"},
            {label: i18next.t("provider:Custom endpoint"), value: "custom"},
            {label: i18next.t("provider:Manual amount"), value: "manual"},
          ]}
        />
      </Field>

      {config !== null && config.manual ? (
        <>
          <Field label={i18next.t("provider:Initial amount")} hint={i18next.t("provider:Manual amount hint")}>
            <NumberInput min={0} value={config.initial} onChange={value => setConfigField("initial", value)} />
          </Field>
          <Field label={i18next.t("provider:Currency")} htmlFor="quota-unit" hint={i18next.t("provider:Manual currency hint")}>
            <Input
              id="quota-unit"
              placeholder="USD"
              value={config.unit}
              onChange={event => setConfigField("unit", event.target.value)}
            />
          </Field>
          <Field
            label={i18next.t("provider:Counting since")}
            hint={i18next.t("provider:Reset counter hint")}
            className="md:col-span-2"
          >
            <div className="flex flex-wrap items-center gap-3">
              <span className="text-muted-foreground text-sm">
                {config.since || i18next.t("provider:Since the start")}
              </span>
              <Button type="button" size="sm" variant="outline" onClick={() => setConfigField("since", "")}>
                {i18next.t("provider:Reset counter")}
              </Button>
            </div>
          </Field>
        </>
      ) : null}

      {config !== null && !config.manual ? (
        <>
          <Field label={i18next.t("provider:Preset")} hint={i18next.t("provider:Preset hint")}>
            <SimpleSelect
              value=""
              placeholder={i18next.t("provider:Fill in from")}
              onChange={value => {
                const preset = quotaPresets.find(item => item.key === value);
                if (preset) {
                  onChange({...preset.config, headers: {...preset.config.headers}});
                }
              }}
              options={quotaPresets.map(preset => ({label: preset.label, value: preset.key}))}
            />
          </Field>
          <Field label={i18next.t("provider:Balance URL")} htmlFor="quota-url" hint={i18next.t("provider:Balance URL hint")}>
            <Input
              id="quota-url"
              placeholder="/api/user/self"
              value={config.url}
              onChange={event => setConfigField("url", event.target.value)}
            />
          </Field>
          <Field label={i18next.t("provider:Balance token")} htmlFor="quota-token" hint={i18next.t("provider:Balance token hint")}>
            <PasswordInput
              id="quota-token"
              value={config.token}
              onChange={event => setConfigField("token", event.target.value)}
            />
          </Field>
          <Field label={i18next.t("provider:Currency")} htmlFor="quota-unit" hint={i18next.t("provider:Currency hint")}>
            <Input
              id="quota-unit"
              placeholder="USD"
              value={config.unit}
              onChange={event => setConfigField("unit", event.target.value)}
            />
          </Field>
          <Field
            label={i18next.t("agentConfig:Headers")}
            htmlFor="quota-headers"
            hint={i18next.t("provider:Balance headers hint")}
            className="md:col-span-2"
          >
            <Textarea
              id="quota-headers"
              rows={2}
              placeholder="Authorization=Bearer {{token}}"
              value={formatPairs(config.headers)}
              onChange={event => setConfigField("headers", parsePairs(event.target.value))}
            />
          </Field>
          <Field label={i18next.t("provider:Remaining path")} htmlFor="quota-remaining" hint={i18next.t("provider:Path hint")}>
            <Input
              id="quota-remaining"
              placeholder="data.quota"
              value={config.remaining}
              onChange={event => setConfigField("remaining", event.target.value)}
            />
          </Field>
          <Field label={i18next.t("provider:Used path")} htmlFor="quota-used">
            <Input
              id="quota-used"
              placeholder="data.used_quota"
              value={config.used}
              onChange={event => setConfigField("used", event.target.value)}
            />
          </Field>
          <Field label={i18next.t("provider:Granted path")} htmlFor="quota-total">
            <Input
              id="quota-total"
              value={config.total}
              onChange={event => setConfigField("total", event.target.value)}
            />
          </Field>
          <Field label={i18next.t("provider:Scale")} hint={i18next.t("provider:Scale hint")}>
            <NumberInput min={1} value={scaleOf(config)} onChange={value => setConfigField("scale", value)} />
          </Field>
        </>
      ) : null}
    </Section>
  );
}
