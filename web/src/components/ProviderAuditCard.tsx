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
import {FileSearch, Stethoscope} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {GradeScaleTip, ScoreDial} from "@/components/AuthenticityScore";
import {ProviderIcon} from "@/components/ProviderIcon";
import {CodeBlock} from "@/components/shared/misc";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {checkQuestion, checkTitle, gradeStyleOf, probeFindings} from "@/lib/authenticity";
import {formatCost, formatTokens} from "@/lib/usage";
import type {
  LlmAuditCheck,
  LlmAuditLevel,
  LlmProviderAudit,
  ProbeCase,
  ProbeCheck,
  Provider,
  ProviderProbe,
  ProviderProbeMode,
} from "@/types";

/** How many model badges fit before the rest become a count. */
const visibleModels = 4;

/** Mirrors llmAuditMinSample on the server, for the sentence that explains it. */
const minSample = 20;

// A tile is a neutral surface. The level is a dot beside the title, and only a
// case that failed colours anything larger than that.
const levelStyles: Record<LlmAuditLevel, {dot: string; value: string; border: string}> = {
  ok: {dot: "bg-success", value: "text-foreground", border: ""},
  warn: {dot: "bg-warning", value: "text-foreground", border: ""},
  alert: {dot: "bg-destructive", value: "text-destructive", border: "border-destructive/35"},
  unknown: {dot: "bg-muted-foreground/40", value: "text-muted-foreground", border: ""},
};

function percent(share: number) {
  const value = share * 100;
  if (value > 0 && value < 1) {
    return "<1%";
  }
  return `${Math.round(value)}%`;
}

function formatMs(ms: number) {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  return `${(ms / 1000).toFixed(ms < 10000 ? 1 : 0)}s`;
}

/** Fills {name} placeholders, which is all the interpolation these strings need. */
function fill(key: string, values: Record<string, string | number>) {
  return Object.entries(values).reduce(
    (text, [name, value]) => text.split(`{${name}}`).join(String(value)),
    i18next.t(key),
  );
}

/** One measurement, whichever side it was measured from. */
function CheckTile({
  title,
  question,
  caseName,
  value,
  detail,
  quote,
  level,
  weight,
  footer,
}: {
  title: React.ReactNode;
  /** What was asked. An answer read without its question explains nothing. */
  question?: string;
  /** The case behind the question, which the question links to. */
  caseName?: string;
  value: React.ReactNode;
  detail: React.ReactNode;
  /** The upstream's own words, kept apart from the sentence Gateway wrote
   * about them: one of the two is evidence and the other is a reading of it. */
  quote?: string;
  level: LlmAuditLevel;
  /** What this case was worth in the score, where it counted towards one. */
  weight?: number;
  /** Kept under the detail, which is where the evidence is opened from. */
  footer?: React.ReactNode;
}) {
  const style = levelStyles[level] ?? levelStyles.unknown;

  return (
    <div className={cn("bg-card flex flex-col gap-1 rounded-lg border p-3", style.border)}>
      <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
        <span className={cn("size-1.5 shrink-0 rounded-full", style.dot)} />
        <span className="truncate">{title}</span>
        {weight && level !== "unknown" ? (
          <SimpleTooltip title={i18next.t("audit:Weight hint")}>
            <span className="ml-auto shrink-0 tabular-nums opacity-70">{`×${weight}`}</span>
          </SimpleTooltip>
        ) : null}
      </div>
      {question ? (
        caseName ? (
          <SimpleTooltip title={i18next.t("audit:See how this is judged")}>
            <Link
              to={`/authenticity?tab=cases&case=${encodeURIComponent(caseName)}`}
              className="hover:text-foreground text-foreground/75 text-xs leading-snug underline decoration-dotted underline-offset-2"
            >
              {question}
            </Link>
          </SimpleTooltip>
        ) : (
          <span className="text-foreground/75 text-xs leading-snug">{question}</span>
        )
      ) : null}
      <span className={cn("text-lg font-semibold tabular-nums", style.value)}>{value}</span>
      <span className="text-muted-foreground text-xs leading-snug">{detail}</span>
      {quote ? (
        <p className="text-foreground/75 line-clamp-3 border-l-2 pl-2 font-mono text-[11px] leading-snug break-words whitespace-pre-wrap">
          {quote}
        </p>
      ) : null}
      {footer}
    </div>
  );
}

