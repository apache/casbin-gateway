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
import {ChevronDown, ChevronRight, Database, Image, Wrench} from "lucide-react";
import i18next from "i18next";

import {CodeBlock, CodeText, CopyButton} from "@/components/shared/misc";
import {Badge, type BadgeVariant} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Tabs, TabsContent, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {cn} from "@/lib/utils";
import {
  parseRequest,
  parseResponse,
  type ParsedRequest,
  type RequestBlock,
  type RequestMessage,
  type ToolSchema,
} from "@/lib/llm-request";

const COLLAPSED_CHARS = 1200;

function formatChars(chars: number) {
  if (chars < 1000) {
    return `${chars} ${i18next.t("llm:chars")}`;
  }
  return `${(chars / 1000).toFixed(1)}k ${i18next.t("llm:chars")}`;
}

function formatJson(value: unknown) {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

const ROLE_VARIANTS: Record<string, BadgeVariant> = {
  user: "info",
  assistant: "success",
  system: "warning",
  developer: "warning",
  tool: "muted",
};

/** Text that opens up when it is longer than a screenful. */
function ExpandableText({text, className}: {text: string; className?: string}) {
  const [expanded, setExpanded] = React.useState(false);
  const isLong = text.length > COLLAPSED_CHARS;
  const shown = expanded || !isLong ? text : text.slice(0, COLLAPSED_CHARS);

  return (
    <div className="relative">
      <pre className={cn("scrollbar-thin overflow-x-auto text-xs leading-relaxed whitespace-pre-wrap", className)}>
        {shown}
        {isLong && !expanded ? <span className="text-muted-foreground">…</span> : null}
      </pre>
      {isLong ? (
        <Button size="sm" variant="ghost" className="mt-1 h-6 px-2" onClick={() => setExpanded(!expanded)}>
          {expanded
            ? i18next.t("llm:Show less")
            : i18next.t("llm:Show all {chars}").replace("{chars}", formatChars(text.length))}
        </Button>
      ) : null}
    </div>
  );
}

function CacheBadge() {
  return (
    <Badge variant="info" title={i18next.t("llm:Cache breakpoint detail")}>
      <Database />
      {i18next.t("llm:Cache breakpoint")}
    </Badge>
  );
}

function BlockView({block}: {block: RequestBlock}) {
  switch (block.kind) {
  case "text":
    return <ExpandableText text={block.text ?? ""} />;
  case "thinking":
    return (
      <div className="grid gap-1">
        <Badge variant="muted">{i18next.t("llm:Thinking")}</Badge>
        <ExpandableText className="text-muted-foreground italic" text={block.text ?? ""} />
      </div>
    );
  case "image":
    return (
      <Badge variant="muted">
        <Image />
        {block.mediaType} · {formatChars(block.chars)}
      </Badge>
    );
  case "toolUse":
    return (
      <div className="grid gap-1">
        <span className="flex items-center gap-2">
          <Badge variant="warning">
            <Wrench />
            {block.toolName}
          </Badge>
          {block.toolUseId ? <span className="text-muted-foreground font-mono text-xs">{block.toolUseId}</span> : null}
        </span>
        <CodeBlock copyable maxHeight="16rem">
          {formatJson(block.data)}
        </CodeBlock>
      </div>
    );
  case "toolResult":
    return (
      <div className="grid gap-1">
        <span className="flex items-center gap-2">
          <Badge variant={block.isError ? "danger" : "muted"}>
            {block.isError ? i18next.t("llm:Tool error") : i18next.t("llm:Tool result")}
          </Badge>
          {block.toolUseId ? <span className="text-muted-foreground font-mono text-xs">{block.toolUseId}</span> : null}
        </span>
        {block.text ? <ExpandableText text={block.text} /> : <CodeBlock maxHeight="16rem">{formatJson(block.data)}</CodeBlock>}
      </div>
    );
  default:
    return <CodeBlock maxHeight="16rem">{formatJson(block.data)}</CodeBlock>;
  }
}

function MessageView({message}: {message: RequestMessage}) {
  return (
    <div className="rounded-lg border">
      <div className="bg-muted/40 flex flex-wrap items-center gap-2 border-b px-3 py-1.5">
        <Badge variant={ROLE_VARIANTS[message.role] ?? "secondary"}>{message.role || i18next.t("llm:Unknown role")}</Badge>
        <span className="text-muted-foreground text-xs">#{message.index + 1}</span>
        <span className="text-muted-foreground text-xs">{formatChars(message.chars)}</span>
        {message.blocks.some(block => block.cached) ? <CacheBadge /> : null}
      </div>
      <div className="grid gap-3 px-3 py-2">
        {message.blocks.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("llm:Empty message")}</span>
        ) : (
          message.blocks.map((block, index) => <BlockView key={index} block={block} />)
        )}
      </div>
    </div>
  );
}

