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
import {CircleCheck, CircleX, ExternalLink, RefreshCw, Save, Zap} from "lucide-react";
import i18next from "i18next";

import * as ChannelBackend from "@/backend/ChannelBackend";
import * as Setting from "@/Setting";
import {FormRow, PageHeader} from "@/components/FormRow";
import {Result} from "@/components/Result";
import {Alert, AlertDescription, AlertTitle} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Combobox} from "@/components/ui/combobox";
import {Input} from "@/components/ui/input";
import {NumberInput} from "@/components/ui/number-input";
import {PasswordInput} from "@/components/ui/password-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {PageSpinner, Spinner} from "@/components/ui/spinner";
import {TagsInput} from "@/components/ui/tags-input";
import {anthropicChannelPresets, anthropicPreset} from "@/lib/anthropicChannelPresets";
import type {Account, Channel, ChannelTestResult} from "@/types";

// Hard-coded presets for milestone 1.1 (per channel type).
const BASE_URL_PRESETS: Record<string, string[]> = {
  openai: ["https://api.openai.com/v1"],
  custom: ["https://oneapi.example.com", "https://api.deepseek.com/v1", "https://api.moonshot.cn/v1"],
};

const MODEL_PRESETS: Record<string, string[]> = {
  openai: ["gpt-5.5", "gpt-5", "gpt-5-mini", "o3", "o4-mini"],
  custom: ["deepseek-chat", "deepseek-reasoner", "moonshot-v1-8k", "qwen-max"],
};

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
  if (!path.endsWith("/v1")) {
    path += "/v1";
  }

  url.pathname = path + endpoint;
  return url.toString();
}

function emptyChannel(owner: string): Channel {
  const randomName = Setting.getRandomName();
  return {
    owner,
    name: `channel_${randomName}`,
    displayName: `New Channel - ${randomName}`,
    type: "openai",
    provider: "",
    authType: "bearer",
    baseUrl: "",
    apiKey: "",
    models: [],
    defaultModel: "",
    haikuModel: "",
    sonnetModel: "",
    opusModel: "",
    priority: 0,
    status: "enabled",
  };
}