/** One block of the evidence, left out entirely where there is none. */
function EvidenceBlock({label, value}: {label: string; value: string}) {
  if (!value) {
    return null;
  }
  return (
    <div className="flex flex-col gap-1">
      <span className="text-muted-foreground text-xs font-medium">{label}</span>
      {/* The copy button floats over the first line, which a short answer
          otherwise runs into. */}
      <CodeBlock copyable maxHeight="16rem" className="[&_pre]:pr-10">
        {value}
      </CodeBlock>
    </div>
  );
}

/**
 * What the case actually asked and what actually came back. The tile is a
 * verdict and this is what it was drawn from, which is the part worth sending
 * to whoever sold the key: a level nobody can see the answer behind is an
 * opinion rather than a finding.
 */
function ProbeEvidence({
  check,
  title,
  question,
  detail,
}: {
  check: ProbeCheck;
  title: React.ReactNode;
  question?: string;
  detail: React.ReactNode;
}) {
  const [open, setOpen] = React.useState(false);

  if (!check.got && !check.want && !check.sent) {
    return null;
  }

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="text-muted-foreground hover:text-foreground mt-0.5 flex w-fit items-center gap-1 text-xs transition-colors"
      >
        <FileSearch className="size-3" />
        {i18next.t("audit:See what came back")}
      </button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[85vh] gap-3 sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{question || i18next.t("audit:Evidence detail")}</DialogDescription>
          </DialogHeader>

          <div className="flex flex-col gap-3 overflow-y-auto">
            <p className="text-xs leading-relaxed">{detail}</p>
            <EvidenceBlock label={i18next.t("audit:What came back")} value={check.got} />
            <EvidenceBlock label={i18next.t("audit:What a pass looks like")} value={check.want} />
            <EvidenceBlock label={i18next.t("audit:What was sent")} value={check.sent} />
            <p className="text-muted-foreground text-xs leading-relaxed">
              {i18next.t("audit:Judged here")}
            </p>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

// ---------------------------------------------------------------------------
// The probe: what the upstream answered when it was asked directly
// ---------------------------------------------------------------------------

/** Strongest evidence first, so the tile that matters is the one read first. */
const probeOrder: ProbeCheck["key"][] = [
  "identity",
  "selfid",
  "hidden",
  "knowledge",
  "feature",
  "repeat",
  "cache",
  "billing",
  "stream",
  "tools",
  "vendor",
];

function probeTitle(key: ProbeCheck["key"]) {
  switch (key) {
  case "identity":
    return i18next.t("audit:Model identity");
  case "cache":
    return i18next.t("audit:Prompt cache");
  case "billing":
    return i18next.t("audit:Token billing");
  case "stream":
    return i18next.t("audit:Stream shape");
  case "tools":
    return i18next.t("audit:Tool schema");
  case "knowledge":
    return i18next.t("audit:Test bank");
  case "selfid":
    return i18next.t("audit:Self-reported maker");
  case "hidden":
    return i18next.t("audit:Hidden instructions");
  case "feature":
    return i18next.t("audit:Documented parameter");
  case "repeat":
    return i18next.t("audit:Repeated request");
  default:
    return i18next.t("audit:Vendor headers");
  }
}

function probeValue(check: ProbeCheck) {
  if (check.level === "unknown") {
    return "—";
  }
  switch (check.key) {
  case "identity":
    if (check.facts[1] === "unverified") {
      return i18next.t("audit:Echoed");
    }
    return check.level === "ok" ? i18next.t("audit:Matches") : i18next.t("audit:Different");
  case "knowledge":
    if (check.level === "ok") {
      return i18next.t("audit:Right");
    }
    return check.level === "alert" ? i18next.t("audit:Wrong") : i18next.t("audit:Unsaid");
  case "selfid":
    if (check.level === "ok") {
      return i18next.t("audit:Matches");
    }
    return check.level === "alert" ? i18next.t("audit:Different") : i18next.t("audit:Unsaid");
  case "hidden":
    return check.level === "ok" ? i18next.t("audit:None") : i18next.t("audit:Found");
  case "feature":
    return check.level === "ok" ? i18next.t("audit:Honored") : i18next.t("audit:Dropped");
  case "repeat":
    if (check.level === "ok") {
      return `${check.facts[1] ?? ""}×`;
    }
    return check.level === "alert" ? i18next.t("audit:Different") : i18next.t("audit:Varies");
  case "cache":
    return check.level === "ok" ? formatTokens(Number(check.facts[1] ?? 0)) : i18next.t("audit:None");
  case "billing":
    return check.facts[0] === "drift" ? i18next.t("audit:Unstable") : `${check.value.toFixed(2)}x`;
  case "stream":
    return check.level === "ok" ? i18next.t("audit:Complete") : String(check.facts.length);
  case "tools":
    return check.level === "ok" ? i18next.t("audit:Held") : i18next.t("audit:Lost");
  default:
    return String(check.facts.length);
  }
}

/**
 * What the probe found, worded from the data the check carries. Nothing here
 * calls a provider dishonest: it says what was asked and what came back, which
 * is the part that can be sent to the provider and argued about.
 */
/**
 * The cases that read the answer rather than the envelope. The sentence says
 * what was made of the answer; the answer itself is quoted under it, so nobody
 * has to take the reading on trust.
 */
function probeAnswerDetail(check: ProbeCheck) {
  const [outcome, answer, extra, wanted] = check.facts;
  if (outcome === "failed") {
    return fill("audit:Question failed", {reason: answer ?? ""});
  }
  if (outcome === "empty") {
    return i18next.t("audit:Answer empty");
  }

  switch (check.key) {
  case "knowledge":
    if (outcome === "missed") {
      return fill("audit:Answer wrong", {expected: extra ?? ""});
    }
    if (outcome === "forbidden") {
      return fill("audit:Answer forbidden", {forbidden: extra ?? ""});
    }
    return i18next.t("audit:Answer right");
  case "selfid":
    if (outcome === "undocumented") {
      return i18next.t("audit:Self undocumented");
    }
    if (outcome === "other") {
      return fill("audit:Self other", {other: extra ?? "", vendor: wanted ?? ""});
    }
    if (outcome === "silent") {
      return fill("audit:Self silent", {vendor: extra ?? ""});
    }
    return fill("audit:Self match", {vendor: extra ?? ""});
  case "hidden":
    return outcome === "hidden" ? i18next.t("audit:Hidden found") : i18next.t("audit:Hidden none");
  case "feature":
    if (outcome === "rejected") {
      return fill("audit:Parameter rejected", {reason: answer ?? ""});
    }
    if (outcome === "ignored") {
      return fill("audit:Parameter ignored", {field: answer ?? ""});
    }
    if (outcome === "shape") {
      return fill("audit:Parameter shape", {field: answer ?? "", value: extra ?? "", wanted: wanted ?? ""});
    }
    if (outcome === "dropped") {
      return fill("audit:Parameter dropped", {answer: answer ?? "", wanted: extra ?? ""});
    }
    return i18next.t("audit:Parameter honored");
  default:
    if (outcome === "model") {
      return fill("audit:Repeat models", {names: check.facts.slice(1).join(", ")});
    }
    if (outcome === "tokens") {
      return fill("audit:Repeat tokens", {counts: check.facts.slice(1).join(", ")});
    }
    if (outcome === "answers") {
      return i18next.t("audit:Repeat answers");
    }
    return fill("audit:Repeat same", {count: answer ?? ""});
  }
}

const answerKeys: ProbeCheck["key"][] = ["knowledge", "selfid", "hidden", "feature", "repeat"];

/** The cases whose answer is writing, which is what a tile can quote as it is. */
const quotedKeys: ProbeCheck["key"][] = ["knowledge", "selfid", "hidden", "repeat"];

/**
 * The upstream's own words, for the tile to show under the finding. A question
 * that never got asked has none to show: the sentence already carries the
 * refusal, and there is nothing the model wrote to quote.
 */
function probeQuote(check: ProbeCheck) {
  if (!quotedKeys.includes(check.key) || !check.got) {
    return "";
  }
  return ["failed", "empty", "undocumented"].includes(check.facts[0] ?? "") ? "" : check.got;
}

function probeDetail(check: ProbeCheck, probe: ProviderProbe) {
  if (answerKeys.includes(check.key)) {
    return probeAnswerDetail(check);
  }
  if (check.level === "unknown" && check.key !== "identity") {
    // An endpoint that is nobody's own has no documented headers to be missing,
    // which is a gap in what Gateway knows rather than one in the answer.
    return check.facts[0] === "undocumented"
      ? i18next.t("audit:Vendor undocumented")
      : i18next.t("audit:This question could not be asked");
  }

  switch (check.key) {
  case "identity":
    if (check.facts.length === 0) {
      return i18next.t("audit:No model name came back");
    }
    if (check.facts[1] === "alias") {
      return fill("audit:Identity alias", {asked: probe.model, answered: check.facts[0]});
    }
    if (check.facts[1] === "unverified") {
      return fill("audit:Identity unverified", {answered: check.facts[0]});
    }
    return check.level === "ok"
      ? fill("audit:Identity ok", {answered: check.facts[0]})
      : fill("audit:Identity different", {asked: probe.model, answered: check.facts[0]});
  case "cache": {
    const written = Number(check.facts[0] ?? 0);
    const read = Number(check.facts[1] ?? 0);
    if (read > 0) {
      return fill("audit:Cache probe ok", {written: written, read: read});
    }
    if (written > 0) {
      return fill("audit:Cache probe written only", {written: written});
    }
    return i18next.t("audit:Cache probe none");
  }
  case "billing":
    return check.facts[0] === "drift"
      ? fill("audit:Billing unstable", {first: check.facts[1], second: check.facts[2]})
      : fill("audit:Billing detail", {billed: check.facts[1], estimate: check.facts[2]});
  case "stream":
    return check.facts.length === 0
      ? i18next.t("audit:Stream complete")
      : fill("audit:Stream missing", {items: check.facts.join(", ")});
  case "tools":
    if (check.level === "ok") {
      return i18next.t("audit:Tools ok");
    }
    return check.level === "warn" ? i18next.t("audit:Tools partial") : i18next.t("audit:Tools none");
  default:
    if (check.facts[0] === "relayed") {
      return fill("audit:Vendor relayed", {vendor: check.facts[1] ?? ""});
    }
    return check.facts.length === 0
      ? i18next.t("audit:No vendor header")
      : fill("audit:Vendor detail", {names: check.facts.join(", ")});
  }
}

/**
 * The score, the letter and what they were drawn from. It is a summary of the
 * tiles below it and of nothing else: a case that could not be asked lowers no
 * score, and the sentence beside the dial says which cases decided it.
 */
function ScoreHeadline({probe}: {probe: ProviderProbe}) {
  const style = gradeStyleOf(probe.grade);
  const {alerts, warnings, measured} = probeFindings(probe);

  return (
    <div className="bg-card flex items-center gap-4 rounded-lg border p-3">
      <ScoreDial probe={probe} size={84} />
      <div className="flex min-w-0 flex-col gap-1">
        <div className="flex items-center gap-1.5">
          <span className="text-sm font-semibold">{i18next.t(style.label)}</span>
          <GradeScaleTip />
        </div>
        <span className="text-muted-foreground text-xs leading-snug">{i18next.t(style.verdict)}</span>
        <span className="text-muted-foreground text-xs">
          {fill("audit:Score from cases", {
            measured: measured.length,
            total: probe.checks.length,
          })}
          {alerts.length > 0
            ? ` · ${fill("audit:Failed cases", {count: alerts.length})}`
            : ""}
          {warnings.length > 0
            ? ` · ${fill("audit:Flagged cases", {count: warnings.length})}`
            : ""}
        </span>
      </div>
    </div>
  );
}

function ProbeSection({
  probe,
  cases,
  probeMode,
  probing,
  onProbe,
}: {
  probe?: ProviderProbe;
  /** The suite as it stands now, so a tile is named by the case it came from. */
  cases: ProbeCase[];
  probeMode: ProviderProbeMode;
  probing: boolean;
  onProbe?: () => void;
}) {
  const label = (
    <span className="text-xs font-semibold tracking-wide uppercase">{i18next.t("audit:Probe")}</span>
  );
  const runButton = onProbe ? (
    <Button size="xs" variant="outline" onClick={onProbe} loading={probing}>
      <Stethoscope />
      {probe ? i18next.t("audit:Probe again") : i18next.t("audit:Probe now")}
    </Button>
  ) : null;

  if (!probe) {
    return (
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          {label}
          <div className="ml-auto">{runButton}</div>
        </div>
        <p className="text-muted-foreground text-xs">
          {probeMode === "auto"
            ? i18next.t("audit:Not probed yet auto")
            : probeMode === "manual"
              ? i18next.t("audit:Not probed yet manual")
              : i18next.t("audit:Probing is off")}
        </p>
      </div>
    );
  }

  // A run stores the checks in the suite order it ran. A report from before
  // the suite was stored has none, so those fall back to strongest first.
  const ordered = probe.checks.some(check => check.case !== "");
  const checks = ordered
    ? probe.checks
    : [...probe.checks].sort((left, right) => probeOrder.indexOf(left.key) - probeOrder.indexOf(right.key));

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        {label}
        <Badge variant="muted">{i18next.t(`audit:Trigger ${probe.trigger}`)}</Badge>
        <span className="text-muted-foreground text-xs">{Setting.getFormattedDate(probe.createdTime)}</span>
        {probe.requests > 0 ? (
          <SimpleTooltip title={i18next.t("audit:Probe cost hint")}>
            <span className="text-muted-foreground text-xs">
              {fill("audit:Probe cost", {
                requests: probe.requests,
                cost: probe.priced ? formatCost(probe.cost) : "?",
              })}
            </span>
          </SimpleTooltip>
        ) : null}
        <div className="ml-auto">{runButton}</div>
      </div>

      {probe.ok ? (
        <>
          <ScoreHeadline probe={probe} />
          <div className="grid grid-cols-2 gap-2 md:grid-cols-3 2xl:grid-cols-6">
            {checks.map((check, index) => {
              const title = check.case ? checkTitle(check, cases) : probeTitle(check.key);
              const question = checkQuestion(check, cases);
              const detail = probeDetail(check, probe);
              return (
                <CheckTile
                  key={`${check.case}-${check.key}-${index}`}
                  title={title}
                  question={question}
                  caseName={check.case}
                  value={probeValue(check)}
                  detail={detail}
                  quote={probeQuote(check)}
                  level={check.level}
                  weight={check.weight}
                  footer={
                    <ProbeEvidence check={check} title={title} question={question} detail={detail} />
                  }
                />
              );
            })}
          </div>
          {probe.ttftMs > 0 ? (
            <p className="text-muted-foreground text-xs">
              {fill("audit:First byte", {ttft: formatMs(probe.ttftMs)})}
            </p>
          ) : null}
        </>
      ) : (
        <p className="text-destructive text-xs break-words">
          {fill("audit:Probe failed", {reason: probe.error})}
        </p>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// The traffic: what the kept records already said
// ---------------------------------------------------------------------------

function trafficTitle(key: LlmAuditCheck["key"]) {
  switch (key) {
  case "cache":
    return i18next.t("audit:Cache accounting");
  case "errors":
    return i18next.t("audit:Failed attempts");
  case "latency":
    return i18next.t("audit:Response time");
  default:
    return i18next.t("audit:Unpriced models");
  }
}

function trafficValue(check: LlmAuditCheck, audit: LlmProviderAudit) {
  if (check.key === "latency") {
    return audit.latencyP50Ms === 0 ? "—" : formatMs(audit.latencyP50Ms);
  }
  if (check.level === "unknown" && check.sample === 0) {
    return "—";
  }
  return percent(check.value);
}

function trafficDetail(check: LlmAuditCheck, audit: LlmProviderAudit) {
  if (check.level === "unknown" && check.sample > 0 && check.sample < minSample) {
    return fill("audit:Too few to measure", {min: minSample});
  }
  if (check.sample === 0) {
    return check.key === "cache"
      ? i18next.t("audit:Nothing could report a cache")
      : i18next.t("audit:Nothing was relayed in this window");
  }

  switch (check.key) {
  case "cache":
    return check.value === 0
      ? fill("audit:Cache never reported", {sample: check.sample})
      : fill("audit:Cache detail", {hits: audit.cacheHits, sample: check.sample});
  case "errors":
    return fill("audit:Failures detail", {
      failed: audit.failed + audit.failedOver,
      sample: check.sample,
    });
  case "latency":
    return fill("audit:Latency detail", {
      p95: formatMs(audit.latencyP95Ms),
      ratio: check.value.toFixed(1),
    });
  default:
    return audit.unpriced === 0
      ? i18next.t("audit:Every model has a rate")
      : fill("audit:Pricing detail", {unpriced: audit.unpriced, sample: check.sample});
  }
}

function Fact({label, value, tone}: {label: React.ReactNode; value: React.ReactNode; tone?: string}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className={cn("text-sm font-medium tabular-nums", tone)}>{value}</span>
    </div>
  );
}

function TrafficSection({audit}: {audit?: LlmProviderAudit}) {
  const label = (
    <span className="text-xs font-semibold tracking-wide uppercase">{i18next.t("audit:Traffic")}</span>
  );

  if (!audit) {
    return (
      <div className="flex flex-col gap-2">
        {label}
        <p className="text-muted-foreground text-xs">{i18next.t("audit:No traffic in this window")}</p>
      </div>
    );
  }

  const shownModels = audit.models.slice(0, visibleModels);

  return (
    <div className="flex flex-col gap-2">
      {label}
      <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
        {audit.checks.map(check => (
          <CheckTile
            key={check.key}
            title={trafficTitle(check.key)}
            value={trafficValue(check, audit)}
            detail={trafficDetail(check, audit)}
            level={check.level}
          />
        ))}
      </div>

      <div className="grid grid-cols-3 gap-3 pt-1 sm:grid-cols-6">
        <Fact label={i18next.t("llm:Requests")} value={audit.requests.toLocaleString()} />
        <Fact
          label={i18next.t("llm:Failed")}
          value={audit.failed.toLocaleString()}
          tone={audit.failed > 0 ? "text-warning" : undefined}
        />
        <SimpleTooltip title={i18next.t("audit:Failed over hint")}>
          <div>
            <Fact
              label={i18next.t("audit:Failed over")}
              value={audit.failedOver.toLocaleString()}
              tone={audit.failedOver > 0 ? "text-warning" : undefined}
            />
          </div>
        </SimpleTooltip>
        <Fact label={i18next.t("llm:Tokens")} value={formatTokens(audit.totalTokens)} />
        <Fact label={i18next.t("llm:Cost")} value={formatCost(audit.cost)} />
        <Fact label={i18next.t("llm:Cache read")} value={formatTokens(audit.cacheReadTokens)} />
      </div>

      {audit.models.length > 0 ? (
        <div className="flex flex-wrap items-center gap-1">
          {shownModels.map(model => (
            <Badge
              key={model}
              variant={audit.unpricedModels.includes(model) ? "warning" : "muted"}
              className="font-mono"
            >
              {model}
            </Badge>
          ))}
          {audit.models.length > shownModels.length ? (
            <SimpleTooltip title={audit.models.slice(visibleModels).join(", ")}>
              <Badge variant="outline">{`+${audit.models.length - shownModels.length}`}</Badge>
            </SimpleTooltip>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

// ---------------------------------------------------------------------------

/**
 * One provider from both sides: what it answered when it was asked directly,
 * and what the kept records say it has been doing. The card is a report and not
 * a score — every tile shows what came back and what it was compared against,
 * and the conclusion is left to whoever is paying the bill.
 */
export function ProviderAuditCard({
  audit,
  probe,
  cases,
  provider,
  providersKnown,
  probeMode,
  probing,
  onProbe,
}: {
  audit?: LlmProviderAudit;
  probe?: ProviderProbe;
  /** The suite as it stands now, which names the tiles. */
  cases: ProbeCase[];
  /** Missing when the records outlive the provider they name. */
  provider?: Provider;
  /** False while the provider listing has not landed, when a missing provider
   * means "not known yet" rather than "no longer configured". */
  providersKnown: boolean;
  probeMode: ProviderProbeMode;
  probing: boolean;
  /** Missing for a provider there is nothing left to probe. */
  onProbe?: () => void;
}) {
  const id = provider ? `${provider.owner}/${provider.name}` : (audit?.provider ?? probe?.provider ?? "");
  const name = provider?.displayName || id.split("/").slice(-1)[0];
  const lastSeen = audit?.lastTime || probe?.createdTime || "";

  return (
    <Card>
      <CardContent className="flex flex-col gap-4 p-4">
        <div className="flex min-w-0 items-center gap-2">
          <ProviderIcon icon={provider?.icon} baseUrl={provider?.baseUrl} alt={name} size={20} />
          <div className="flex min-w-0 flex-col">
            <span className="truncate text-sm font-medium">{name}</span>
            <span className="text-muted-foreground truncate font-mono text-xs">{id}</span>
          </div>
          <div className="ml-auto flex shrink-0 items-center gap-2">
            {/* The records outlive the provider they name, and a report about
                one that is no longer configured has to say so before it is read
                as advice about the current setup. */}
            {providersKnown && !provider ? (
              <Badge variant="muted">{i18next.t("audit:Removed")}</Badge>
            ) : null}
            {lastSeen ? (
              <SimpleTooltip title={i18next.t("audit:Last seen")}>
                <span className="text-muted-foreground text-xs">{Setting.getFormattedDate(lastSeen)}</span>
              </SimpleTooltip>
            ) : null}
          </div>
        </div>

        <ProbeSection
          probe={probe}
          cases={cases}
          probeMode={probeMode}
          probing={probing}
          onProbe={onProbe}
        />
        <TrafficSection audit={audit} />
      </CardContent>
    </Card>
  );
}
