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

import * as AgentBackend from "@/backend/AgentBackend";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {providerIdOf, usesClientAuth, usesSubscription} from "@/lib/providers";
import type {Provider, ProviderStreamResult} from "@/types";

const protocolNames: Record<string, string> = {
  anthropic: "Anthropic Messages",
  openai: "OpenAI Chat Completions",
  responses: "OpenAI Responses",
};

export function ProviderStreamCheck({agentId, provider, disabled}: {agentId: string; provider: Provider; disabled: boolean}) {
  const [checking, setChecking] = React.useState(false);
  const [result, setResult] = React.useState<ProviderStreamResult | null>(null);
  const canCheck = !usesClientAuth(provider) && !usesSubscription(provider) &&
    Boolean(provider.apiKey?.trim()) && (provider.models ?? []).some(model => model.trim() !== "");

  const run = async () => {
    setChecking(true);
    setResult(null);
    try {
      const response = await AgentBackend.checkAgentProviderStream(agentId, providerIdOf(provider));
      setResult(response.status === "ok" && response.data ? response.data : {
        success: false, model: "", protocol: "", message: response.msg,
      });
    } catch (error) {
      setResult({success: false, model: "", protocol: "", message: String(error)});
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="space-y-2 rounded-md border p-3">
      <Button type="button" variant="outline" size="sm" loading={checking} disabled={!canCheck || disabled} onClick={() => void run()}>
        {i18next.t("agent:Test provider stream")}
      </Button>
      <p className="text-xs text-muted-foreground">
        {i18next.t(canCheck ? "agent:Provider stream check hint" : "agent:Provider stream check requires key and model")}
      </p>
      <div aria-live="polite">
        {result ? (
          <div className="space-y-1 text-xs">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={result.success ? "success" : "destructive"}>
                {i18next.t(result.success ? "agent:Received streamed text" : "agent:Provider stream check failed")}
              </Badge>
              {result.protocol ? <span>{protocolNames[result.protocol] ?? result.protocol}</span> : null}
              {result.model ? <code>{result.model}</code> : null}
              {result.statusCode ? <span>HTTP {result.statusCode}</span> : null}
              {typeof result.firstTextMs === "number" ? (
                <span>{i18next.t("agent:First streamed text in {ms} ms").replace("{ms}", String(result.firstTextMs))}</span>
              ) : null}
            </div>
            {result.message ? <p className="break-words text-muted-foreground">{result.message}</p> : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}
