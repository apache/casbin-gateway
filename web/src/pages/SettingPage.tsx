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

import * as SettingBackend from "@/backend/SettingBackend";
import * as Setting from "@/Setting";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {UnauthorizedResult} from "@/components/shared/misc";
import {NumberInput} from "@/components/shared/number-input";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {SimpleSelect} from "@/components/shared/simple-select";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import type {Account, Setting as SettingType} from "@/types";

type KeysOfType<T> = {[K in keyof SettingType]: SettingType[K] extends T ? K : never}[keyof SettingType];
type StringKey = KeysOfType<string>;
type NumberKey = KeysOfType<number>;

export default function SettingPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [setting, setSetting] = React.useState<SettingType | null>(null);
  const [saving, setSaving] = React.useState(false);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }

    SettingBackend.getSetting().then(res => {
      if (res.status === "ok") {
        setSetting(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get data")}: ${res.msg}`);
      }
    });
  }, [isAdmin]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  if (setting === null) {
    return <Loading type="page" />;
  }

  const updateField = <K extends keyof SettingType>(key: K, value: SettingType[K]) => {
    setSetting(current => (current === null ? current : {...current, [key]: value}));
  };

  // The backend applies what it stores, so a port it cannot bind or a value a
  // subsystem refuses comes back as an error while the setting stays saved.
  const save = () => {
    setSaving(true);
    SettingBackend.updateSetting(Setting.deepCopy(setting))
      .then(res => {
        setSaving(false);
        if (res.status === "ok") {
          setSetting(res.data);
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(error => {
        setSaving(false);
        Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`);
      });
  };

  const textField = (key: StringKey, label: string, hint?: string) => (
    <Field label={label} htmlFor={`setting-${key}`} hint={hint}>
      <Input
        id={`setting-${key}`}
        value={setting[key]}
        onChange={event => updateField(key, event.target.value)}
      />
    </Field>
  );

  const secretField = (key: StringKey, label: string, hint?: string) => (
    <Field label={label} htmlFor={`setting-${key}`} hint={hint}>
      <PasswordInput
        id={`setting-${key}`}
        value={setting[key]}
        onChange={event => updateField(key, event.target.value)}
      />
    </Field>
  );

  const numberField = (key: NumberKey, label: string, hint?: string, min = 0, max?: number) => (
    <Field label={label} htmlFor={`setting-${key}`} hint={hint}>
      <NumberInput
        id={`setting-${key}`}
        value={setting[key]}
        onChange={value => updateField(key, value)}
        min={min}
        max={max}
      />
    </Field>
  );

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("setting:Settings")}
        description={i18next.t("setting:Page description")}
        actions={
          <Button onClick={save} loading={saving}>
            {i18next.t("general:Save")}
          </Button>
        }
      />

      <Section columns={2} title={i18next.t("setting:LLM records")} description={i18next.t("setting:LLM records description")}>
        <Field label={i18next.t("setting:Record mode")}>
          <SimpleSelect
            value={setting.llmRecordMode}
            onChange={value => updateField("llmRecordMode", value as SettingType["llmRecordMode"])}
            options={[
              {label: i18next.t("llm:Recording off"), value: "off"},
              {label: i18next.t("llm:Record metadata"), value: "metadata"},
              {label: i18next.t("llm:Record metadata and bodies"), value: "full"},
            ]}
          />
        </Field>
        {numberField("llmRecordRetentionDays", i18next.t("setting:Retention days"), undefined, 1)}
        {numberField("llmRecordMaxRecords", i18next.t("setting:Max records"), undefined, 1)}
        {numberField("llmRecordQueueCapacity", i18next.t("setting:Queue capacity"), i18next.t("setting:Queue capacity hint"), 1)}
        {numberField("llmRecordMaxPayloadBytes", i18next.t("setting:Max payload bytes"), undefined, 65536, 33554432)}
        {textField("llmPricingFile", i18next.t("setting:Pricing file"), i18next.t("setting:Pricing file hint"))}
      </Section>

      <Section
        columns={2}
        title={i18next.t("setting:Channel probes")}
        description={i18next.t("setting:Channel probes description")}
      >
        <Field label={i18next.t("setting:Probe mode")} hint={i18next.t("setting:Probe mode hint")}>
          <SimpleSelect
            value={setting.providerProbeMode}
            onChange={value => updateField("providerProbeMode", value as SettingType["providerProbeMode"])}
            options={[
              {label: i18next.t("audit:Probe mode auto"), value: "auto"},
              {label: i18next.t("audit:Probe mode manual"), value: "manual"},
              {label: i18next.t("audit:Probe mode off"), value: "off"},
            ]}
          />
        </Field>
      </Section>

      <Section columns={2} title={i18next.t("setting:Agents")}>
        {textField("agentPatchStateDir", i18next.t("setting:Agent state dir"), i18next.t("setting:Agent state dir hint"))}
        {numberField("agentRecordCapacity", i18next.t("setting:Agent record capacity"), i18next.t("setting:Agent record capacity hint"), 1)}
        {numberField("agentMonitorPollSeconds", i18next.t("setting:Agent poll seconds"), undefined, 1)}
      </Section>

      <Section columns={2} collapsible title={i18next.t("setting:Sign-in")} description={i18next.t("setting:Sign-in description")}>
        {textField("casdoorEndpoint", i18next.t("setting:Casdoor endpoint"))}
        {textField("clientId", i18next.t("setting:Client ID"))}
        {secretField("clientSecret", i18next.t("setting:Client secret"))}
        {textField("casdoorOrganization", i18next.t("setting:Casdoor organization"))}
        {textField("casdoorApplication", i18next.t("setting:Casdoor application"))}
      </Section>

      <Section columns={2} title={i18next.t("setting:Security")}>
        {secretField("apiKeyEncryptionKey", i18next.t("setting:API key encryption key"), i18next.t("setting:API key encryption key hint"))}
        {textField("relayToken", i18next.t("setting:Relay token"), i18next.t("setting:Relay token hint"))}
      </Section>

      <Section columns={2} collapsible title={i18next.t("setting:Network")}>
        {textField("httpProxy", i18next.t("setting:Outbound SOCKS5 proxy"), i18next.t("setting:Outbound SOCKS5 proxy hint"))}
      </Section>
    </PageContainer>
  );
}
