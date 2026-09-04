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
import {ChevronLeft, ExternalLink, Plug} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {ProviderIcon} from "@/components/ProviderIcon";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {McpPresetInputs, McpPresetPicker} from "@/components/agent-config/mcp-preset-picker";
import {TargetPicker} from "@/components/agent-config/target-picker";
import {Field} from "@/components/shared/form-dialog";
import {CodeText} from "@/components/shared/misc";
import {SimpleSelect} from "@/components/shared/simple-select";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Input} from "@/components/ui/input";
import {Switch} from "@/components/ui/switch";
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {TagsInput} from "@/components/ui/tags-input";
import {Textarea} from "@/components/ui/textarea";
import {counted} from "@/lib/agent-configs";
import {draftFromPreset, parseMcpJson, presetFilled, type McpDraft, type McpPreset} from "@/lib/mcp";
import {formatPairs, parsePairs} from "@/lib/pairs";
import type {AgentConfigInventory, AgentConfigPlanItem, McpTransport} from "@/types";

/** Which of the three ways of describing a server the dialog is showing. */
type Channel = "preset" | "form" | "json";

const jsonExample = `{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp"]
    }
  }
}`;

/**
 * Setting up one MCP server from Gateway: it is written into every agent picked
 * here, in that agent's own format and file, so the agents that should share a
 * server are configured once instead of one file at a time.
 *
 * The three tabs are the three ways a server is described in the wild - picked
 * from a catalogue, typed out field by field, or pasted as the JSON block its
 * own page hands out - and all of them end up as the same request.
 */
