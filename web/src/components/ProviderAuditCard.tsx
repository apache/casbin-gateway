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
import {CircleAlert, CircleCheck, CircleHelp, Stethoscope, TriangleAlert} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {ProviderIcon} from "@/components/ProviderIcon";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {formatCost, formatTokens} from "@/lib/usage";
import type {
  LlmAuditCheck,
  LlmAuditLevel,
  LlmProviderAudit,
  ProbeCheck,
  Provider,
  ProviderProbe,
  ProviderProbeMode,
} from "@/types";

/** How many model badges fit before the rest become a count. */
const visibleModels = 4;

/** Mirrors llmAuditMinSample on the server, for the sentence that explains it. */
const minSample = 20;

const levelStyles: Record<LlmAuditLevel, {text: string; border: string; icon: React.ElementType}> = {
  ok: {text: "text-success", border: "border-success/25 bg-success/5", icon: CircleCheck},
  warn: {text: "text-warning", border: "border-warning/30 bg-warning/5", icon: TriangleAlert},
  alert: {text: "text-destructive", border: "border-destructive/30 bg-destructive/5", icon: CircleAlert},
  unknown: {text: "text-muted-foreground", border: "border-border", icon: CircleHelp},
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
  value,
  detail,
  level,
}: {
  title: React.ReactNode;
  value: React.ReactNode;
  detail: React.ReactNode;
  level: LlmAuditLevel;
}) {
  const style = levelStyles[level] ?? levelStyles.unknown;
  const Icon = style.icon;

  return (
    <div className={cn("flex flex-col gap-1 rounded-lg border p-3", style.border)}>
      <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
        <Icon className={cn("size-3.5 shrink-0", style.text)} />
        <span className="truncate">{title}</span>
      </div>
      <span className={cn("text-lg font-semibold tabular-nums", style.text)}>{value}</span>
      <span className="text-muted-foreground text-xs leading-snug">{detail}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// The probe: what the upstream answered when it was asked directly
// ---------------------------------------------------------------------------

/** Strongest evidence first, so the tile that matters is the one read first. */
const probeOrder: ProbeCheck["key"][] = ["identity", "cache", "billing", "stream", "tools", "vendor"];

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
  default:
    return i18next.t("audit:Vendor headers");
  }
}

function probeValue(check: ProbeCheck, probe: ProviderProbe) {
  if (check.level === "unknown") {
    return "—";
  }
  switch (check.key) {
  case "identity":
    return check.level === "ok" ? i18next.t("audit:Matches") : i18next.t("audit:Different");
  case "cache":
    return check.level === "ok" ? formatTokens(Number(check.facts[1] ?? 0)) : i18next.t("audit:None");
  case "billing":
    return check.facts[0] === "drift" ? i18next.t("audit:Unstable") : `${check.value.toFixed(2)}x`;
  case "stream":
    return check.level === "ok" ? i18next.t("audit:Complete") : String(check.facts.length);
  case "tools":
    return check.level === "ok" ? i18next.t("audit:Held") : i18next.t("audit:Lost");
  default:
    return String(probe.vendorHeaders.length);
  }
}

/**
 * What the probe found, worded from the data the check carries. Nothing here
 * calls a provider dishonest: it says what was asked and what came back, which
 * is the part that can be sent to the provider and argued about.
 */
function probeDetail(check: ProbeCheck, probe: ProviderProbe) {
  if (check.level === "unknown" && check.key !== "identity") {
    return i18next.t("audit:This question could not be asked");
  }

  switch (check.key) {
  case "identity":
    if (check.facts.length === 0) {
      return i18next.t("audit:No model name came back");
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
    return probe.vendorHeaders.length === 0
      ? i18next.t("audit:No vendor header")
      : fill("audit:Vendor detail", {names: probe.vendorHeaders.join(", ")});
  }
}

function ProbeSection({
  probe,
  probeMode,
  probing,
  onProbe,
}: {
  probe?: ProviderProbe;
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

  const checks = [...probe.checks].sort(
    (left, right) => probeOrder.indexOf(left.key) - probeOrder.indexOf(right.key),
  );

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
          <div className="grid grid-cols-2 gap-2 md:grid-cols-3 2xl:grid-cols-6">
            {checks.map(check => (
              <CheckTile
                key={check.key}
                title={probeTitle(check.key)}
                value={probeValue(check, probe)}
                detail={probeDetail(check, probe)}
                level={check.level}
              />
            ))}
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
  provider,
  providersKnown,
  probeMode,
  probing,
  onProbe,
}: {
  audit?: LlmProviderAudit;
  probe?: ProviderProbe;
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

        <ProbeSection probe={probe} probeMode={probeMode} probing={probing} onProbe={onProbe} />
        <TrafficSection audit={audit} />
      </CardContent>
    </Card>
  );
}
