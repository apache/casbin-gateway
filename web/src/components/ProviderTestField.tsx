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
import {CircleCheck, CircleX, Zap} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import {Field} from "@/components/shared/form-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import type {Provider, ProviderTestResult} from "@/types";

export interface ProviderTest {
  testing: boolean;
  result: ProviderTestResult | null;
  /** False while the form has no upstream to probe yet. */
  canTest: boolean;
  /** True once a failed probe has been shown, i.e. the next save goes through. */
  overridden: boolean;
  run: () => Promise<ProviderTestResult>;
  /** Probes the upstream, then saves if it answered. See the comment on guard. */
  guard: (save: () => void) => void;
}

/** What the probe found, in the one line a form has room for. */
export function testMessage(result: ProviderTestResult) {
  return result.statusCode ? `HTTP ${result.statusCode} - ${result.message}` : result.message;
}

/**
 * The connectivity probe behind both provider forms. It asks the first model
 * for one short answer, because that is the question a new provider raises:
 * not whether the endpoint is there, but whether it will serve the model that
 * requests are going to be routed to.
 */
export function useProviderTest(provider: Provider | null | undefined): ProviderTest {
  const [testing, setTesting] = React.useState(false);
  const [result, setResult] = React.useState<ProviderTestResult | null>(null);
  const [overridden, setOverridden] = React.useState(false);

  // The answer belongs to one upstream, one key and the model that was asked,
  // so it stops being an answer as soon as any of them changes.
  const identity = provider
    ? `${provider.type}|${provider.baseUrl}|${provider.apiKey}|${provider.authMode}|${(provider.models ?? []).join(",")}`
    : "";
  React.useEffect(() => {
    setResult(null);
    setOverridden(false);
  }, [identity]);

  const run = () => {
    if (!provider) {
      const failed = {success: false, message: i18next.t("provider:Nothing to test")};
      setResult(failed);
      return Promise.resolve(failed);
    }

    setTesting(true);
    setResult(null);
    // A rejected request is a failed probe, not a broken page: "no such host"
    // is exactly what the person filling in the form needs to read.
    return ProviderBackend.testProvider(provider)
      .then(res => (res.status === "ok" && res.data ? res.data : {success: false, message: res.msg}))
      .catch(error => ({success: false, message: String(error)}))
      .then(data => {
        setTesting(false);
        setResult(data);
        return data;
      });
  };

  // A failing upstream is a warning and not a verdict: a key that is not live
  // yet, or a vendor that is briefly down, is still worth saving. So the first
  // attempt stops to show what went wrong and the second one saves anyway.
  const guard = (save: () => void) => {
    if (!provider || provider.baseUrl === "" || overridden) {
      save();
      return;
    }

    run().then(data => {
      if (data.success) {
        save();
        return;
      }
      setOverridden(true);
    });
  };

  return {
    testing: testing,
    result: result,
    canTest: (provider?.baseUrl ?? "") !== "",
    overridden: overridden,
    run: run,
    guard: guard,
  };
}

/** What the probe answered, as a badge and the upstream's own words under it. */
export function ProviderTestOutcome({test}: {test: ProviderTest}) {
  if (!test.result) {
    return null;
  }

  return (
    <div className="grid gap-1">
      <Badge variant={test.result.success ? "success" : "destructive"} className="w-fit">
        {test.result.success ? <CircleCheck /> : <CircleX />}
        {test.result.success
          ? i18next.t("provider:Connection Successful")
          : i18next.t("provider:Connection Failed")}
      </Badge>
      <p className={test.result.success ? "text-muted-foreground text-xs" : "text-destructive text-xs"}>
        {testMessage(test.result)}
      </p>
    </div>
  );
}

/** The "does this key work" row of a provider form. */
export function ProviderTestField({test, submitText}: {test: ProviderTest; submitText: string}) {
  return (
    <Field
      label={i18next.t("provider:Connectivity")}
      hint={
        test.overridden
          ? i18next.t("provider:Save anyway hint").replace("{action}", submitText)
          : i18next.t("provider:Connectivity hint")
      }
    >
      <div className="flex flex-wrap items-center gap-3">
        <Button
          type="button"
          size="sm"
          variant="outline"
          loading={test.testing}
          disabled={!test.canTest}
          onClick={() => test.run()}
        >
          <Zap />
          {i18next.t("provider:Test Connectivity")}
        </Button>
        <ProviderTestOutcome test={test} />
      </div>
    </Field>
  );
}
