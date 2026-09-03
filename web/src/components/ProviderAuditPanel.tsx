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
import {Logs, RefreshCw, Stethoscope} from "lucide-react";
import i18next from "i18next";

import * as LlmRecordBackend from "@/backend/LlmRecordBackend";
import * as ProbeCaseBackend from "@/backend/ProbeCaseBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {ProviderAuditCard} from "@/components/ProviderAuditCard";
import {EmptyState, ErrorState} from "@/components/shared/empty-state";
import {Loading} from "@/components/shared/loading";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {providerIdOf} from "@/lib/providers";
import type {
  LlmAuditReport,
  LlmProviderAudit,
  ProbeCase,
  Provider,
  ProviderProbe,
  ProviderProbeMode,
} from "@/types";

/** The windows the traffic side offers, in hours. 0 is every record still kept. */
const windows = [24, 24 * 7, 24 * 30, 0];

function windowLabel(hours: number) {
  if (hours === 0) {
    return i18next.t("llm:All time");
  }
  if (hours === 24) {
    return i18next.t("llm:Last 24 hours");
  }
  return i18next.t("usage:Last {count} days").replace("{count}", String(hours / 24));
}

/** One provider, from whichever of the three sources happen to know it. */
interface AuditRow {
  id: string;
  provider?: Provider;
  audit?: LlmProviderAudit;
  probe?: ProviderProbe;
}

/**
 * Rows for every provider any source knows about: the ones configured now, plus
 * the ones only the records or an old probe still name. A provider that was
 * added a minute ago has no traffic and belongs here just as much as one that
 * has served for a month.
 */
function buildRows(
  providers: Provider[],
  audits: LlmProviderAudit[],
  probes: ProviderProbe[],
): AuditRow[] {
  const rows = new Map<string, AuditRow>();
  const rowOf = (id: string) => {
    const existing = rows.get(id);
    if (existing) {
      return existing;
    }
    const created: AuditRow = {id: id};
    rows.set(id, created);
    return created;
  };

  providers.forEach(provider => {
    rowOf(providerIdOf(provider)).provider = provider;
  });
  audits.forEach(audit => {
    rowOf(audit.provider).audit = audit;
  });
  probes.forEach(probe => {
    rowOf(probe.provider).probe = probe;
  });

  // Configured providers first, then the lowest score: what a page about which
  // upstreams are what they claim to be has to put at the top is the one that
  // answered least like the API it is sold as. Providers nobody could measure
  // sort below the ones that were, and traffic breaks the remaining ties.
  return [...rows.values()].sort((left, right) => {
    if (Boolean(left.provider) !== Boolean(right.provider)) {
      return left.provider ? -1 : 1;
    }
    const scored = (row: AuditRow) =>
      row.probe?.ok && row.probe.grade !== "unknown" ? row.probe.score : Number.POSITIVE_INFINITY;
    if (scored(left) !== scored(right)) {
      return scored(left) - scored(right);
    }
    const traffic = (right.audit?.requests ?? 0) - (left.audit?.requests ?? 0);
    return traffic !== 0 ? traffic : left.id.localeCompare(right.id);
  });
}

/**
 * The channel audit: what each upstream looks like from here, from both sides.
 * The traffic side reads records the proxy already kept and costs nothing. The
 * probe side asks the upstream directly — four short requests whose answers are
 * documented, which is the only way to learn something a real request never
 * happened to show — and spends a few cents of that provider's own credit.
 */
