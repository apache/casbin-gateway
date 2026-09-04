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

import {TriangleAlert} from "lucide-react";
import i18next from "i18next";

import {Alert, AlertDescription, AlertTitle} from "@/components/ui/alert";
import {CodeText} from "@/components/shared/misc";
import type {EnvConflict} from "@/types";

/** The wording for each kind of place agentenv.Conflict reports. */
const sourceLabels: Record<string, string> = {
  process: "agent:Set in the environment Gateway runs in",
  shell: "agent:Set in a shell startup file",
  profile: "agent:Set in a PowerShell profile",
  user: "agent:Set in the user environment variables",
  machine: "agent:Set in the system environment variables",
  system: "agent:Set in the system environment file",
};

/**
 * The warning for a variable the shell already exports. Writing the agent's
 * configuration file succeeds either way, and the agent still reads the
 * environment first, so without this the switch looks applied and changes
 * nothing.
 */
export function EnvWarningBanner({
  conflicts,
  applied,
  className,
}: {
  conflicts?: EnvConflict[];
  applied: boolean;
  className?: string;
}) {
  const items = conflicts ?? [];
  if (items.length === 0) {
    return null;
  }

  return (
    <Alert variant="warning" className={className}>
      <TriangleAlert />
      <AlertTitle>{i18next.t("agent:Environment overrides this configuration")}</AlertTitle>
      <AlertDescription>
        <p>{i18next.t(applied ? "agent:Env conflict hint" : "agent:Env conflict pending hint")}</p>
        <div className="w-full space-y-2 pt-1">
          {items.map(conflict => (
            <div key={conflict.key} className="w-full space-y-1">
              <CodeText copyable>{`${conflict.key}=${conflict.value}`}</CodeText>
              <p className="text-xs opacity-80">
                {i18next.t(sourceLabels[conflict.source] ?? "agent:Set in the environment")}
                {conflict.path ? <code className="ml-1 break-all">{conflict.path}</code> : null}
              </p>
              {conflict.fix ? (
                <p className="flex flex-wrap items-center gap-1 text-xs opacity-80">
                  <span>{i18next.t("agent:Clear it with")}</span>
                  <CodeText copyable>{conflict.fix}</CodeText>
                </p>
              ) : (
                <p className="text-xs opacity-80">{i18next.t("agent:Remove the line to clear it")}</p>
              )}
            </div>
          ))}
        </div>
      </AlertDescription>
    </Alert>
  );
}
