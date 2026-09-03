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

import i18next from "i18next";

import {cn} from "@/lib/utils";
import {formatScore, gradeLetter, gradeStyleOf} from "@/lib/authenticity";
import {Badge} from "@/components/ui/badge";
import {SimpleTooltip} from "@/components/ui/tooltip";
import type {ProbeGrade, ProviderProbe} from "@/types";

/**
 * The score as a dial. The number is the weighted share of the test cases that
 * could be measured which answered the way the vendor's own API documents, so
 * the ring is drawn over the same 0-100 the letter is cut from.
 */
export function ScoreDial({
  probe,
  size = 92,
  className,
}: {
  probe?: ProviderProbe;
  size?: number;
  className?: string;
}) {
  const grade: ProbeGrade = probe?.ok ? probe.grade : "unknown";
  const style = gradeStyleOf(grade);
  const measured = grade !== "unknown";
  const share = measured ? Math.max(0, Math.min(100, probe?.score ?? 0)) / 100 : 0;

  const stroke = Math.max(6, Math.round(size / 12));
  const radius = (size - stroke) / 2;
  const circumference = 2 * Math.PI * radius;

  return (
    <div className={cn("relative shrink-0", className)} style={{width: size, height: size}}>
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          strokeWidth={stroke}
          className="stroke-muted"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={`${circumference * share} ${circumference}`}
          className={cn("transition-[stroke-dasharray] duration-500", style.ring)}
          stroke="currentColor"
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className={cn("font-semibold tabular-nums", style.text)} style={{fontSize: size / 3.4}}>
          {formatScore(probe)}
        </span>
        <span className={cn("text-[10px] font-medium tracking-wide uppercase", style.text)}>
          {i18next.t("audit:Grade")} {gradeLetter(grade)}
        </span>
      </div>
    </div>
  );
}

/**
 * The same finding in one line, for a card that has no room for a dial. It
 * always says what it means: a provider nobody could measure reads as
 * unmeasured rather than as passing.
 */
export function ScoreBadge({
  probe,
  showLabel = true,
  className,
}: {
  probe?: ProviderProbe;
  showLabel?: boolean;
  className?: string;
}) {
  const grade: ProbeGrade = probe?.ok ? probe.grade : "unknown";
  const style = gradeStyleOf(grade);
  const title = probe
    ? `${i18next.t(style.label)} · ${i18next.t(style.verdict)}`
    : i18next.t("audit:Not probed yet");

  return (
    <SimpleTooltip title={title}>
      <Badge className={cn("gap-1.5 font-medium", style.badge, className)}>
        <span className="tabular-nums">
          {gradeLetter(grade)}
          {grade === "unknown" ? "" : ` · ${formatScore(probe)}`}
        </span>
        {showLabel ? <span className="hidden sm:inline">{i18next.t(style.label)}</span> : null}
      </Badge>
    </SimpleTooltip>
  );
}
