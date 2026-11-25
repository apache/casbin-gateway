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
import {useTranslation} from "react-i18next";
import {Link, useParams, useSearchParams} from "react-router-dom";
import {ArrowLeft, Bot, ChevronDown, ChevronUp, Download, Image as ImageIcon, RefreshCw, Search, Wrench, X} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {CodeBlock, CopyButton, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {sessionResumeCommand} from "@/lib/agent-sessions";
import {transcriptFileName, transcriptMarkdown} from "@/lib/transcript-markdown";
import {findTranscriptMatches, type TranscriptMatch} from "@/lib/transcript-search";
import type {Account, AgentMessage, AgentMessageBlock, AgentTranscript} from "@/types";

const ROLE_VARIANTS: Record<string, BadgeVariant> = {
  user: "info",
  assistant: "success",
  system: "warning",
  developer: "warning",
  tool: "muted",
};

function highlightedText(text: string, matches: TranscriptMatch[], active: number) {
  const parts: React.ReactNode[] = [];
  let cursor = 0;
  for (const {start, end, index} of matches) {
    const current = index === active;
    parts.push(
      text.slice(cursor, start),
      <mark
        key={start}
        data-current-match={current ? "" : undefined}
        className={current ? "bg-primary text-primary-foreground rounded-sm" : "bg-warning/25 text-foreground rounded-sm"}
      >
        {text.slice(start, end)}
      </mark>,
    );
    cursor = end;
  }
  parts.push(text.slice(cursor));
  return parts;
}

function BlockView({block, matches, active}: {
  block: AgentMessageBlock;
  matches?: TranscriptMatch[];
  active: number;
}) {
  const text = matches && block.text ? highlightedText(block.text, matches, active) : block.text;
  switch (block.kind) {
  case "thinking":
    return (
      <div className="flex flex-wrap items-start gap-2">
        <Badge variant="muted">{i18next.t("agent:Thinking")}</Badge>
        {block.text ? (
          <pre className="text-muted-foreground scrollbar-thin min-w-0 flex-1 overflow-x-auto text-xs leading-relaxed whitespace-pre-wrap italic">
            {text}
          </pre>
        ) : (
          <span className="text-muted-foreground text-xs">{i18next.t("agent:Not recorded")}</span>
        )}
      </div>
    );
  case "toolUse":
    return (
      <div className="grid gap-1">
        <Badge variant="warning">
          <Wrench />
          {block.tool || i18next.t("agent:Tool call")}
        </Badge>
        {block.text ? <CodeBlock maxHeight="16rem">{text}</CodeBlock> : null}
      </div>
    );
  case "toolResult":
    return (
      <div className="grid gap-1">
        <Badge variant={block.isError ? "danger" : "muted"}>
          {block.isError ? i18next.t("agent:Tool error") : i18next.t("agent:Tool result")}
        </Badge>
        {block.text ? <CodeBlock maxHeight="16rem">{text}</CodeBlock> : null}
      </div>
    );
  case "image":
    return (
      <Badge variant="muted">
        <ImageIcon />
        {i18next.t("agent:Image")}
      </Badge>
    );
  default:
    return (
      <pre className="scrollbar-thin overflow-x-auto text-xs leading-relaxed whitespace-pre-wrap">{text}</pre>
    );
  }
}

const MessageView = React.memo(function MessageView({message, index, matches, active, language}: {
  message: AgentMessage;
  index: number;
  matches?: Map<number, TranscriptMatch[]>;
  active: number;
  language: string;
}) {
  return (
    <div className="rounded-lg border">
      <div className="bg-muted/40 flex flex-wrap items-center gap-2 border-b px-3 py-1.5">
        <Badge variant={ROLE_VARIANTS[message.role] ?? "secondary"}>{message.role}</Badge>
        <span className="text-muted-foreground text-xs">#{index + 1}</span>
        {message.time ? (
          <span className="text-muted-foreground text-xs">{new Date(message.time).toLocaleString(language)}</span>
        ) : null}
      </div>
      <div className="grid gap-3 px-3 py-2">
        {message.blocks.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("agent:Empty message")}</span>
        ) : (
          message.blocks.map((block, position) => (
            <BlockView key={position} block={block} matches={matches?.get(position)} active={active} />
          ))
        )}
      </div>
    </div>
  );
});

