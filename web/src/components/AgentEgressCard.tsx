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

import {DataTable, type Column} from "@/components/shared/data-table";
import {Fold} from "@/components/shared/fold";
import {MessageAlert} from "@/components/ui/alert";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {formatBytes} from "@/lib/agent-configs";
import {egressDetailOf, hasEgress, useAgentEgress} from "@/lib/egress";
import type {Agent, AgentRecord, EgressDestination, EgressSnapshot} from "@/types";

const kinds: Record<string, {label: string; variant: BadgeVariant}> = {
  model: {label: "Model API", variant: "muted"},
  storage: {label: "Object storage", variant: "danger"},
  telemetry: {label: "Telemetry", variant: "warning"},
  local: {label: "Local process", variant: "info"},
  other: {label: "Other", variant: "outline"},
};

function t(key: string, values: Record<string, string | number> = {}) {
  let text = i18next.t(`agent:${key}`);
  for (const name of Object.keys(values)) {
    text = text.replace(`{${name}}`, String(values[name]));
  }
  return text;
}

function KindBadge({kind}: {kind: string}) {
  const entry = kinds[kind] ?? kinds.other;
  return <Badge variant={entry.variant}>{t(entry.label)}</Badge>;
}

function bytes(value: number | undefined, counted: boolean) {
  return counted ? formatBytes(value) || "0 B" : "-";
}

/** A destination by name when the resolver had one; a local proxy by the
 *  process behind it, since that is all this host can see of where it went. */
function DestinationName({host, address, kind, via}: {host?: string; address: string; kind: string; via?: string}) {
  const primary = kind === "local" ? via || address : host || address;
  const secondary = primary === address ? "" : address;
  return (
    <div className="flex min-w-0 flex-col">
      <span className="truncate">{primary}</span>
      {secondary ? <code className="text-muted-foreground truncate text-xs">{secondary}</code> : null}
    </div>
  );
}

function SnapshotAlert({agent, snapshot}: {agent: Agent; snapshot: EgressSnapshot}) {
  const title = snapshot.accepted
    ? t("Snapshot uploaded", {agent: agent.name})
    : t("Snapshot packed", {agent: agent.name});
  const facts = [t("Snapshot size", {size: formatBytes(snapshot.encryptedBytes) || "-"})];
  if (snapshot.files) {
    facts.push(t("Snapshot files", {files: snapshot.files, gitFiles: snapshot.gitFiles ?? 0}));
  }
  if (snapshot.failures) {
    facts.push(t("Snapshot retries", {count: snapshot.failures}));
  }

  return (
    <MessageAlert
      variant={snapshot.accepted ? "destructive" : "warning"}
      title={title}
      description={
        <div className="space-y-1">
          <code className="block break-all text-xs">{snapshot.workspace}</code>
          <p>{facts.join(" · ")}</p>
          {snapshot.gitFiles ? <p>{t("Snapshot rotate secrets")}</p> : null}
        </div>
      }
    />
  );
}

function FindingsTable({findings, counted}: {findings: AgentRecord[]; counted: boolean}) {
  const columns: Column<AgentRecord>[] = [
    {
      title: t("Time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "180px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: t("Finding"),
      key: "action",
      width: "140px",
      render: (_value, record) => (
        <Badge variant="danger">
          {t(record.action === "large-upload" ? "Large upload" : "Object storage")}
        </Badge>
      ),
    },
    {
      title: t("Destination"),
      key: "destination",
      render: (_value, record) => {
        const detail = egressDetailOf(record);
        return detail ? <DestinationName {...detail} /> : <span>{String(record.object ?? "-")}</span>;
      },
    },
    {
      title: t("Sent"),
      key: "sent",
      width: "110px",
      render: (_value, record) => {
        const detail = egressDetailOf(record);
        return <span className="tabular-nums">{bytes(detail?.bytesOut, detail?.counted ?? counted)}</span>;
      },
    },
  ];

  return (
    <DataTable
      columns={columns}
      dataSource={findings}
      rowKey={record => String(record.id)}
      pageSize={0}
      emptyText={t("No suspicious uploads")}
    />
  );
}

function DestinationsTable({destinations, counted}: {destinations: EgressDestination[]; counted: boolean}) {
  const columns: Column<EgressDestination>[] = [
    {
      title: t("Destination"),
      key: "destination",
      render: (_value, dest) => <DestinationName {...dest} />,
    },
    {
      title: t("Kind"),
      key: "kind",
      width: "150px",
      render: (_value, dest) => (
        <span className="flex items-center gap-1">
          <KindBadge kind={dest.kind} />
          {dest.flagged ? <span className="bg-destructive size-1.5 rounded-full" /> : null}
        </span>
      ),
    },
    {
      title: t("Sent"),
      key: "bytesOut",
      width: "100px",
      render: (_value, dest) => <span className="tabular-nums">{bytes(dest.bytesOut, counted)}</span>,
    },
    {
      title: t("Received"),
      key: "bytesIn",
      width: "100px",
      render: (_value, dest) => <span className="tabular-nums">{bytes(dest.bytesIn, counted)}</span>,
    },
    {
      title: t("Last seen"),
      key: "lastSeen",
      dataIndex: "lastSeen",
      width: "180px",
      render: (value: string) => new Date(value).toLocaleString(),
    },
  ];

  return (
    <DataTable
      columns={columns}
      dataSource={destinations}
      rowKey={dest => `${dest.kind}|${dest.host ?? ""}|${dest.via ?? ""}|${dest.address}`}
      pageSize={0}
      emptyText={t("No connections yet")}
    />
  );
}

/** Where this agent's own processes send data. The proxy only sees what goes
 *  to the model; a repository posted straight to cloud storage shows here. */
export function AgentEgressCard({agent, enabled = true}: {agent: Agent; enabled?: boolean}) {
  const egress = useAgentEgress(agent.agentId, agent.owner, enabled);
  if (!hasEgress(egress)) {
    return null;
  }

  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-base font-semibold">{t("Outbound traffic")}</h2>
        {egress.supported && !egress.counted ? (
          <SimpleTooltip title={t("Bytes not counted hint")}>
            <span>
              <Badge variant="muted">{t("Bytes not counted")}</Badge>
            </span>
          </SimpleTooltip>
        ) : null}
      </div>
      <p className="text-muted-foreground text-sm">{t("Outbound traffic hint")}</p>

      {egress.snapshots.map(snapshot => (
        <SnapshotAlert key={snapshot.workspace} agent={agent} snapshot={snapshot} />
      ))}

      {egress.supported || egress.findings.length > 0 ? (
        <FindingsTable findings={egress.findings} counted={egress.counted} />
      ) : null}

      {egress.supported ? (
        <Fold title={t("Destinations ({count})", {count: egress.destinations.length})}>
          <DestinationsTable destinations={egress.destinations} counted={egress.counted} />
        </Fold>
      ) : null}
    </section>
  );
}