function ToolView({tool}: {tool: ToolSchema}) {
  const [expanded, setExpanded] = React.useState(false);

  return (
    <div className="rounded-lg border">
      <button
        type="button"
        className="hover:bg-muted/40 flex w-full items-center gap-2 px-3 py-2 text-left"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ChevronDown className="size-4 shrink-0" /> : <ChevronRight className="size-4 shrink-0" />}
        <code className="font-mono text-xs font-medium">{tool.name}</code>
        <span className="text-muted-foreground min-w-0 flex-1 truncate text-xs">{tool.description}</span>
        {tool.cached ? <CacheBadge /> : null}
        <Badge variant="muted">
          {tool.parameters.length} {i18next.t("llm:params")}
        </Badge>
        <span className="text-muted-foreground text-xs">{formatChars(tool.chars)}</span>
      </button>

      {expanded ? (
        <div className="grid gap-3 border-t px-3 py-2">
          {tool.description ? <ExpandableText className="text-muted-foreground" text={tool.description} /> : null}
          {tool.parameters.length > 0 ? (
            <table className="w-full text-xs">
              <tbody>
                {tool.parameters.map(parameter => (
                  <tr key={parameter.name} className="border-b last:border-0">
                    <td className="py-1 pr-3 align-top font-mono whitespace-nowrap">{parameter.name}</td>
                    <td className="text-muted-foreground py-1 pr-3 align-top whitespace-nowrap">{parameter.type}</td>
                    <td className="py-1 pr-3 align-top whitespace-nowrap">
                      {parameter.required ? <Badge variant="warning">{i18next.t("llm:required")}</Badge> : null}
                    </td>
                    <td className="text-muted-foreground py-1 align-top">{parameter.description}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : null}
          <CodeBlock copyable maxHeight="20rem">
            {formatJson(tool.schema)}
          </CodeBlock>
        </div>
      ) : null}
    </div>
  );
}

function SystemView({request}: {request: ParsedRequest}) {
  if (request.system.length === 0) {
    return <span className="text-muted-foreground text-xs">{i18next.t("llm:No system prompt")}</span>;
  }

  return (
    <div className="grid gap-3">
      <div className="flex items-center gap-2">
        <span className="text-muted-foreground text-xs">
          {request.system.length} {i18next.t("llm:blocks")} · {formatChars(request.systemChars)}
        </span>
        <CopyButton value={request.system.map(block => block.text ?? "").join("\n\n")} />
      </div>
      {request.system.map((block, index) => (
        <div key={index} className="rounded-lg border px-3 py-2">
          {block.cached ? (
            <div className="mb-1">
              <CacheBadge />
            </div>
          ) : null}
          <BlockView block={block} />
        </div>
      ))}
    </div>
  );
}

function ResponseView({response}: {response: string}) {
  const parsed = React.useMemo(() => parseResponse(response), [response]);

  if (!response) {
    return <span className="text-muted-foreground text-xs">{i18next.t("llm:No response stored")}</span>;
  }
  if (parsed.invalid || !parsed.message) {
    return (
      <div className="grid gap-2">
        <span className="text-muted-foreground text-xs">{i18next.t("llm:Body is not JSON")}</span>
        <CodeBlock copyable maxHeight="24rem">
          {parsed.raw}
        </CodeBlock>
      </div>
    );
  }

  return (
    <div className="grid gap-2">
      <div className="flex items-center gap-2">
        {parsed.stopReason ? (
          <span className="text-muted-foreground text-xs">
            {i18next.t("llm:Stop reason")}: <CodeText>{parsed.stopReason}</CodeText>
          </span>
        ) : null}
        <CopyButton value={parsed.raw} />
      </div>
      <MessageView message={parsed.message} />
    </div>
  );
}

/**
 * Everything the client sent this turn: the system prompt, every message in
 * order, and the schema of every tool the model was offered. The answer it got
 * back has a tab of its own.
 */
export function RequestInspector({payload, response = ""}: {payload: string; response?: string}) {
  const request = React.useMemo(() => parseRequest(payload), [payload]);
  const [tab, setTab] = React.useState("messages");

  if (request.invalid) {
    return (
      <div className="grid gap-2">
        <span className="text-muted-foreground text-xs">{i18next.t("llm:Body is not JSON")}</span>
        <CodeBlock copyable maxHeight="24rem">
          {request.raw}
        </CodeBlock>
      </div>
    );
  }

  return (
    <Tabs value={tab} onValueChange={setTab} className="gap-3">
      <TabsList>
        <TabsTrigger value="messages">
          {i18next.t("llm:Messages")}
          <Badge variant="muted">{request.messages.length}</Badge>
        </TabsTrigger>
        <TabsTrigger value="response">{i18next.t("llm:Response")}</TabsTrigger>
        <TabsTrigger value="system">
          {i18next.t("llm:System prompt")}
          {request.systemChars > 0 ? <Badge variant="muted">{formatChars(request.systemChars)}</Badge> : null}
        </TabsTrigger>
        <TabsTrigger value="tools">
          {i18next.t("llm:Tools")}
          <Badge variant="muted">{request.tools.length}</Badge>
        </TabsTrigger>
        <TabsTrigger value="params">{i18next.t("llm:Parameters")}</TabsTrigger>
        <TabsTrigger value="raw">{i18next.t("llm:Raw")}</TabsTrigger>
      </TabsList>

      <TabsContent value="messages" className="grid gap-2">
        {request.messages.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("llm:No messages")}</span>
        ) : (
          request.messages.map(message => <MessageView key={message.index} message={message} />)
        )}
      </TabsContent>

      <TabsContent value="response">
        <ResponseView response={response} />
      </TabsContent>

      <TabsContent value="system">
        <SystemView request={request} />
      </TabsContent>

      <TabsContent value="tools" className="grid gap-2">
        {request.tools.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("llm:No tools")}</span>
        ) : (
          request.tools.map(tool => <ToolView key={tool.name} tool={tool} />)
        )}
      </TabsContent>

      <TabsContent value="params">
        {request.params.length === 0 ? (
          <span className="text-muted-foreground text-xs">{i18next.t("llm:No parameters")}</span>
        ) : (
          <table className="w-full text-sm">
            <tbody>
              {request.params.map(parameter => (
                <tr key={parameter.key} className="border-b last:border-0">
                  <td className="text-muted-foreground w-[220px] py-1.5 pr-3 align-top text-xs">{parameter.key}</td>
                  <td className="py-1.5 align-top break-all">
                    <CodeText>{parameter.value}</CodeText>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </TabsContent>

      <TabsContent value="raw">
        <CodeBlock copyable maxHeight="30rem">
          {request.raw}
        </CodeBlock>
      </TabsContent>
    </Tabs>
  );
}
