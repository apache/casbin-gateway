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
import {useNavigate, useParams} from "react-router-dom";
import {CircleCheck, CircleX, Save} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {ProviderIcon, ProviderIconField} from "@/components/ProviderIcon";
import {ProviderLoginField} from "@/components/ProviderLoginField";
import {ProviderModelsField} from "@/components/ProviderModelsField";
import {ProviderQuotaSection} from "@/components/ProviderQuota";
import {ProviderTestField, useProviderTest} from "@/components/ProviderTestField";
import {EnvSnippet} from "@/components/EnvSnippet";
import {Field} from "@/components/shared/form-dialog";
import {Loading} from "@/components/shared/loading";
import {CodeText, ResultScreen} from "@/components/shared/misc";
import {NumberInput} from "@/components/shared/number-input";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {baseUrlOptions, providerTypeOptions, wireApiOptions} from "@/components/shared/brand-options";
import {SearchSelect, SimpleSelect} from "@/components/shared/simple-select";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {
  authModeOptions,
  baseUrlPlaceholder,
  providerProtocol,
  providerWireApi,
  gatewayBaseUrl,
  localShell,
  usesClientAuth,
  usesSubscription,
  wireApiEndpoint,
} from "@/lib/providers";
import type {Provider, QuotaConfig} from "@/types";

// Mirrors object.namesApiVersion on the server.
function namesApiVersion(path: string) {
  return path.split("/").some(segment => /^v[0-9]/.test(segment));
}

// Mirrors object.BuildOpenAiUrl on the server.
function buildOpenAiUrl(baseUrl: string, endpoint: string) {
  let url: URL;
  try {
    url = new URL(baseUrl);
  } catch {
    return "";
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return "";
  }

  let path = url.pathname.replace(/\/+$/, "");
  if (path.endsWith(endpoint)) {
    path = path.slice(0, path.length - endpoint.length);
  }
  if (!namesApiVersion(path)) {
    path += "/v1";
  }

  url.pathname = path + endpoint;
  return url.toString();
}

// Mirrors object.BuildAnthropicUrl: the base URL is bare and the endpoint
// carries the /v1 prefix, the opposite of the OpenAI convention.
function buildAnthropicUrl(baseUrl: string, endpoint: string) {
  let url: URL;
  try {
    url = new URL(baseUrl);
  } catch {
    return "";
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return "";
  }

  let path = url.pathname.replace(/\/+$/, "");
  if (path.endsWith(endpoint)) {
    path = path.slice(0, path.length - endpoint.length);
  }
  path = path.replace(/\/+$/, "").replace(/\/v1$/, "");

  url.pathname = path + endpoint;
  return url.toString();
}

/** The upstream URL requests to this provider end up at. */
function buildUpstreamUrl(provider: Provider) {
  const wireApi = providerWireApi(provider);
  const endpoint = wireApiEndpoint(wireApi);
  return wireApi === "anthropic"
    ? buildAnthropicUrl(provider.baseUrl, endpoint)
    : buildOpenAiUrl(provider.baseUrl, endpoint);
}

