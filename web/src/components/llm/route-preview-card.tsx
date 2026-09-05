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
import {Play} from "lucide-react";
import i18next from "i18next";

import * as ModelRouteBackend from "@/backend/ModelRouteBackend";
import {SearchSelect, type SelectOption} from "@/components/shared/simple-select";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import type {Agent, RoutePreviewStep} from "@/types";

/**
 * Asks the gateway where a model name would actually go, without sending a
 * request. The first line is what answers it today; the lines below are the
 * downgrade, which is the part nobody sees until an upstream is already failing.
 */
export function RoutePreviewCard({agents}: {agents: Agent[]}) {
  const [model, setModel] = React.useState("");
  const [agent, setAgent] = React.useState("");
  const [steps, setSteps] = React.useState<RoutePreviewStep[] | null>(null);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(false);

  const agentOptions: SelectOption[] = Array.from(new Set(agents.map(entry => entry.agentId || entry.name)))
    .filter(id => id !== "")
    .map(id => ({value: id, label: id}));

  const run = () => {
    if (model.trim() === "") {
      return;
    }
    setLoading(true);
    ModelRouteBackend.previewModelRoute(model.trim(), agent)
      .then(res => {
        if (res.status === "ok") {
          setSteps(res.data ?? []);
          setError("");
        } else {
          setSteps(null);
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => {
        setSteps(null);
        setError(failure.message || String(failure));
      })
      .then(() => setLoading(false));
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{i18next.t("llm:Where would this go?")}</CardTitle>
        <CardDescription>{i18next.t("llm:Route preview description")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <form
          className="grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end"
          onSubmit={event => {
            event.preventDefault();
            run();
          }}
        >
          <div className="grid gap-2">
            <Label htmlFor="preview-model">{i18next.t("agent:Model")}</Label>
            <Input
              id="preview-model"
              className="font-mono"
              placeholder="claude-haiku-4-5"
              value={model}
              onChange={event => setModel(event.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="preview-agent">{i18next.t("agent:Agent")}</Label>
            <SearchSelect
              id="preview-agent"
              allowClear
              allowCustomValue
              value={agent}
              onChange={setAgent}
              options={agentOptions}
              placeholder={i18next.t("llm:No agent, by model name")}
            />
          </div>
          <Button type="submit" loading={loading} disabled={model.trim() === ""}>
            <Play />
            {i18next.t("llm:Trace it")}
          </Button>
        </form>

        {error ? <p className="text-destructive text-sm">{error}</p> : null}

        {steps !== null && error === "" ? (
          steps.length === 0 ? (
            <p className="text-muted-foreground text-sm">{i18next.t("llm:Nothing would answer this")}</p>
          ) : (
            <ol className="flex flex-col gap-1.5">
              {steps.map((step, index) => (
                <li
                  key={`${step.provider}|${step.model}`}
                  className="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2 text-sm"
                >
                  <span className="text-muted-foreground w-6 shrink-0 tabular-nums">{index + 1}</span>
                  <span className="font-mono text-xs">{step.model}</span>
                  <span className="text-muted-foreground text-xs">@</span>
                  <span className="truncate">{step.displayName || step.provider}</span>
                  <span className="ml-auto flex shrink-0 items-center gap-1">
                    {step.route ? <Badge variant="info">{step.route}</Badge> : null}
                    {index === 0 ? (
                      <Badge variant="success">{i18next.t("llm:Answers it now")}</Badge>
                    ) : (
                      <Badge variant="muted">{i18next.t("llm:Fallback")}</Badge>
                    )}
                    {step.suspended ? (
                      <Badge variant="warning">{i18next.t("llm:In cooldown")}</Badge>
                    ) : null}
                  </span>
                </li>
              ))}
            </ol>
          )
        ) : null}
      </CardContent>
    </Card>
  );
}
