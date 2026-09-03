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

import type {ProbeCase, ProbeCheck, ProbeGrade, ProviderProbe} from "@/types";

/** Mirrors the grade floors in object/provider_probe_score.go. */
export const gradeFloors: {grade: ProbeGrade; floor: number}[] = [
  {grade: "A", floor: 90},
  {grade: "B", floor: 75},
  {grade: "C", floor: 60},
  {grade: "D", floor: 40},
  {grade: "F", floor: 0},
];

/**
 * How each grade is painted and worded. The wording is deliberately about what
 * was measured rather than about the provider's character: an F says the answers
 * did not match what the API documents, which is a finding to send to whoever
 * sold the key.
 */
export const gradeStyles: Record<
  ProbeGrade,
  {text: string; badge: string; ring: string; label: string; verdict: string}
> = {
  A: {
    text: "text-success",
    badge: "border-success/30 bg-success/10 text-success",
    ring: "text-success",
    label: "audit:Grade A",
    verdict: "audit:Grade A detail",
  },
  B: {
    text: "text-success",
    badge: "border-success/30 bg-success/10 text-success",
    ring: "text-success",
    label: "audit:Grade B",
    verdict: "audit:Grade B detail",
  },
  C: {
    text: "text-warning",
    badge: "border-warning/30 bg-warning/10 text-warning",
    ring: "text-warning",
    label: "audit:Grade C",
    verdict: "audit:Grade C detail",
  },
  D: {
    text: "text-warning",
    badge: "border-warning/30 bg-warning/10 text-warning",
    ring: "text-warning",
    label: "audit:Grade D",
    verdict: "audit:Grade D detail",
  },
  F: {
    text: "text-destructive",
    badge: "border-destructive/30 bg-destructive/10 text-destructive",
    ring: "text-destructive",
    label: "audit:Grade F",
    verdict: "audit:Grade F detail",
  },
  unknown: {
    text: "text-muted-foreground",
    badge: "border-border bg-muted text-muted-foreground",
    ring: "text-muted-foreground",
    label: "audit:Grade unknown",
    verdict: "audit:Grade unknown detail",
  },
};

export function gradeStyleOf(grade: ProbeGrade | undefined) {
  return gradeStyles[grade ?? "unknown"] ?? gradeStyles.unknown;
}

/** The grade as it is written: a letter, or a dash where nothing was measured. */
export function gradeLetter(grade: ProbeGrade | undefined) {
  return !grade || grade === "unknown" ? "—" : grade;
}

export function formatScore(probe: ProviderProbe | undefined) {
  if (!probe || probe.grade === "unknown" || !probe.ok) {
    return "—";
  }
  return `${Math.round(probe.score)}`;
}

/** The checks that did not pass, which are what a summary line has to name. */
export function probeFindings(probe: ProviderProbe | undefined) {
  const checks = probe?.checks ?? [];
  return {
    alerts: checks.filter(check => check.level === "alert"),
    warnings: checks.filter(check => check.level === "warn"),
    measured: checks.filter(check => check.level !== "unknown"),
  };
}

/** One line for the whole machine, which is what the home screen carries. */
export interface AuthenticitySummary {
  /** Providers with a probe that measured something. */
  graded: number;
  /** Providers configured but not yet graded. */
  ungraded: number;
  alerting: number;
  /** The lowest grade any provider holds, which is the one worth acting on. */
  worst: ProbeGrade;
  worstProvider: string;
  /** The weighted-average score across graded providers. */
  average: number;
}

const gradeOrder: ProbeGrade[] = ["F", "D", "C", "B", "A", "unknown"];

export function summarizeAuthenticity(
  probes: ProviderProbe[],
  providerCount: number,
): AuthenticitySummary {
  const graded = probes.filter(probe => probe.ok && probe.grade !== "unknown");
  const worst = graded.reduce<ProviderProbe | undefined>((lowest, probe) => {
    if (!lowest) {
      return probe;
    }
    return probe.score < lowest.score ? probe : lowest;
  }, undefined);

  return {
    graded: graded.length,
    ungraded: Math.max(providerCount - graded.length, 0),
    alerting: graded.filter(probe => probeFindings(probe).alerts.length > 0).length,
    worst: worst?.grade ?? "unknown",
    worstProvider: worst?.provider ?? "",
    average:
      graded.length === 0
        ? 0
        : graded.reduce((total, probe) => total + probe.score, 0) / graded.length,
  };
}

export function isWorseGrade(left: ProbeGrade, right: ProbeGrade) {
  return gradeOrder.indexOf(left) < gradeOrder.indexOf(right);
}

/**
 * A built-in case is stored in English, because it is a row someone may edit.
 * Where this build ships a translation of that row it is preferred, so a reader
 * is not shown English for a case nobody has touched; a case someone wrote
 * shows exactly as it was typed.
 */
function localized(probeCase: ProbeCase, field: "displayName" | "question" | "method") {
  const key = `audit:case.${probeCase.name}.${field}`;
  if (probeCase.builtIn && !probeCase.edited && i18next.exists(key)) {
    return i18next.t(key);
  }
  return probeCase[field];
}

export function caseTitle(probeCase: ProbeCase) {
  return localized(probeCase, "displayName") || probeCase.name;
}

export function caseQuestion(probeCase: ProbeCase) {
  return localized(probeCase, "question");
}

export function caseMethod(probeCase: ProbeCase) {
  return localized(probeCase, "method");
}

/**
 * What a check was called. The name stored on the check is what the case was
 * called when it ran, which is what an old report has to keep saying; the live
 * case is preferred where it still exists, so a renamed case renames its tile.
 */
export function checkTitle(check: ProbeCheck, cases: ProbeCase[]) {
  const known = cases.find(probeCase => probeCase.name === check.case);
  return known ? caseTitle(known) : check.title || check.case || check.key;
}