export function AddMcpDialog({
  open,
  onOpenChange,
  inventories,
  source,
  onDone,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  inventories: AgentConfigInventory[];
  source: AgentConfigInventory;
  onDone: () => void;
}) {
  const [channel, setChannel] = React.useState<Channel>("preset");
  const [preset, setPreset] = React.useState<McpPreset | null>(null);
  const [values, setValues] = React.useState<Record<string, string>>({});
  const [json, setJson] = React.useState("");
  const [name, setName] = React.useState("");
  const [transport, setTransport] = React.useState<McpTransport>("stdio");
  const [command, setCommand] = React.useState("");
  const [args, setArgs] = React.useState<string[]>([]);
  const [env, setEnv] = React.useState("");
  const [url, setUrl] = React.useState("");
  const [headers, setHeaders] = React.useState("");
  const [targets, setTargets] = React.useState<string[]>([]);
  const [overwrite, setOverwrite] = React.useState(false);
  const [result, setResult] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [named, setNamed] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  // The server is written into one account's home directory, so the agents
  // offered are the ones reading that same home - which is also what makes one
  // owner enough for the whole request.
  const candidates = inventories.filter(inventory => inventory.home === source.home);
  const defaultTarget = source.mcpWritable ? source.agentId : "";

  React.useEffect(() => {
    if (open) {
      setChannel("preset");
      setPreset(null);
      setValues({});
      setJson("");
      setName("");
      setTransport("stdio");
      setCommand("");
      setArgs([]);
      setEnv("");
      setUrl("");
      setHeaders("");
      setTargets(defaultTarget === "" ? [] : [defaultTarget]);
      setOverwrite(false);
      setResult(null);
      setNamed(false);
      setError("");
    }
  }, [open, defaultTarget]);

  const parsed = React.useMemo(() => parseMcpJson(json), [json]);

  // A pasted block names its server, so the name field follows what was pasted
  // until the block itself changes again.
  const pastedName = parsed.servers.length === 1 ? parsed.servers[0].name : "";
  React.useEffect(() => {
    if (pastedName !== "") {
      setName(pastedName);
    }
  }, [pastedName]);

  const formDraft = (): McpDraft => ({
    name: name.trim(),
    transport: transport,
    command: transport === "stdio" ? command.trim() : "",
    args: transport === "stdio" ? args : [],
    env: transport === "stdio" ? parsePairs(env) : {},
    url: transport === "http" ? url.trim() : "",
    headers: transport === "http" ? parsePairs(headers) : {},
  });

  // What would be written, in the tab the dialog is on. The name typed above
  // wins over the one a preset or a pasted block came with, so a second copy of
  // the same server is a rename rather than an overwrite.
  const drafts = (): McpDraft[] => {
    if (channel === "preset") {
      return preset === null ? [] : [{...draftFromPreset(preset, values), name: name.trim()}];
    }
    if (channel === "json") {
      return parsed.servers.length === 1
        ? [{...parsed.servers[0], name: name.trim()}]
        : parsed.servers.map(server => ({...server, name: server.name.trim()}));
    }
    return [formDraft()];
  };

  const complete = (draft: McpDraft) =>
    draft.name !== "" && (draft.transport === "http" ? draft.url !== "" : draft.command !== "");

  const pending = drafts();
  const ready = targets.length > 0 && pending.length > 0 && pending.every(complete) &&
    (channel !== "preset" || preset === null || presetFilled(preset, values));

  // Picking a preset only fills the fields in; nothing is written until the tab
  // it lands on is submitted, so the command it will run stays reviewable.
  const pick = (picked: McpPreset) => {
    setPreset(picked);
    setValues({});
    setName(picked.key);
    setTransport(picked.transport);
    setError("");
  };

  const fillForm = (draft: McpDraft) => {
    setName(draft.name);
    setTransport(draft.transport);
    setCommand(draft.command);
    setArgs(draft.args);
    setEnv(formatPairs(draft.env));
    setUrl(draft.url);
    setHeaders(formatPairs(draft.headers));
  };

  // Moving to the fields carries whatever the other tab has, so a preset or a
  // pasted block can be adjusted rather than retyped.
  const switchChannel = (next: Channel) => {
    const carried = drafts();
    if (next === "form" && channel !== "form" && carried.length === 1) {
      fillForm(carried[0]);
    }
    setChannel(next);
    setError("");
  };

  const toggleTarget = (agentId: string) => {
    setTargets(previous =>
      previous.includes(agentId) ? previous.filter(item => item !== agentId) : [...previous, agentId],
    );
  };

  const add = () => {
    const sending = drafts();
    setBusy(true);
    setError("");
    setNamed(sending.length > 1);

    // One request per server, in order, so a block naming several of them is
    // still reported server by server and agent by agent.
    sending
      .reduce(
        (chain, draft) =>
          chain.then(items =>
            AgentConfigBackend.addAgentConfigMcp({
              owner: source.owner,
              to: targets,
              name: draft.name,
              transport: draft.transport,
              command: draft.transport === "stdio" ? draft.command : undefined,
              args: draft.transport === "stdio" ? draft.args : undefined,
              env: draft.transport === "stdio" ? draft.env : undefined,
              url: draft.transport === "http" ? draft.url : undefined,
              headers: draft.transport === "http" ? draft.headers : undefined,
              overwrite: overwrite,
            }).then(res => {
              if (res.status !== "ok") {
                throw new Error(
                  res.msg || i18next.t("agentConfig:Failed to add this MCP server"),
                );
              }
              return [...items, ...(res.data ?? [])];
            }),
          ),
        Promise.resolve([] as AgentConfigPlanItem[]),
      )
      .then(written => {
        const done = written.filter(item => item.action === "create" || item.action === "overwrite").length;
        const failed = written.filter(item => item.action === "failed").length;
        setResult(written);
        Setting.showMessage(
          failed > 0 || done === 0 ? "error" : "success",
          failed > 0
            ? i18next
              .t("agentConfig:Added to {done}, {failed} failed")
              .replace("{done}", String(done))
              .replace("{failed}", String(failed))
            : counted(done, "agentConfig:Added to 1 agent", "agentConfig:Added to {done} agents", "{done}"),
        );
        onDone();
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  const nameOf = (agentId: string) =>
    candidates.find(inventory => inventory.agentId === agentId)?.name ?? agentId;

  const jsonErrors: Record<string, string> = {
    notJson: i18next.t("agentConfig:This is not valid JSON"),
    notServers: i18next.t("agentConfig:No MCP server in this JSON"),
    noCommand: i18next.t("agentConfig:A server here has no command or URL"),
  };

  // Browsing the catalogue is a screen of its own: the targets and the button
  // belong to the server that has not been picked yet.
  const browsing = result === null && channel === "preset" && preset === null;
  const single = pending.length === 1 ? pending[0] : null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[85vh] flex-col overflow-hidden sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{i18next.t("agentConfig:Add MCP server")}</DialogTitle>
          <DialogDescription>{i18next.t("agentConfig:Add MCP server hint")}</DialogDescription>
        </DialogHeader>

        {result === null ? (
          <Tabs value={channel} onValueChange={value => switchChannel(value as Channel)}>
            <TabsList className="w-full">
              <TabsTrigger value="preset">{i18next.t("agentConfig:From a template")}</TabsTrigger>
              <TabsTrigger value="form">{i18next.t("agentConfig:Fill in by hand")}</TabsTrigger>
              <TabsTrigger value="json">{i18next.t("agentConfig:Paste JSON")}</TabsTrigger>
            </TabsList>
          </Tabs>
        ) : null}

        <div className="scrollbar-thin -mx-1 flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-1 py-0.5">
          {browsing ? <McpPresetPicker onPick={pick} /> : null}

          {result === null && channel === "preset" && preset !== null ? (
            <>
              <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border p-3">
                <span className="flex min-w-0 items-center gap-2 text-sm font-medium">
                  <ProviderIcon
                    baseUrl={preset.website}
                    size={16}
                    fallback={<Plug className="size-4 shrink-0" />}
                  />
                  {preset.label}
                  <a
                    href={preset.website}
                    target="_blank"
                    rel="noreferrer"
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <ExternalLink className="size-3.5" />
                  </a>
                </span>
                <Button type="button" variant="ghost" size="sm" onClick={() => setPreset(null)}>
                  <ChevronLeft className="size-4" />
                  {i18next.t("agentConfig:Pick another")}
                </Button>
              </div>

              <McpPresetInputs
                preset={preset}
                values={values}
                onChange={(key, value) => setValues(previous => ({...previous, [key]: value}))}
              />

              <Field label={i18next.t("general:Name")} htmlFor="mcp-name" required>
                <Input id="mcp-name" value={name} onChange={event => setName(event.target.value)} />
              </Field>

              {single === null ? null : (
                <Field label={i18next.t("agentConfig:This runs")}>
                  <CodeText>
                    {single.transport === "http"
                      ? single.url
                      : [single.command, ...single.args].join(" ")}
                  </CodeText>
                </Field>
              )}
            </>
          ) : null}

          {result === null && channel === "json" ? (
            <>
              <Field
                label={i18next.t("agentConfig:Configuration JSON")}
                htmlFor="mcp-json"
                hint={i18next.t("agentConfig:Paste JSON hint")}
                error={jsonErrors[parsed.error]}
              >
                <Textarea
                  id="mcp-json"
                  value={json}
                  rows={10}
                  spellCheck={false}
                  placeholder={jsonExample}
                  className="font-mono text-xs"
                  onChange={event => setJson(event.target.value)}
                />
              </Field>

              {parsed.servers.length === 1 ? (
                <Field label={i18next.t("general:Name")} htmlFor="mcp-name" required>
                  <Input id="mcp-name" value={name} onChange={event => setName(event.target.value)} />
                </Field>
              ) : null}

              {parsed.servers.length > 1 ? (
                <Field label={i18next.t("agentConfig:Servers in this JSON")}>
                  <ul className="divide-y rounded-md border">
                    {parsed.servers.map(server => (
                      <li key={server.name} className="flex items-center gap-3 px-3 py-1.5 text-sm">
                        <span className="shrink-0 font-medium">{server.name}</span>
                        <span className="text-muted-foreground truncate font-mono text-xs">
                          {server.transport === "http" ? server.url : [server.command, ...server.args].join(" ")}
                        </span>
                      </li>
                    ))}
                  </ul>
                </Field>
              ) : null}
            </>
          ) : null}

          {result === null && channel === "form" ? (
            <>
              <Field label={i18next.t("general:Name")} htmlFor="mcp-name" required>
                <Input
                  id="mcp-name"
                  value={name}
                  placeholder="context7"
                  onChange={event => setName(event.target.value)}
                />
              </Field>

              <Field label={i18next.t("agentConfig:Transport")}>
                <SimpleSelect
                  value={transport}
                  options={[
                    {label: i18next.t("agentConfig:Local command"), value: "stdio"},
                    {label: i18next.t("agentConfig:HTTP endpoint"), value: "http"},
                  ]}
                  onChange={value => setTransport(value as McpTransport)}
                />
              </Field>

              {transport === "stdio" ? (
                <>
                  <Field
                    label={i18next.t("agentConfig:Command")}
                    htmlFor="mcp-command"
                    hint={i18next.t("agentConfig:Command hint")}
                    required
                  >
                    <Input
                      id="mcp-command"
                      value={command}
                      placeholder="npx"
                      onChange={event => setCommand(event.target.value)}
                    />
                  </Field>
                  <Field label={i18next.t("agentConfig:Arguments")}>
                    <TagsInput value={args} placeholder="-y, @upstash/context7-mcp" onChange={setArgs} />
                  </Field>
                  <Field
                    label={i18next.t("agentConfig:Environment")}
                    htmlFor="mcp-env"
                    hint={i18next.t("agentConfig:Pairs hint")}
                  >
                    <Textarea
                      id="mcp-env"
                      value={env}
                      rows={2}
                      placeholder="API_KEY=..."
                      onChange={event => setEnv(event.target.value)}
                    />
                  </Field>
                </>
              ) : (
                <>
                  <Field label={i18next.t("agentConfig:URL")} htmlFor="mcp-url" required>
                    <Input
                      id="mcp-url"
                      value={url}
                      placeholder="https://mcp.example.com/mcp"
                      onChange={event => setUrl(event.target.value)}
                    />
                  </Field>
                  <Field
                    label={i18next.t("agentConfig:Headers")}
                    htmlFor="mcp-headers"
                    hint={i18next.t("agentConfig:Pairs hint")}
                  >
                    <Textarea
                      id="mcp-headers"
                      value={headers}
                      rows={2}
                      placeholder="Authorization=Bearer ..."
                      onChange={event => setHeaders(event.target.value)}
                    />
                  </Field>
                </>
              )}
            </>
          ) : null}

          {browsing ? null : (
            <Field label={i18next.t("agentConfig:Add to")} hint={i18next.t("agentConfig:Add to hint")}>
              <TargetPicker
                candidates={candidates}
                kind="mcp"
                selected={targets}
                onToggle={toggleTarget}
                disabled={result !== null}
              />
            </Field>
          )}

          {result === null && !browsing ? (
            <label className="flex items-center gap-2 text-sm">
              <Switch checked={overwrite} onCheckedChange={setOverwrite} />
              <span>{i18next.t("agentConfig:Replace items that already exist")}</span>
            </label>
          ) : null}

          {error ? <MessageAlert description={error} /> : null}

          {result === null ? null : (
            <ul className="divide-y rounded-md border">
              {result.map(item => (
                <li
                  key={`${item.agentId}/${item.name}`}
                  className="flex items-center justify-between gap-3 px-3 py-1.5 text-sm"
                >
                  <span className="truncate">
                    {nameOf(item.agentId)}
                    {named ? <span className="text-muted-foreground"> · {item.name}</span> : null}
                  </span>
                  <span className="flex shrink-0 items-center gap-2">
                    {item.reason ? <span className="text-muted-foreground text-xs">{item.reason}</span> : null}
                    <ActionBadge action={item.action} />
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {result === null ? i18next.t("general:Cancel") : i18next.t("general:Close")}
          </Button>
          {result === null && !browsing ? (
            <Button onClick={add} disabled={busy || !ready} loading={busy}>
              {pending.length > 1
                ? i18next.t("agentConfig:Add {count} servers").replace("{count}", String(pending.length))
                : i18next.t("agentConfig:Add MCP server")}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
