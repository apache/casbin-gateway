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
import {ShieldCheck} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import {ScoreBadge, ScoreDial} from "@/components/AuthenticityScore";
import {ProviderIcon} from "@/components/ProviderIcon";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {cn} from "@/lib/utils";
import {gradeStyleOf, probeFindings, summarizeAuthenticity} from "@/lib/authenticity";
import {providerIdOf} from "@/lib/providers";
import type {Provider, ProviderProbe, ProviderProbeMode} from "@/types";

/** How often the scores are read again while the page is open. */
const refreshMs = 20000;

/** Fills {name} placeholders, which is all the interpolation these strings need. */
function fill(key: string, values: Record<string, string | number>) {
  return Object.entries(values).reduce(
    (text, [name, value]) => text.split(`{${name}}`).join(String(value)),
    i18next.t(key),
  );
}

/** One provider's line: who it is, what it scored, and what failed if anything. */
function ProviderScore({provider, probe}: {provider: Provider; probe?: ProviderProbe}) {
  const {alerts, warnings} = probeFindings(probe);
  const name = provider.displayName || provider.name;

  return (
    <div className="flex min-w-0 items-center gap-2 rounded-lg border px-3 py-2">
      <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} alt={name} size={18} />
      <span className="truncate text-sm">{name}</span>
      <div className="ml-auto flex shrink-0 items-center gap-2">
        {alerts.length > 0 ? (
          <Badge variant="danger">{fill("audit:Failed cases", {count: alerts.length})}</Badge>
        ) : warnings.length > 0 ? (
          <Badge variant="warning">{fill("audit:Flagged cases", {count: warnings.length})}</Badge>
        ) : null}
        <ScoreBadge probe={probe} showLabel={false} />
      </div>
    </div>
  );
}

/**
 * Whether the APIs on this machine are what they are sold as, in one card. It
 * is measured without being asked for: every provider is probed when it is
 * added and again once its report goes stale, so this says something before
 * anyone thinks to check.
 */
export function AuthenticityOverview({
  providers,
  showLink = true,
  className,
}: {
  providers: Provider[];
  /** False on the page this links to, where the link would point at itself. */
  showLink?: boolean;
  className?: string;
}) {
  const [probes, setProbes] = React.useState<ProviderProbe[]>([]);
  const [mode, setMode] = React.useState<ProviderProbeMode>("auto");
  const [loaded, setLoaded] = React.useState(false);

  React.useEffect(() => {
    const load = () => {
      ProviderBackend.getProviderProbes()
        .then(res => {
          if (res.status !== "ok") {
            return;
          }
          setProbes(res.data ?? []);
          setMode(res.data2 ?? "auto");
        })
        .catch(() => undefined)
        .then(() => setLoaded(true));
    };

    load();
    const interval = setInterval(load, refreshMs);
    return () => clearInterval(interval);
  }, []);

  const configured = providers.filter(provider => provider.status !== "disabled");
  const byProvider = new Map(probes.map(probe => [probe.provider, probe]));
  const scored = configured
    .map(provider => ({provider: provider, probe: byProvider.get(providerIdOf(provider))}))
    .sort((left, right) => {
      const score = (entry: {probe?: ProviderProbe}) =>
        entry.probe?.ok && entry.probe.grade !== "unknown" ? entry.probe.score : Number.POSITIVE_INFINITY;
      return score(left) - score(right);
    });

  const summary = summarizeAuthenticity(
    scored.filter(entry => entry.probe).map(entry => entry.probe as ProviderProbe),
    configured.length,
  );
  // The headline grade is the worst one held, not the average: a machine with
  // four honest providers and one that is not is not four fifths fine.
  const worst = scored.find(entry => entry.probe?.ok && entry.probe.grade !== "unknown")?.probe;
  const style = gradeStyleOf(worst?.grade);

  if (configured.length === 0) {
    return null;
  }

  return (
    <Card className={className}>
      <CardContent className="flex flex-col gap-4 p-4">
        <div className="flex flex-wrap items-center gap-4">
          <ScoreDial probe={worst} size={88} />
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex items-center gap-2">
              <ShieldCheck className={cn("size-4", style.text)} />
              <span className="text-sm font-semibold">{i18next.t("audit:Authenticity")}</span>
            </div>
            <span className={cn("text-sm", style.text)}>
              {summary.graded === 0
                ? i18next.t(mode === "off" ? "audit:Probing is off" : "audit:Measuring now")
                : i18next.t(style.verdict)}
            </span>
            <span className="text-muted-foreground text-xs">
              {fill("audit:Graded providers", {
                graded: summary.graded,
                total: configured.length,
              })}
              {summary.alerting > 0 ? ` · ${fill("audit:Providers alerting", {count: summary.alerting})}` : ""}
              {summary.ungraded > 0 ? ` · ${fill("audit:Providers unmeasured", {count: summary.ungraded})}` : ""}
            </span>
          </div>

          {showLink ? (
            <Button asChild variant="outline" className="ml-auto">
              <Link to="/authenticity">
                <ShieldCheck />
                {i18next.t("audit:See the evidence")}
              </Link>
            </Button>
          ) : null}
        </div>

        {loaded ? (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {scored.map(entry => (
              <ProviderScore
                key={providerIdOf(entry.provider)}
                provider={entry.provider}
                probe={entry.probe}
              />
            ))}
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
