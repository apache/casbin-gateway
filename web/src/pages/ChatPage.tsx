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
import {ArrowUp, MessageSquare, Plus, Square, Trash2, Wrench} from "lucide-react";
import i18next from "i18next";

import * as DriveBackend from "@/backend/DriveBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {EmptyState} from "@/components/shared/empty-state";
import {Fold} from "@/components/shared/fold";
import {Markdown} from "@/components/shared/markdown";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleSelect} from "@/components/shared/simple-select";
import {UnauthorizedResult} from "@/components/shared/misc";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {cn} from "@/lib/utils";
import type {Account, DrivableAgent, DrivenEvent, DrivenSession} from "@/types";

/** One tool the agent reached for, and what came back. */
interface ToolBlock {
  kind: "tool";
  seq: number;
  name: string;
  input: string;
  result?: string;
}

type Block = {kind: "text" | "thinking"; seq: number; text: string} | ToolBlock;

/** One exchange: what was asked, and everything said before the next question. */
interface Turn {
  seq: number;
  prompt: string;
  blocks: Block[];
  // notices are what the agent mentioned in passing; they are shown beside the
  // answer rather than in it, because the turn carried on regardless.
  notices: string[];
  error: string;
  usage?: DrivenEvent;
  done: boolean;
}

/**
 * Flattens the event feed into exchanges. Consecutive pieces of prose are joined
 * so a streamed answer reads as one paragraph rather than as the fragments the
 * agent happened to send it in.
 */
function toTurns(events: DrivenEvent[]): Turn[] {
  const turns: Turn[] = [];
  let turn: Turn | null = null;

  const append = (kind: "text" | "thinking", event: DrivenEvent) => {
    if (!turn || !event.text) {
      return;
    }
    const last = turn.blocks[turn.blocks.length - 1];
    if (last && last.kind === kind) {
      last.text += event.text;
      return;
    }
    turn.blocks.push({kind: kind, seq: event.seq, text: event.text});
  };

  for (const event of events) {
    switch (event.type) {
    case "prompt":
      turn = {seq: event.seq, prompt: event.text ?? "", blocks: [], notices: [], error: "", done: false};
      turns.push(turn);
      break;
    case "text":
      append("text", event);
      break;
    case "thinking":
      append("thinking", event);
      break;
    case "toolUse":
      turn?.blocks.push({
        kind: "tool",
        seq: event.seq,
        name: event.toolName ?? "",
        input: event.toolInput ?? "",
      });
      break;
    case "toolResult": {
      const tool = [...(turn?.blocks ?? [])].reverse().find(block => block.kind === "tool") as ToolBlock | undefined;
      if (tool) {
        tool.result = (tool.result ?? "") + (event.text ?? "");
      }
      break;
    }
    case "usage":
      if (turn && (event.inputTokens || event.outputTokens || event.costUsd)) {
        turn.usage = event;
      }
      break;
    case "error":
      if (turn) {
        turn.error = event.text ?? "";
      }
      break;
    case "notice":
      if (turn && event.text && !turn.notices.includes(event.text)) {
        turn.notices.push(event.text);
      }
      break;
    case "done":
      if (turn) {
        turn.done = true;
      }
      break;
    }
  }
  return turns;
}

function ToolRow({block}: {block: ToolBlock}) {
  return (
    <div className="bg-muted/40 rounded-md border text-xs">
      <div className="flex items-center gap-2 border-b px-2.5 py-1.5">
        <Wrench className="text-muted-foreground size-3.5 shrink-0" />
        <span className="font-medium">{block.name}</span>
      </div>
      <pre className="max-h-40 overflow-auto px-2.5 py-1.5 font-mono text-[11px] whitespace-pre-wrap">{block.input}</pre>
      {block.result ? (
        <pre className="text-muted-foreground max-h-40 overflow-auto border-t px-2.5 py-1.5 font-mono text-[11px] whitespace-pre-wrap">
          {block.result}
        </pre>
      ) : null}
    </div>
  );
}