export default function ChannelEditPage({account}: {account: Account}) {
  const {owner = "", channelName = ""} = useParams();
  const navigate = useNavigate();
  // undefined while loading, null when the channel could not be loaded.
  const [channel, setChannel] = React.useState<Channel | null | undefined>(undefined);
  const [testing, setTesting] = React.useState(false);
  const [fetchingModels, setFetchingModels] = React.useState(false);
  const [fetchedModels, setFetchedModels] = React.useState<string[]>([]);
  const [result, setResult] = React.useState<ChannelTestResult | null>(null);
  const creating = !owner || !channelName;

  React.useEffect(() => {
    if (creating) {
      setChannel(emptyChannel(account.name));
      return;
    }
    ChannelBackend.getChannel(owner, channelName)
      .then(res => {
        if (res.status === "ok") {
          setChannel(res.data);
        } else {
          setChannel(null);
          Setting.showMessage("error", `${i18next.t("channel:Failed to get channel")}: ${res.msg}`);
        }
      })
      .catch(error => {
        setChannel(null);
        Setting.showMessage("error", `${i18next.t("channel:Failed to get channel")}: ${error}`);
      });
  }, [account.name, channelName, creating, owner]);

  const setField = <K extends keyof Channel>(key: K, value: Channel[K]) => {
    setChannel(current => (current ? {...current, [key]: value} : current));
  };

  const save = () => {
    if (!channel) {
      return Promise.resolve(false);
    }

    const request = creating
      ? ChannelBackend.addChannel(channel)
      : ChannelBackend.updateChannel(owner, channelName, channel);
    return request
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to save")}: ${res.msg}`);
          return false;
        }
        Setting.showMessage("success", i18next.t("channel:Channel saved"));
        if (creating) {
          navigate(`/channels/${channel.owner}/${channel.name}`, {replace: true});
        }
        return true;
      })
      .catch(error => {
        Setting.showMessage("error", `${i18next.t("channel:Failed to save")}: ${error}`);
        return false;
      });
  };

  const selectType = (type: string) => {
    if (type !== "anthropic") {
      setField("type", type);
      return;
    }
    const preset = anthropicPreset("anthropic")!;
    setChannel(current => current ? {
      ...current,
      type,
      provider: preset.id,
      authType: preset.authType,
      baseUrl: preset.baseUrl,
      defaultModel: preset.defaultModel,
      haikuModel: preset.haikuModel,
      sonnetModel: preset.sonnetModel,
      opusModel: preset.opusModel,
    } : current);
  };

  const selectProvider = (provider: string) => {
    const preset = anthropicPreset(provider);
    if (!preset) return;
    setChannel(current => current ? {
      ...current,
      provider,
      displayName: current.displayName.startsWith("New Channel") ? preset.name : current.displayName,
      authType: preset.authType,
      baseUrl: preset.baseUrl,
      defaultModel: preset.defaultModel,
      haikuModel: preset.haikuModel,
      sonnetModel: preset.sonnetModel,
      opusModel: preset.opusModel,
    } : current);
  };

  const fetchModels = () => {
    if (!channel) return;
    setFetchingModels(true);
    ChannelBackend.fetchChannelModels(channel)
      .then(res => {
        if (res.status === "ok") {
          setFetchedModels(res.data ?? []);
        } else {
          Setting.showMessage("error", `${i18next.t("channel:Failed to fetch models")}: ${res.msg}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("channel:Failed to fetch models")}: ${error}`))
      .then(() => setFetchingModels(false));
  };

  // The test probes the stored channel, so the edits have to be saved first.
  const test = () => {
    setTesting(true);
    setResult(null);
    save()
      .then(saved => (saved ? ChannelBackend.testChannel(owner, channelName) : null))
      .then(res => {
        setTesting(false);
        if (res === null) {
          return;
        }
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("channel:Failed to test")}: ${res.msg}`);
          return;
        }
        setResult(res.data);
      })
      .catch(error => {
        setTesting(false);
        Setting.showMessage("error", `${i18next.t("channel:Failed to test")}: ${error}`);
      });
  };

  if (channel === undefined) {
    return <PageSpinner />;
  }

  if (channel === null) {
    return (
      <Result
        status="404"
        title={i18next.t("channel:Channel not found")}
        extra={
          <Button onClick={() => navigate("/channels")}>{i18next.t("channel:Channels")}</Button>
        }
      />
    );
  }

  const upstreamUrl = buildOpenAiUrl(channel.baseUrl, channel.type === "anthropic" ? "/messages" : "/chat/completions");
  const preset = anthropicPreset(channel.provider);
  const isZhipuChannel = channel.type === "anthropic" && channel.provider === "zhipu";
  const pageTitle = creating
    ? i18next.t("channel:New Channel")
    : i18next.t("channel:Edit Channel");
  const anthropicModelFields = [
    {field: "defaultModel", label: i18next.t("channel:Default model")},
    {field: "haikuModel", label: i18next.t("channel:Haiku model")},
    {field: "sonnetModel", label: i18next.t("channel:Sonnet model")},
    {field: "opusModel", label: i18next.t("channel:Opus model")},
  ] as const;
  const modelOptions = Array.from(new Set([
    ...(preset ? [preset.defaultModel, preset.haikuModel, preset.sonnetModel, preset.opusModel] : []),
    ...fetchedModels,
  ].filter(Boolean))).map(value => ({value}));

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={`${pageTitle}: ${channel.displayName}`}>
        <Button variant="outline" onClick={test} disabled={testing || creating || isZhipuChannel}>
          {testing ? <Spinner /> : <Zap />}
          {i18next.t("channel:Test Connectivity")}
        </Button>
        <Button onClick={save}>
          <Save />
          {i18next.t("general:Save")}
        </Button>
      </PageHeader>

      <Card>
        <CardContent className="divide-y py-0">
          <FormRow label={i18next.t("general:Name")}>
            <Input value={channel.name} disabled={!creating} onChange={event => setField("name", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("general:Display name")}>
            <Input
              value={channel.displayName}
              onChange={event => setField("displayName", event.target.value)}
            />
          </FormRow>
          <FormRow label={i18next.t("channel:Type")}>
            <Select value={channel.type} onValueChange={selectType}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="openai">OpenAI</SelectItem>
                <SelectItem value="custom">Custom</SelectItem>
                <SelectItem value="anthropic">Anthropic</SelectItem>
              </SelectContent>
            </Select>
          </FormRow>
          {channel.type === "anthropic" ? (
            <FormRow label={i18next.t("channel:Provider preset")}>
              <Select value={channel.provider || "custom"} onValueChange={selectProvider}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {anthropicChannelPresets.map(item => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}
                </SelectContent>
              </Select>
              {preset?.website ? (
                <div className="mt-2 flex gap-3 text-sm">
                  <a className="text-primary hover:underline" href={preset.website} target="_blank" rel="noreferrer">{preset.name} <ExternalLink className="inline h-3 w-3" /></a>
                  {preset.apiKeyUrl ? <a className="text-primary hover:underline" href={preset.apiKeyUrl} target="_blank" rel="noreferrer">{i18next.t("channel:Get API Key")} <ExternalLink className="inline h-3 w-3" /></a> : null}
                </div>
              ) : null}
            </FormRow>
          ) : null}
          <FormRow
            label={i18next.t("channel:Base URL")}
            hint={
              upstreamUrl === "" ? undefined : (
                <>
                  {i18next.t("channel:Base URL hint")}:{" "}
                  <span className="font-mono">{upstreamUrl}</span>
                </>
              )
            }
          >
            <Combobox
              allowCustomValue
              value={channel.baseUrl}
              placeholder="https://api.openai.com/v1"
              options={(BASE_URL_PRESETS[channel.type] ?? []).map(url => ({value: url}))}
              onChange={value => setField("baseUrl", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("channel:API Key")} hint={i18next.t("channel:API Key hint")}>
            <PasswordInput
              placeholder="sk-..."
              value={channel.apiKey}
              onChange={event => setField("apiKey", event.target.value)}
            />
          </FormRow>
          {channel.type !== "anthropic" ? <FormRow label={i18next.t("channel:Models")}>
            <TagsInput
              value={channel.models}
              placeholder="gpt-5, gpt-5-mini"
              suggestions={MODEL_PRESETS[channel.type] ?? []}
              onChange={value => setField("models", value)}
            />
          </FormRow> : (
            <>
              <FormRow label={i18next.t("channel:Upstream format")}><Input readOnly value={i18next.t("channel:Anthropic Messages (native)")} /></FormRow>
              {anthropicModelFields.map(({field, label}) => (
                <FormRow key={field} label={label}>
                  <Combobox allowCustomValue value={channel[field]} options={modelOptions} onChange={value => setField(field, value)} />
                </FormRow>
              ))}
              <FormRow label={i18next.t("channel:Model actions")}>
                <div className="flex gap-2">
                  <Button type="button" variant="outline" onClick={fetchModels} disabled={fetchingModels}>{fetchingModels ? <Spinner /> : <RefreshCw />}{i18next.t("channel:Fetch models")}</Button>
                  <Button type="button" variant="outline" onClick={() => setChannel(current => current ? {...current, haikuModel: current.defaultModel, sonnetModel: current.defaultModel, opusModel: current.defaultModel} : current)}>{i18next.t("channel:Use Default for all")}</Button>
                </div>
              </FormRow>
              <FormRow label={i18next.t("channel:Authentication")}>
                <Select value={channel.authType} onValueChange={value => setField("authType", value as Channel["authType"])}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent><SelectItem value="bearer">Bearer Token</SelectItem><SelectItem value="x-api-key">x-api-key</SelectItem></SelectContent>
                </Select>
              </FormRow>
            </>
          )}
          <FormRow label={i18next.t("channel:Priority")} hint={i18next.t("channel:Priority hint")}>
            <NumberInput
              min={0}
              className="max-w-xs"
              value={channel.priority}
              onChange={value => setField("priority", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("channel:Status")}>
            <Select value={channel.status} onValueChange={value => setField("status", value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="enabled">
                  <span className="flex items-center gap-2">
                    <CircleCheck className="h-4 w-4 text-success" />
                    {i18next.t("channel:Enabled")}
                  </span>
                </SelectItem>
                <SelectItem value="disabled">
                  <span className="flex items-center gap-2">
                    <CircleX className="h-4 w-4 text-destructive" />
                    {i18next.t("channel:Disabled")}
                  </span>
                </SelectItem>
              </SelectContent>
            </Select>
          </FormRow>
        </CardContent>
      </Card>

      {result ? (
        <Alert className="mt-4" variant={result.success ? "success" : "destructive"}>
          {result.success ? <CircleCheck /> : <CircleX />}
          <AlertTitle>
            {result.success
              ? i18next.t("channel:Connection Successful")
              : i18next.t("channel:Connection Failed")}
          </AlertTitle>
          <AlertDescription>
            {result.statusCode ? `HTTP ${result.statusCode} - ${result.message}` : result.message}
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  );
}
