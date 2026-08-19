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
import {Bot, ChevronRight, CircleX, Copy, RefreshCw} from "lucide-react";
import copy from "copy-to-clipboard";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {PageHeader} from "@/components/FormRow";
import {UnauthorizedResult} from "@/components/Result";
import {StatisticCard} from "@/components/charts/ChartCards";
import {Alert, AlertDescription} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader} from "@/components/ui/card";
import {ConfirmButton} from "@/components/ui/confirm-button";
import {Spinner} from "@/components/ui/spinner";
import {Switch} from "@/components/ui/switch";
import {Tooltip} from "@/components/ui/tooltip";
import {activityOf, agentDetailPath, agentKey, useAgents, useAgentSessions} from "@/lib/agents";
import type {Account, Agent} from "@/types";

/** One labelled line inside an agent card. */
function CardRow({label, children}: {label: string; children: React.ReactNode}) {
  return (
    <div className="flex items-baseline gap-2">
      <span className="shrink-0 text-xs text-muted-foreground">{label}</span>
      <span className="min-w-0 flex-1 text-right">{children}</span>
    </div>
  );
}

function AgentCard({
  agent,
  busy,
  activity,
  onToggle,
}: {
  agent: Agent;
  busy: boolean;
  activity?: {sessionCount: number; recordCount: number; lastTime: string};
  onToggle: () => void;
}) {
  const action = i18next.t(`agent:${agent.patched ? "Unpatch" : "Patch"}`);
  const note = [agent.notice, agent.followup].filter(Boolean).join(" ");

  const toggle = (
    <Switch
      checked={agent.patched}
      disabled={!agent.supported || busy}
      aria-label={action}
      // The confirmation dialog owns the click, so the switch never flips on
      // its own - it only ever mirrors what the last scan reported.
      onCheckedChange={() => undefined}
    />
  );

  return (
    <Card className="flex flex-col transition-colors hover:border-primary/50">
      <CardHeader className="flex flex-row items-start gap-3 space-y-0 p-4">
        <AgentIcon
          agent={agent.agentId || agent.name}
          size={40}
          fallback={<Bot className="h-10 w-10 text-muted-foreground" />}
        />
        <div className="min-w-0 flex-1">
          <Link
            to={agentDetailPath(agent)}
            className="block truncate font-semibold hover:text-primary hover:underline"
          >
            {agent.name}
          </Link>
          <div className="mt-1 flex flex-wrap items-center gap-1.5">
            <Badge variant="secondary">{agent.version || i18next.t("agent:Unknown")}</Badge>
            {agent.installMethod ? <Badge variant="secondary">{agent.installMethod}</Badge> : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {busy ? <Spinner /> : null}
          {agent.supported ? (
            // The dialog trigger clones its child to attach the click, so the
            // switch has to be that child - a wrapper that is not a DOM element
            // would drop the handler and the switch would do nothing.
            <ConfirmButton
              title={`${action} ${agent.name}?`}
              description={note || undefined}
              okText={action}
              destructive={agent.patched}
              onConfirm={onToggle}
            >
              {toggle}
            </ConfirmButton>
          ) : (
            <Tooltip title={agent.detail || i18next.t("agent:Not supported")}>
              <span>{toggle}</span>
            </Tooltip>
          )}
        </div>
      </CardHeader>

      <CardContent className="flex flex-1 flex-col gap-2 p-4 pt-0 text-sm">
        <CardRow label={i18next.t("general:Path")}>
          <span className="flex items-center justify-end gap-1">
            <Tooltip title={agent.path}>
              <code className="truncate text-xs">{agent.path}</code>
            </Tooltip>
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 shrink-0"
              aria-label={i18next.t("agent:Copy path")}
              onClick={() => {
                copy(agent.path);
                Setting.showMessage("success", i18next.t("agent:Path copied to clipboard"));
              }}
            >
              <Copy className="h-3.5 w-3.5" />
            </Button>
          </span>
        </CardRow>
        <CardRow label={i18next.t("general:Owner")}>
          <span className="truncate text-xs">{agent.owner}</span>
        </CardRow>

        <div className="mt-auto flex items-center justify-between border-t pt-3">
          {/* The patcher's own wording is the most precise thing there is about
              a given state, so it hangs off the status line as its tooltip. */}
          <Tooltip title={agent.detail}>
            <span className="min-w-0 truncate text-xs text-muted-foreground">
              {!agent.supported
                ? i18next.t("agent:Not supported")
                : !agent.patched
                  ? i18next.t("agent:Turn on monitoring to collect activity")
                  : activity
                    ? `${i18next
                      .t("agent:{count} sessions")
                      .replace("{count}", String(activity.sessionCount))} · ${new Date(
                      activity.lastTime,
                    ).toLocaleString()}`
                    : i18next.t("agent:Monitoring, no activity yet")}
            </span>
          </Tooltip>
          <Link
            to={agentDetailPath(agent)}
            className="inline-flex shrink-0 items-center text-xs text-primary hover:underline"
          >
            {i18next.t("agent:Details")}
            <ChevronRight className="h-3.5 w-3.5" />
          </Link>
        </div>
      </CardContent>
    </Card>
  );
}

export default function AgentDashboardPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {agents, loading, error, busyKey, scanned, scan, togglePatch} = useAgents(isAdmin);
  const {activity, recordCount} = useAgentSessions(isAdmin, "", 5000);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const patchedCount = agents.filter(agent => agent.patched).length;
  const sessionCount = Object.values(activity).reduce(
    (total, entry) => total + entry.sessionCount,
    0,
  );

  return (
    <div className="space-y-4 p-4 md:p-6">
      <PageHeader title={i18next.t("agent:Agents on this machine")}>
        <Button variant="outline" onClick={() => scan(true)} disabled={loading}>
          <RefreshCw className={loading ? "animate-spin" : undefined} />
          {i18next.t("agent:Scan")}
        </Button>
      </PageHeader>

      {error && (
        <Alert variant="destructive">
          <CircleX />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatisticCard title={i18next.t("agent:Installed")} value={agents.length} />
        <StatisticCard title={i18next.t("agent:Monitored")} value={patchedCount} />
        <StatisticCard title={i18next.t("agent:Agent Sessions")} value={sessionCount} />
        <StatisticCard title={i18next.t("agent:Records")} value={recordCount} />
      </div>

      {!scanned ? (
        <div className="flex justify-center p-10">
          <Spinner />
        </div>
      ) : agents.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 p-10 text-center">
            <Bot className="h-12 w-12 text-muted-foreground" />
            <div className="font-medium">{i18next.t("agent:No supported agents found")}</div>
            <div className="max-w-md text-sm text-muted-foreground">
              {i18next.t("agent:Install an AI agent on this machine, then scan again")}
            </div>
            <Button variant="outline" onClick={() => scan(true)} disabled={loading}>
              <RefreshCw className={loading ? "animate-spin" : undefined} />
              {i18next.t("agent:Scan")}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {agents.map(agent => (
            <AgentCard
              key={agentKey(agent)}
              agent={agent}
              busy={busyKey === agentKey(agent)}
              activity={activityOf(activity, agent)}
              onToggle={() => togglePatch(agent)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