function TranscriptMessages({messages}: {messages: AgentMessage[]}) {
  const {i18n} = useTranslation();
  const [query, setQuery] = React.useState("");
  const [search, setSearch] = React.useState("");
  const [matchIndex, setMatchIndex] = React.useState(0);
  const {byMessage, messageIndices, limited} = React.useMemo(
    () => findTranscriptMatches(messages, search),
    [messages, search],
  );
  const content = React.useRef<HTMLDivElement>(null);
  const input = React.useRef<HTMLInputElement>(null);
  const count = messageIndices.length;
  const active = Math.min(matchIndex, count - 1);
  const pending = query.trim() !== search;
  const searchLabel = i18next.t("agent:Find in transcript");
  const previousLabel = i18next.t("agent:Previous transcript match");
  const nextLabel = i18next.t("agent:Next transcript match");
  const clearLabel = i18next.t("agent:Clear transcript search");

  React.useEffect(() => {
    if (!pending) {
      return;
    }
    const timer = window.setTimeout(() => setSearch(query.trim()), 150);
    return () => window.clearTimeout(timer);
  }, [pending, query]);

  React.useEffect(() => {
    if (!pending && active >= 0) {
      content.current?.querySelector("[data-current-match]")?.scrollIntoView({block: "center", inline: "nearest"});
    }
  }, [active, byMessage, pending]);

  const updateQuery = (value: string) => {
    setQuery(value);
    if (value.trim() !== query.trim()) {
      setMatchIndex(0);
    }
    if (!value.trim()) {
      setSearch("");
    }
  };

  const move = (direction: number) => {
    if (!pending && count > 0) {
      setMatchIndex((active + direction + count) % count);
    }
  };

  let status = "";
  if (query.trim() && !pending) {
    const options = {position: active + 1, total: count};
    status = count === 0 ? i18next.t("agent:No transcript matches")
      : limited ? i18next.t("agent:Transcript match limit", options)
        : i18next.t("agent:Transcript match position", options);
  }

  return (
    <>
      <div
        role="search"
        aria-label={searchLabel}
        className="bg-background sticky top-13 z-20 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border p-2"
      >
        <div className="relative min-w-32 flex-1 sm:max-w-xs">
          <Search aria-hidden="true" className="text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4" />
          <Input
            ref={input}
            aria-label={searchLabel}
            placeholder={searchLabel}
            value={query}
            onChange={event => updateQuery(event.target.value)}
            onKeyDown={event => {
              if (event.nativeEvent.isComposing) {
                return;
              }
              if (event.key === "Enter") {
                event.preventDefault();
                if (pending) {
                  setSearch(query.trim());
                } else {
                  move(event.shiftKey ? -1 : 1);
                }
              } else if (event.key === "Escape" && query) {
                event.preventDefault();
                updateQuery("");
              }
            }}
            className="pr-9 pl-8"
          />
          {query ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={clearLabel}
              className="absolute top-1 right-1"
              onClick={() => {
                updateQuery("");
                input.current?.focus();
              }}
            >
              <X />
            </Button>
          ) : null}
        </div>
        <div className="ml-auto flex items-center gap-1 sm:ml-0">
          <SimpleTooltip title={previousLabel}>
            <Button type="button" variant="outline" size="icon-sm" aria-label={previousLabel} disabled={pending || count < 2} onClick={() => move(-1)}>
              <ChevronUp />
            </Button>
          </SimpleTooltip>
          <SimpleTooltip title={nextLabel}>
            <Button type="button" variant="outline" size="icon-sm" aria-label={nextLabel} disabled={pending || count < 2} onClick={() => move(1)}>
              <ChevronDown />
            </Button>
          </SimpleTooltip>
        </div>
        <span role="status" className="text-muted-foreground text-xs tabular-nums max-sm:w-full">{status}</span>
      </div>
      <div ref={content} className="grid gap-3">
        {messages.map((message, index) => (
          <MessageView
            key={index}
            message={message}
            index={index}
            matches={byMessage.get(index)}
            active={messageIndices[active] === index ? active : -1}
            language={i18n.language}
          />
        ))}
      </div>
    </>
  );
}

/**
 * One session, read out of the transcript the agent itself wrote. Monitoring
 * only ever sees what happened after it was turned on, so on the first day this
 * is the only way to open anything on the Sessions page.
 */
export default function AgentSessionPage({account}: {account: Account}) {
  const {sessionKey = ""} = useParams();
  const [searchParams] = useSearchParams();
  const agentId = searchParams.get("agent") ?? "";

  if (!Setting.isAdminUser(account)) {
    return <UnauthorizedResult />;
  }

  return <AgentTranscriptView key={JSON.stringify([agentId, sessionKey])} agentId={agentId} sessionKey={sessionKey} />;
}