export default function ProviderEditPage() {
  const {owner = "", providerName = ""} = useParams();
  const navigate = useNavigate();
  // undefined while loading, null when the provider could not be loaded.
  const [provider, setProvider] = React.useState<Provider | null | undefined>(undefined);
  const test = useProviderTest(provider);

  React.useEffect(() => {
    ProviderBackend.getProvider(owner, providerName)
      .then(res => {
        if (res.status === "ok") {
          setProvider(res.data);
        } else {
          setProvider(null);
          Setting.showMessage("error", `${i18next.t("provider:Failed to get provider")}: ${res.msg}`);
        }
      })
      .catch(error => {
        setProvider(null);
        Setting.showMessage("error", `${i18next.t("provider:Failed to get provider")}: ${error}`);
      });
  }, [providerName, owner]);

  const setField = <K extends keyof Provider>(key: K, value: Provider[K]) => {
    setProvider(current => (current ? {...current, [key]: value} : current));
  };

  const save = () => {
    if (!provider) {
      return;
    }

    ProviderBackend.updateProvider(owner, providerName, provider)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("provider:Failed to save")}: ${res.msg}`);
          return;
        }
        Setting.showMessage("success", i18next.t("provider:Provider saved"));
        navigate("/providers");
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("provider:Failed to save")}: ${error}`);
      });
  };

  if (provider === undefined) {
    return <Loading type="page" />;
  }

  if (provider === null) {
    return (
      <ResultScreen
        status="404"
        title={i18next.t("provider:Provider not found")}
        extra={<Button onClick={() => navigate("/providers")}>{i18next.t("provider:Providers")}</Button>}
      />
    );
  }

  const upstreamUrl = buildUpstreamUrl(provider);

  return (
    <PageContainer>
      <PageHeader
        title={
          <span className="flex items-center gap-2">
            <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} alt={provider.name} size={22} />
            {i18next.t("provider:Edit Provider")}
          </span>
        }
        description={`${provider.owner} / ${provider.name}`}
        actions={
          <Button onClick={() => test.guard(save)} loading={test.testing}>
            <Save />
            {i18next.t("general:Save")}
          </Button>
        }
      />

      <Section title={i18next.t("provider:Provider")}>
        <Field label={i18next.t("general:Display name")} htmlFor="provider-display-name">
          <Input
            id="provider-display-name"
            value={provider.displayName}
            onChange={event => setField("displayName", event.target.value)}
          />
        </Field>
        <Field label={i18next.t("provider:Type")}>
          <SimpleSelect
            value={provider.type}
            onChange={value => setField("type", value)}
            options={providerTypeOptions}
          />
        </Field>
        <Field
          label={i18next.t("provider:Base URL")}
          hint={
            upstreamUrl === "" ? undefined : (
              <>
                {i18next.t("provider:Base URL hint")}: <CodeText>{upstreamUrl}</CodeText>
              </>
            )
          }
        >
          <SearchSelect
            allowCustomValue
            value={provider.baseUrl}
            placeholder={baseUrlPlaceholder(provider.type)}
            options={baseUrlOptions(provider.type)}
            onChange={value => setField("baseUrl", value)}
          />
        </Field>
        <Field label={i18next.t("provider:Upstream API")} hint={i18next.t("provider:Upstream API hint")}>
          <SimpleSelect
            value={provider.protocol ?? ""}
            onChange={value => setField("protocol", value)}
            options={wireApiOptions(i18next.t("provider:From the type"))}
          />
        </Field>
        <Field
          label={i18next.t("provider:Authentication")}
          hint={usesClientAuth(provider) ? i18next.t("provider:Client auth hint") : undefined}
        >
          <SimpleSelect
            value={provider.authMode}
            // The stored key is meaningless once the caller's own is forwarded,
            // and the server drops it on save anyway.
            onChange={value =>
              setProvider(current => (current ? {...current, authMode: value, apiKey: ""} : current))
            }
            options={authModeOptions(provider, key => i18next.t(key))}
          />
        </Field>
        {usesSubscription(provider) ? (
          <ProviderLoginField
            provider={provider}
            vendor={provider.subscriptionVendor ?? ""}
            onSignedIn={session =>
              setProvider(current =>
                current
                  ? {
                    ...current,
                    loginId: session.id,
                    subscriptionAccount: session.account ?? "",
                    subscriptionPlan: session.plan ?? "",
                  }
                  : current,
              )
            }
          />
        ) : usesClientAuth(provider) ? null : (
          <Field
            label={i18next.t("provider:API Key")}
            htmlFor="provider-api-key"
            hint={i18next.t("provider:API Key hint")}
          >
            <PasswordInput
              id="provider-api-key"
              placeholder="sk-..."
              value={provider.apiKey}
              onChange={event => setField("apiKey", event.target.value)}
            />
          </Field>
        )}
        <ProviderModelsField
          provider={provider}
          hint={
            usesClientAuth(provider) || usesSubscription(provider)
              ? i18next.t("provider:Any model hint")
              : undefined
          }
          onChange={value => setField("models", value)}
        />
        <Field label={i18next.t("provider:Priority")} hint={i18next.t("provider:Priority hint")}>
          <NumberInput min={0} value={provider.priority} onChange={value => setField("priority", value)} />
        </Field>
        <Field label={i18next.t("provider:Status")}>
          <SimpleSelect
            value={provider.status}
            onChange={value => setField("status", value)}
            options={[
              {
                label: (
                  <span className="flex items-center gap-2">
                    <CircleCheck className="text-success size-4" />
                    {i18next.t("provider:Enabled")}
                  </span>
                ),
                value: "enabled",
              },
              {
                label: (
                  <span className="flex items-center gap-2">
                    <CircleX className="text-destructive size-4" />
                    {i18next.t("provider:Disabled")}
                  </span>
                ),
                value: "disabled",
              },
            ]}
          />
        </Field>
        <ProviderIconField provider={provider} onChange={value => setField("icon", value)} />
        <ProviderTestField test={test} submitText={i18next.t("general:Save")} />
        <Field
          label={i18next.t("provider:Notes")}
          htmlFor="provider-notes"
          hint={i18next.t("provider:Notes hint")}
          className="md:col-span-2 lg:col-span-3"
        >
          <Textarea
            id="provider-notes"
            rows={3}
            value={provider.notes ?? ""}
            onChange={event => setField("notes", event.target.value)}
          />
        </Field>
      </Section>

      <ProviderQuotaSection
        provider={provider}
        onChange={(quota: QuotaConfig | null) => setField("quota", quota)}
      />

      <Section title={i18next.t("provider:Usage")} description={i18next.t("provider:Usage hint")} columns={1}>
        <EnvSnippet
          protocol={providerProtocol(provider.type)}
          baseUrl={gatewayBaseUrl(providerProtocol(provider.type))}
          defaultShell={localShell()}
          includeToken={!usesClientAuth(provider)}
        />
        {usesClientAuth(provider) ? (
          <p className="text-muted-foreground text-sm">{i18next.t("provider:Client auth usage hint")}</p>
        ) : null}
        <p className="text-muted-foreground text-sm">{i18next.t("provider:Model routing hint")}</p>
      </Section>
    </PageContainer>
  );
}