function TurnView({turn}: {turn: Turn}) {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex justify-end">
        <div className="bg-primary/10 border-primary/20 max-w-[85%] rounded-lg border px-3 py-2 text-sm whitespace-pre-wrap">
          {turn.prompt}
        </div>
      </div>

      {turn.blocks.map(block => {
        if (block.kind === "tool") {
          return <ToolRow key={block.seq} block={block} />;
        }
        if (block.kind === "thinking") {
          return (
            <Fold key={block.seq} title={i18next.t("chat:Thinking")}>
              <div className="text-muted-foreground text-sm whitespace-pre-wrap italic">{block.text}</div>
            </Fold>
          );
        }
        return (
          <div key={block.seq} className="text-sm">
            <Markdown content={block.text} />
          </div>
        );
      })}

      {turn.notices.map(notice => (
        <MessageAlert key={notice} variant="warning" description={notice} />
      ))}

      {turn.error ? <MessageAlert variant="destructive" title={turn.error} /> : null}

      {turn.usage ? (
        <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
          <Badge variant="outline">
            {i18next.t("chat:{count} tokens in").replace("{count}", String(turn.usage.inputTokens ?? 0))}
          </Badge>
          <Badge variant="outline">
            {i18next.t("chat:{count} tokens out").replace("{count}", String(turn.usage.outputTokens ?? 0))}
          </Badge>
          {turn.usage.costUsd ? <Badge variant="outline">${turn.usage.costUsd.toFixed(4)}</Badge> : null}
        </div>
      ) : null}
    </div>
  );
}

/**
 * Driving an agent from Gateway. The agent that runs is the real one, with the
 * configuration and the hooks already written into it, so what it costs and what
 * it was allowed to do are recorded exactly as when somebody types at it.
 */