/** A blob of what the page already holds, so saving the file never asks the server again. */
function downloadMarkdown(name: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], {type: "text/markdown;charset=utf-8"}));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 0);
}

function AgentTranscriptView({agentId, sessionKey}: {agentId: string; sessionKey: string}) {
  const [transcript, setTranscript] = React.useState<AgentTranscript | null>(null);
  const [error, setError] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [refresh, setRefresh] = React.useState(0);
  React.useEffect(() => {
    if (!sessionKey) return;
    let current = true;
    AgentBackend.getAgentSession(agentId, sessionKey)
      .then(res => {
        if (!current) return;
        if (res.status === "ok") {
          setTranscript(res.data ?? null);
          setError("");
        } else {
          setError(res.msg || i18next.t("agent:Failed to read this transcript"));
        }
      })
      .catch(err => {
        if (current) setError(err.message || String(err));
      })
      .finally(() => {
        if (current) setLoading(false);
      });
    return () => { current = false; };
  }, [agentId, sessionKey, refresh]);

  const reload = () => {
    setLoading(true);
    setRefresh(n => n + 1);
  };

  const session = transcript?.session;
  const messages = transcript?.messages ?? [];
  const command = session && !loading && !error ? sessionResumeCommand(session) : "";

  const ready = !loading && !error ? transcript : null;

  const downloadTranscript = () => {
    if (ready === null) {
      return;
    }

    try {
      downloadMarkdown(transcriptFileName(ready), transcriptMarkdown(ready));
    } catch (failure) {
      Setting.showMessage("error", `${i18next.t("agent:Failed to download the transcript")}: ${String(failure)}`);
    }
  };

  return (
    <PageContainer>
      <PageHeader
        title={session?.title || sessionKey}
        description={command && session?.cwd ? session.path : session?.cwd || session?.path}
        actions={
          <>
            <Button variant="outline" size="sm" asChild>
              <Link to="/agent-sessions">
                <ArrowLeft />
                {i18next.t("agent:Agent Sessions")}
              </Link>
            </Button>
            <Button variant="outline" size="sm" onClick={downloadTranscript} disabled={ready === null}>
              <Download />
              {i18next.t("agent:Download Markdown")}
            </Button>
            <Button variant="outline" size="sm" onClick={reload} loading={loading}>
              <RefreshCw />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      {session ? (
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="info">
            <AgentIcon agent={session.agent} fallback={<Bot className="size-3" />} size={12} />
            {session.agent}
          </Badge>
          {session.archived && <Badge variant="muted">{i18next.t("agent:Archived")}</Badge>}
          <Badge variant="muted">
            {i18next.t("agent:{count} messages").replace("{count}", String(messages.length))}
          </Badge>
          <span className="text-muted-foreground text-xs">{new Date(session.lastTime).toLocaleString()}</span>
        </div>
      ) : null}

      {command ? (
        <section className="space-y-2 rounded-lg border p-4">
          <h2 className="text-sm font-medium">{i18next.t("agent:Resume in a terminal")}</h2>
          <p className="text-muted-foreground text-sm">
            {i18next.t(session?.cwd ? "agent:Resume from the project directory hint" : "agent:Resume from a terminal hint")}
          </p>
          {session?.cwd ? (
            <div className="flex min-w-0 items-center gap-2">
              <code className="text-muted-foreground min-w-0 truncate text-xs" title={session.cwd}>{session.cwd}</code>
              <CopyButton className="shrink-0" value={session.cwd} label={i18next.t("agent:Copy project directory")} />
            </div>
          ) : null}
          <div className="flex items-center gap-2">
            <CodeBlock className="min-w-0 flex-1">{command}</CodeBlock>
            <CopyButton size="icon-sm" value={command} label={i18next.t("agent:Copy resume command")} />
          </div>
        </section>
      ) : null}

      {transcript?.truncated ? <MessageAlert title={i18next.t("agent:Transcript truncated")} /> : null}

      {transcript !== null && messages.length === 0 ? (
        <Card>
          <CardContent className="text-muted-foreground py-8 text-center text-sm">
            {i18next.t("agent:This transcript holds no conversation")}
          </CardContent>
        </Card>
      ) : messages.length > 0 ? (
        <TranscriptMessages messages={messages} />
      ) : null}
    </PageContainer>
  );
}