export function ProviderAuditPanel({owner}: {owner: string}) {
  const [hours, setHours] = React.useState(24 * 7);
  const [report, setReport] = React.useState<LlmAuditReport | null>(null);
  // Null until a listing lands. An empty list and "not asked yet" must not
  // render the same: every card would claim its provider is no longer
  // configured, which is the one thing this page must never get wrong.
  const [providers, setProviders] = React.useState<Provider[] | null>(null);
  const [probes, setProbes] = React.useState<ProviderProbe[]>([]);
  const [cases, setCases] = React.useState<ProbeCase[]>([]);
  const [probeMode, setProbeMode] = React.useState<ProviderProbeMode>("auto");
  const [probing, setProbing] = React.useState<string[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [recording, setRecording] = React.useState(true);

  React.useEffect(() => {
    LlmRecordBackend.getLlmRecordStatus()
      .then(res => setRecording(res.status !== "ok" || (res.data?.mode ?? "off") !== "off"))
      .catch(() => undefined);
  }, []);

  const loadProbes = React.useCallback(() => {
    ProviderBackend.getProviderProbes()
      .then(res => {
        if (res.status !== "ok") {
          return;
        }
        setProbes(res.data ?? []);
        setProbeMode(res.data2 ?? "auto");
      })
      .catch(() => undefined);
  }, []);

  React.useEffect(() => {
    ProbeCaseBackend.getProbeCases()
      .then(res => setCases(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined);
  }, []);

  const load = React.useCallback(() => {
    setLoading(true);
    loadProbes();
    // Reloaded with the rest, so a listing that failed once is not left to
    // mislabel every card until the page is opened again.
    ProviderBackend.getProviders(owner, 1, 100)
      .then(res => setProviders(res.status === "ok" ? (res.data ?? []) : null))
      .catch(() => setProviders(null));
    LlmRecordBackend.getLlmProviderAudit({windowHours: hours})
      .then(res => {
        if (res.status === "ok" && res.data) {
          setReport(res.data);
          setError("");
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)))
      .then(() => setLoading(false));
  }, [hours, owner, loadProbes]);

  React.useEffect(() => {
    load();
  }, [load]);

  // A probe started automatically finishes after the page has loaded, so the
  // list is asked again for a while rather than only on a refresh.
  React.useEffect(() => {
    const interval = setInterval(loadProbes, 20000);
    return () => clearInterval(interval);
  }, [loadProbes]);

  const runProbe = (id: string) => {
    setProbing(current => [...current, id]);
    ProviderBackend.probeProvider(id)
      .then(res => {
        if (res.status !== "ok" || !res.data) {
          Setting.showMessage("error", res.msg || i18next.t("audit:The probe could not run"));
          return;
        }
        const probe = res.data;
        setProbes(current => [probe, ...current.filter(item => item.provider !== id)]);
      })
      .catch(failure => Setting.showMessage("error", `${failure}`))
      .then(() => setProbing(current => current.filter(item => item !== id)));
  };

  const rows = buildRows(providers ?? [], report?.providers ?? [], probes);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-muted-foreground max-w-3xl text-sm">{i18next.t("audit:Page description")}</p>
        <div className="flex shrink-0 gap-2">
          <SimpleSelect
            className="w-[150px]"
            value={String(hours)}
            onChange={value => setHours(Number(value))}
            options={windows.map(option => ({label: windowLabel(option), value: String(option)}))}
          />
          <Button variant="outline" onClick={load} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        </div>
      </div>

      <MessageAlert
        variant="info"
        showIcon={false}
        title={i18next.t(`audit:Probe mode ${probeMode}`)}
        description={i18next.t("audit:Probe mode detail")}
      />

      {!recording ? (
        <MessageAlert
          variant="warning"
          title={i18next.t("llm:Recording is off")}
          description={i18next.t("audit:Recording is off detail")}
          action={
            <Button asChild size="sm" variant="outline">
              <Link to="/llm-records">
                <Logs />
                {i18next.t("llm:LLM Records")}
              </Link>
            </Button>
          }
        />
      ) : null}

      {report?.truncated ? (
        <MessageAlert
          variant="info"
          title={i18next
            .t("audit:Only the newest records were read")
            .replace("{count}", report.scanned.toLocaleString())}
        />
      ) : null}

      {error !== "" ? (
        <Card>
          <ErrorState error={error} onRetry={load} />
        </Card>
      ) : loading && report === null ? (
        <Loading />
      ) : rows.length === 0 ? (
        <Card>
          <EmptyState
            icon={Stethoscope}
            title={i18next.t("audit:Nothing to audit yet")}
            description={i18next.t("audit:Nothing to audit yet detail")}
          />
        </Card>
      ) : (
        <>
          <div className="space-y-3">
            {rows.map(row => (
              <ProviderAuditCard
                key={row.id}
                audit={row.audit}
                probe={row.probe}
                cases={cases}
                provider={row.provider}
                providersKnown={providers !== null}
                probeMode={probeMode}
                probing={probing.includes(row.id)}
                // A provider the records still name but nothing is configured
                // for has no endpoint left to ask.
                onProbe={row.provider && probeMode !== "off" ? () => runProbe(row.id) : undefined}
              />
            ))}
          </div>
          <p className="text-muted-foreground text-xs">
            {i18next
              .t("audit:Read from {count} records")
              .replace("{count}", (report?.scanned ?? 0).toLocaleString())}
            {" · "}
            {i18next.t("audit:What this cannot tell you")}
          </p>
        </>
      )}
    </div>
  );
}