export default function ChatPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [agents, setAgents] = React.useState<DrivableAgent[]>([]);
  const [sessions, setSessions] = React.useState<DrivenSession[]>([]);
  const [activeId, setActiveId] = React.useState("");
  const [events, setEvents] = React.useState<DrivenEvent[]>([]);
  const [draft, setDraft] = React.useState("");
  const [target, setTarget] = React.useState("");
  const [workDir, setWorkDir] = React.useState("");
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const bottom = React.useRef<HTMLDivElement>(null);

  const active = sessions.find(session => session.id === activeId);
  const running = active?.state === "running";

  const loadSessions = React.useCallback(() => {
    return DriveBackend.getDrivenSessions()
      .then(res => {
        if (res.status === "ok") {
          setSessions(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)));
  }, []);

  React.useEffect(() => {
    if (!isAdmin) {
      setLoading(false);
      return;
    }
    Promise.all([
      DriveBackend.getDrivableAgents().then(res => {
        if (res.status === "ok") {
          const found = res.data ?? [];
          setAgents(found);
          setTarget(previous => previous || (found[0] ? found[0].agentId + "\n" + found[0].path : ""));
        }
      }),
      loadSessions(),
    ]).then(() => setLoading(false));
  }, [isAdmin, loadSessions]);

  // One stream per session: the server replays what was already said, so the
  // conversation is read from the feed rather than fetched separately.
  React.useEffect(() => {
    if (!activeId) {
      setEvents([]);
      return;
    }
    setEvents([]);
    const close = DriveBackend.streamDrivenSession(activeId, {
      onEvent: event => {
        setEvents(previous => (previous.some(seen => seen.seq === event.seq) ? previous : [...previous, event]));
        if (event.type === "done") {
          loadSessions();
        }
      },
    });
    return close;
  }, [activeId, loadSessions]);

  React.useEffect(() => {
    bottom.current?.scrollIntoView({behavior: "smooth"});
  }, [events.length]);

  const open = () => {
    const [agentId, path] = target.split("\n");
    const installation = agents.find(candidate => candidate.agentId === agentId && candidate.path === path);
    if (!installation) {
      return;
    }
    DriveBackend.openDrivenSession(
      {agentId: installation.agentId, path: installation.path, owner: installation.owner},
      workDir.trim(),
    )
      .then(res => {
        if (res.status === "ok" && res.data) {
          setSessions(previous => [res.data as DrivenSession, ...previous]);
          setActiveId((res.data as DrivenSession).id);
          setError("");
        } else {
          setError(res.msg || i18next.t("general:Failed to save"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)));
  };

  const send = () => {
    const prompt = draft.trim();
    if (!prompt || !activeId || running) {
      return;
    }
    setDraft("");
    DriveBackend.sendDrivenSession(activeId, prompt)
      .then(res => {
        if (res.status === "ok") {
          loadSessions();
        } else {
          setError(res.msg || i18next.t("general:Failed to save"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)));
  };

  const close = (id: string) => {
    DriveBackend.closeDrivenSession(id).then(() => {
      setSessions(previous => previous.filter(session => session.id !== id));
      setActiveId(previous => (previous === id ? "" : previous));
    });
  };

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const turns = toTurns(events);

  return (
    <PageContainer className="lg:h-[calc(100svh-3.25rem-4.5rem)]">
      <PageHeader
        title={i18next.t("chat:Chat")}
        description={i18next.t("chat:Ask an agent on this machine something, and watch it work")}
      />

      {error ? <MessageAlert variant="destructive" title={error} /> : null}

      {/* The chat pane owns the viewport, so only the transcript scrolls. */}
      <div className="grid min-h-0 gap-4 lg:flex-1 lg:grid-cols-[300px_1fr]">
        <div className="flex flex-col gap-3 lg:min-h-0 lg:overflow-y-auto">
          <div className="flex flex-col gap-2 rounded-lg border p-3">
            <SimpleSelect
              value={target}
              onChange={setTarget}
              size="sm"
              placeholder={i18next.t("chat:Pick an agent")}
              options={agents.map(candidate => ({
                value: candidate.agentId + "\n" + candidate.path,
                text: candidate.name,
                label: (
                  <span className="flex items-center gap-2">
                    <AgentIcon agent={candidate.name} size={16} />
                    <span className="truncate">{candidate.name}</span>
                  </span>
                ),
              }))}
            />
            <Input
              value={workDir}
              onChange={event => setWorkDir(event.target.value)}
              placeholder={i18next.t("chat:Working directory")}
              className="h-8 text-xs"
            />
            <Button size="sm" onClick={open} disabled={!target}>
              <Plus className="size-4" />
              {i18next.t("chat:New session")}
            </Button>
          </div>

          <div className="flex flex-col gap-1">
            {sessions.map(session => (
              <button
                key={session.id}
                type="button"
                onClick={() => setActiveId(session.id)}
                className={cn(
                  "group hover:bg-muted/60 flex items-center gap-2 rounded-md border border-transparent px-2 py-1.5 text-left text-sm",
                  session.id === activeId && "bg-muted border-border",
                )}
              >
                <AgentIcon agent={session.agentId} size={16} />
                <span className="min-w-0 flex-1 truncate">
                  {session.title || i18next.t("chat:New session")}
                  {session.state === "running" ? <span className="text-muted-foreground"> ...</span> : null}
                </span>
                <Trash2
                  className="text-muted-foreground hover:text-destructive size-3.5 shrink-0 opacity-0 group-hover:opacity-100"
                  onClick={event => {
                    event.stopPropagation();
                    close(session.id);
                  }}
                />
              </button>
            ))}
          </div>
        </div>

        <div className="flex min-h-[60vh] min-w-0 flex-col rounded-lg border lg:min-h-0">
          {active ? (
            <>
              <div className="text-muted-foreground flex items-center gap-2 border-b px-3 py-2 text-xs">
                <AgentIcon agent={active.agentId} size={14} />
                <span className="shrink-0 font-medium">{active.agentId}</span>
                <span className="min-w-0 flex-1 truncate font-mono" title={active.workDir}>
                  {active.workDir || "~"}
                </span>
                {active.resumable ? null : <Badge variant="outline">{i18next.t("chat:Each message stands alone")}</Badge>}
              </div>

              <div className="flex-1 overflow-y-auto px-3 py-4">
                {turns.length === 0 ? (
                  <EmptyState icon={MessageSquare} title={i18next.t("chat:Nothing asked yet")} />
                ) : (
                  <div className="flex flex-col gap-6">
                    {turns.map(turn => (
                      <TurnView key={turn.seq} turn={turn} />
                    ))}
                  </div>
                )}
                <div ref={bottom} />
              </div>

              <div className="flex items-end gap-2 border-t p-2">
                <Textarea
                  value={draft}
                  onChange={event => setDraft(event.target.value)}
                  onKeyDown={event => {
                    if (event.key === "Enter" && !event.shiftKey) {
                      event.preventDefault();
                      send();
                    }
                  }}
                  placeholder={i18next.t("chat:Ask the agent something")}
                  className="max-h-40 min-h-10 resize-none"
                />
                {running ? (
                  <Button size="icon" variant="outline" onClick={() => DriveBackend.interruptDrivenSession(activeId)}>
                    <Square className="size-4" />
                  </Button>
                ) : (
                  <Button size="icon" onClick={send} disabled={!draft.trim()}>
                    <ArrowUp className="size-4" />
                  </Button>
                )}
              </div>
            </>
          ) : (
            <EmptyState
              icon={MessageSquare}
              title={loading ? i18next.t("general:Loading") : i18next.t("chat:No session is open")}
              description={
                agents.length === 0 && !loading ? i18next.t("chat:No agent on this machine can be driven") : undefined
              }
            />
          )}
        </div>
      </div>
    </PageContainer>
  );
}
