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
import {Link} from "react-router-dom";
import {CheckCircle2, ChevronRight, Circle, ListChecks, X} from "lucide-react";
import i18next from "i18next";

import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Progress} from "@/components/ui/progress";
import {cn} from "@/lib/utils";
import type {Agent, LlmAgentStat, Provider} from "@/types";

const dismissedStorageKey = "onboardingChecklistDismissed";

function readDismissed(): boolean {
  try {
    return localStorage.getItem(dismissedStorageKey) === "true";
  } catch {
    return false;
  }
}

function writeDismissed(dismissed: boolean) {
  try {
    if (dismissed) {
      localStorage.setItem(dismissedStorageKey, "true");
    } else {
      localStorage.removeItem(dismissedStorageKey);
    }
  } catch {
    // Private-mode storage failures must not take the page down.
  }
}

interface Step {
  done: boolean;
  title: string;
  description: string;
  to: string;
}

export interface Onboarding {
  steps: Step[];
  /** Whether the card is on screen, which is what the reopen entry hangs off. */
  open: boolean;
  dismiss: () => void;
  reopen: () => void;
}

/**
 * The three-step path from an empty machine to seeing traffic flow, taken from
 * the README's "Send an agent's traffic through Gateway" section. It hides
 * itself once every step is done and stays hidden once dismissed, so the state
 * is held here rather than inside the card: something has to offer it back.
 */
export function useOnboarding({
  providers,
  agents,
  stats,
}: {
  providers: Provider[];
  agents: Agent[];
  stats: LlmAgentStat[];
}): Onboarding {
  const [dismissed, setDismissed] = React.useState(readDismissed);
  // Set by the reopen entry, which shows the card again even where every step
  // is done: someone asking for the guide is asking to read it, not to be told
  // there is nothing left to do.
  const [reopened, setReopened] = React.useState(false);

  const steps: Step[] = [
    {
      done: providers.length > 0,
      title: i18next.t("agent:Onboarding add provider"),
      description: i18next.t("agent:Onboarding add provider detail"),
      to: "/providers",
    },
    {
      done: agents.some(agent => agent.provider !== ""),
      title: i18next.t("agent:Onboarding connect agent"),
      description: i18next.t("agent:Onboarding connect agent detail"),
      to: "/",
    },
    {
      done: stats.some(stat => stat.requests > 0),
      title: i18next.t("agent:Onboarding see it work"),
      description: i18next.t("agent:Onboarding see it work detail"),
      to: "/llm-records",
    },
  ];

  return {
    steps: steps,
    open: reopened || (!dismissed && !steps.every(step => step.done)),
    dismiss: () => {
      writeDismissed(true);
      setDismissed(true);
      setReopened(false);
    },
    reopen: () => {
      writeDismissed(false);
      setDismissed(false);
      setReopened(true);
    },
  };
}

/** The entry that brings the card back, for whoever closed it. */
export function OnboardingButton({onboarding}: {onboarding: Onboarding}) {
  return (
    <Button variant="ghost" onClick={onboarding.reopen}>
      <ListChecks />
      {i18next.t("agent:Getting started")}
    </Button>
  );
}

export function OnboardingChecklist({onboarding}: {onboarding: Onboarding}) {
  const steps = onboarding.steps;
  const doneCount = steps.filter(step => step.done).length;

  return (
    <Card className="gap-0 py-0 shadow-xs">
      <CardContent className="flex flex-col gap-3 p-4">
        <div className="flex items-center gap-3">
          <ListChecks className="text-muted-foreground size-4 shrink-0" />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-medium">{i18next.t("agent:Getting started")}</span>
              <span className="text-muted-foreground text-xs">
                {doneCount}/{steps.length}
              </span>
            </div>
            <Progress value={(doneCount / steps.length) * 100} className="h-1.5" />
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onboarding.dismiss}
            aria-label={i18next.t("general:Dismiss")}
          >
            <X />
          </Button>
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
          {steps.map(step => (
            <Link
              key={step.title}
              to={step.to}
              className={cn(
                "hover:border-foreground/20 flex items-start gap-2 rounded-lg border px-3 py-2 transition-colors",
                step.done && "opacity-60",
              )}
            >
              {step.done ? (
                <CheckCircle2 className="text-muted-foreground mt-0.5 size-4 shrink-0" />
              ) : (
                <Circle className="text-muted-foreground mt-0.5 size-4 shrink-0" />
              )}
              <div className="flex min-w-0 flex-col">
                <span className="text-xs font-medium">{step.title}</span>
                <span className="text-muted-foreground text-[11px]">{step.description}</span>
              </div>
              {!step.done ? <ChevronRight className="text-muted-foreground ml-auto size-3.5 shrink-0" /> : null}
            </Link>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
