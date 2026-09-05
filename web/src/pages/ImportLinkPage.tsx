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
import {useNavigate} from "react-router-dom";
import {Link2} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as ImportLinkBackend from "@/backend/ImportLinkBackend";
import * as Setting from "@/Setting";
import {CcSwitchImportCard} from "@/components/CcSwitchImportCard";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {TargetPicker} from "@/components/agent-config/target-picker";
import {CodeBlock, CodeText, DescriptionList} from "@/components/shared/misc";
import {Loading} from "@/components/shared/loading";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import {Textarea} from "@/components/ui/textarea";
import {connectorText} from "@/lib/connectors";
import {parseMcpJson} from "@/lib/mcp";
import type {
  AgentConfigInventory,
  AgentConfigPlanItem,
  ImportLink,
  ConnectionImport,
  McpImport,
  PromptImport,
  SkillImport,
} from "@/types";

/**
 * The two ways settings arrive from somewhere else: a whole CC Switch
 * installation on this machine, brought over in one go, and a vendor's "add
 * this to Gateway" link. Both are shown before any of them is kept — a link
 * arrives from a website, clicked there and routed here by the URL scheme
 * handler or pasted in below — so nothing on this page is written until the
 * button under it is pressed.
 *
 * A provider from a link goes on to the provider page's own form, which is
 * where one is reviewed and tested. The other three are written from here,
 * through the same endpoints that add one by hand.
 */
export default function ImportLinkPage() {
  const navigate = useNavigate();
  const [link, setLink] = React.useState<ImportLink | null>(null);
  const [pasted, setPasted] = React.useState("");
  const [loading, setLoading] = React.useState(true);
  const [reading, setReading] = React.useState(false);
  const [error, setError] = React.useState("");
  const taken = React.useRef(false);

  // The waiting link is handed over once and then forgotten, so it is asked for
  // once however many times the effect runs.
  React.useEffect(() => {
    if (taken.current) {
      return;
    }
    taken.current = true;

    ImportLinkBackend.getPendingImportLink()
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("link:This link cannot be read"));
          return;
        }
        if (res.data) {
          setLink(res.data);
        }
      })
      .catch(err => setError(`${err}`))
      .then(() => setLoading(false));
  }, []);

  // A provider is added on the provider page: its form is what shows the base
  // URL and the key, and what tests them against the upstream before storing.
  React.useEffect(() => {
    if (link?.provider) {
      navigate("/providers", {replace: true, state: {importProvider: link.provider}});
    }
  }, [link, navigate]);

  const read = () => {
    const raw = pasted.trim();
    if (raw === "") {
      return;
    }
    setReading(true);
    setError("");
    ImportLinkBackend.parseImportLink(raw)
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("link:This link cannot be read"));
          return;
        }
        setLink(res.data);
      })
      .catch(err => setError(`${err}`))
      .then(() => setReading(false));
  };

  const discard = () => {
    setLink(null);
    setPasted("");
    setError("");
  };

  return (
    <PageContainer>
      <PageHeader title={i18next.t("link:Import settings")} description={i18next.t("link:Import hint")} />

      {error === "" ? null : <MessageAlert title={error} />}

      {loading ? (
        <Loading />
      ) : link === null ? (
        <>
          <CcSwitchImportCard />
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-[15px]">
                <Link2 className="size-4" />
                {i18next.t("link:Paste a link")}
              </CardTitle>
              <CardDescription>{i18next.t("link:Paste a link hint")}</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <Textarea
                value={pasted}
                onChange={event => setPasted(event.target.value)}
                placeholder="ccswitch://v1/import?resource=..."
                rows={3}
                className="font-mono text-xs"
              />
              <div>
                <Button onClick={read} disabled={reading || pasted.trim() === ""}>
                  {i18next.t("link:Read link")}
                </Button>
              </div>
            </CardContent>
          </Card>
        </>
      ) : link.mcp ? (
        <McpImportCard value={link.mcp} onDone={discard} />
      ) : link.prompt ? (
        <PromptImportCard value={link.prompt} onDone={discard} />
      ) : link.skill ? (
        <SkillImportCard value={link.skill} onDone={discard} />
      ) : link.connection ? (
        <ConnectionImportCard value={link.connection} onDone={discard} />
      ) : null}
    </PageContainer>
  );
}

/** The agents on this host, for the two resources that are written into them. */
function useInventories() {
  const [inventories, setInventories] = React.useState<AgentConfigInventory[]>([]);
  const [loading, setLoading] = React.useState(true);

  React.useEffect(() => {
    AgentConfigBackend.getAgentConfigs()
      .then(res => setInventories(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setInventories([]))
      .then(() => setLoading(false));
  }, []);

  return {inventories: inventories, loading: loading};
}

/** The apps a link named that Gateway does not manage, said once. */
function UnknownApps({apps}: {apps: string[]}) {
  if (apps.length === 0) {
    return null;
  }
  return (
    <MessageAlert
      variant="warning"
      title={i18next.t("link:Some apps are not managed here")}
      description={i18next.t("link:Some apps are not managed here hint").replace("{apps}", apps.join(", "))}
    />
  );
}

/** What a write did, agent by agent. */
function PlanResult({items}: {items: AgentConfigPlanItem[]}) {
  return (
    <div className="flex flex-col gap-2">
      {items.map((item, index) => (
        <div key={`${item.agentId}-${item.name}-${index}`} className="flex items-center gap-2 text-sm">
          <ActionBadge action={item.action} />
          <span className="truncate">{item.agentId}</span>
          {item.reason ? <span className="text-muted-foreground truncate text-xs">{item.reason}</span> : null}
        </div>
      ))}
    </div>
  );
}

function McpImportCard({value, onDone}: {value: McpImport; onDone: () => void}) {
  const {inventories, loading} = useInventories();
  const [targets, setTargets] = React.useState<string[]>([]);
  const [overwrite, setOverwrite] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [result, setResult] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [error, setError] = React.useState("");

  const parsed = React.useMemo(() => parseMcpJson(value.config), [value.config]);

  // A server is written into one account's home directory, so the agents
  // offered are the ones reading the same home as the first the link asked for.
  const asked = inventories.find(inventory => value.targets.includes(inventory.agentId));
  const source = asked ?? inventories[0];
  const candidates = source ? inventories.filter(inventory => inventory.home === source.home) : [];

  React.useEffect(() => {
    setTargets(candidates.filter(inventory => value.targets.includes(inventory.agentId)).map(item => item.agentId));
    // The candidates are derived from the inventories, which is what changing
    // here really means.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [inventories, value.targets]);

  const toggle = (agentId: string) => {
    setTargets(previous =>
      previous.includes(agentId) ? previous.filter(item => item !== agentId) : [...previous, agentId],
    );
  };

  const add = () => {
    setBusy(true);
    setError("");

    // One request per server, in order, so a block naming several of them is
    // reported server by server and agent by agent.
    parsed.servers
      .reduce(
        (chain, server) =>
          chain.then(items =>
            AgentConfigBackend.addAgentConfigMcp({
              owner: source?.owner ?? "",
              to: targets,
              name: server.name.trim() === "" ? value.name : server.name.trim(),
              transport: server.transport,
              command: server.transport === "stdio" ? server.command : undefined,
              args: server.transport === "stdio" ? server.args : undefined,
              env: server.transport === "stdio" ? server.env : undefined,
              url: server.transport === "http" ? server.url : undefined,
              headers: server.transport === "http" ? server.headers : undefined,
              overwrite: overwrite,
            }).then(res => {
              if (res.status !== "ok") {
                throw new Error(res.msg || i18next.t("agentConfig:Failed to add this MCP server"));
              }
              return [...items, ...(res.data ?? [])];
            }),
          ),
        Promise.resolve([] as AgentConfigPlanItem[]),
      )
      .then(written => {
        setResult(written);
        const failed = written.filter(item => item.action === "failed").length;
        Setting.showMessage(
          failed > 0 ? "error" : "success",
          failed > 0 ? i18next.t("link:Import finished with failures") : i18next.t("link:Imported"),
        );
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  const names = parsed.servers.map(server => (server.name === "" ? value.name : server.name)).filter(Boolean);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-[15px]">{i18next.t("link:MCP servers from this link")}</CardTitle>
        <CardDescription>{i18next.t("link:MCP servers hint")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {parsed.error === "" ? null : <MessageAlert title={i18next.t("link:This link cannot be read")} />}
        {error === "" ? null : <MessageAlert title={error} />}
        <UnknownApps apps={value.unknown} />

        <DescriptionList
          items={[
            {label: i18next.t("general:Name"), value: <CodeText>{names.join(", ")}</CodeText>},
          ]}
        />
        <CodeBlock maxHeight="16rem">{value.config}</CodeBlock>

        {result === null ? (
          <>
            <div className="flex flex-col gap-2">
              <Label>{i18next.t("agentConfig:Add to")}</Label>
              {loading ? (
                <Loading />
              ) : (
                <TargetPicker
                  candidates={candidates}
                  kind="mcp"
                  selected={targets}
                  onToggle={toggle}
                  disabled={busy}
                />
              )}
            </div>
            <div className="flex items-center gap-2">
              <Switch id="mcp-overwrite" checked={overwrite} onCheckedChange={setOverwrite} disabled={busy} />
              <Label htmlFor="mcp-overwrite">{i18next.t("agentConfig:Replace items that already exist")}</Label>
            </div>
            <div className="flex gap-2">
              <Button onClick={add} disabled={busy || targets.length === 0 || parsed.servers.length === 0}>
                {i18next.t("link:Import")}
              </Button>
              <Button variant="outline" onClick={onDone} disabled={busy}>
                {i18next.t("link:Discard")}
              </Button>
            </div>
          </>
        ) : (
          <>
            <PlanResult items={result} />
            <div>
              <Button variant="outline" onClick={onDone}>
                {i18next.t("link:Done")}
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

function PromptImportCard({value, onDone}: {value: PromptImport; onDone: () => void}) {
  const {inventories, loading} = useInventories();
  const [targets, setTargets] = React.useState<string[]>([]);
  const [busy, setBusy] = React.useState(false);
  const [result, setResult] = React.useState<AgentConfigPlanItem[] | null>(null);
  const [error, setError] = React.useState("");

  React.useEffect(() => {
    setTargets(inventories.filter(item => value.targets.includes(item.agentId)).map(item => item.agentId));
  }, [inventories, value.targets]);

  const toggle = (agentId: string) => {
    setTargets(previous =>
      previous.includes(agentId) ? previous.filter(item => item !== agentId) : [...previous, agentId],
    );
  };

  const write = () => {
    setBusy(true);
    setError("");

    const picked = inventories.filter(inventory => targets.includes(inventory.agentId));
    picked
      .reduce(
        (chain, inventory) =>
          chain.then(items =>
            AgentConfigBackend.saveAgentConfigPrompt(inventory.agentId, inventory.owner, value.content).then(res => [
              ...items,
              {
                agentId: inventory.agentId,
                name: value.name,
                action: res.status === "ok" ? "overwrite" : "failed",
                reason: res.status === "ok" ? "" : res.msg,
              } as AgentConfigPlanItem,
            ]),
          ),
        Promise.resolve([] as AgentConfigPlanItem[]),
      )
      .then(written => {
        setResult(written);
        const failed = written.filter(item => item.action === "failed").length;
        Setting.showMessage(
          failed > 0 ? "error" : "success",
          failed > 0 ? i18next.t("link:Import finished with failures") : i18next.t("link:Imported"),
        );
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setBusy(false));
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-[15px]">{value.name}</CardTitle>
        <CardDescription>
          {value.description === "" ? i18next.t("link:Instructions hint") : value.description}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {error === "" ? null : <MessageAlert title={error} />}
        <UnknownApps apps={value.unknown} />
        <MessageAlert
          variant="warning"
          title={i18next.t("link:This replaces the instructions")}
          description={i18next.t("link:This replaces the instructions hint")}
        />

        <CodeBlock maxHeight="20rem">{value.content}</CodeBlock>

        {result === null ? (
          <>
            <div className="flex flex-col gap-2">
              <Label>{i18next.t("agentConfig:Add to")}</Label>
              {loading ? (
                <Loading />
              ) : (
                <TargetPicker
                  candidates={inventories}
                  kind="prompt"
                  selected={targets}
                  onToggle={toggle}
                  disabled={busy}
                />
              )}
            </div>
            <div className="flex gap-2">
              <Button onClick={write} disabled={busy || targets.length === 0}>
                {i18next.t("link:Import")}
              </Button>
              <Button variant="outline" onClick={onDone} disabled={busy}>
                {i18next.t("link:Discard")}
              </Button>
            </div>
          </>
        ) : (
          <>
            <PlanResult items={result} />
            <div>
              <Button variant="outline" onClick={onDone}>
                {i18next.t("link:Done")}
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}

function SkillImportCard({value, onDone}: {value: SkillImport; onDone: () => void}) {
  const navigate = useNavigate();
  const {inventories} = useInventories();
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState("");

  // Sources belong to the account whose home the agents read, which is the one
  // the listing is of - not the account signed in to Gateway.
  const owner = inventories[0]?.owner ?? "";

  const add = () => {
    setBusy(true);
    setError("");
    AgentConfigBackend.addSkillSource({
      owner: owner,
      kind: "github",
      url: value.repo,
      ref: value.ref,
      subdir: value.subdir,
      name: value.name,
    })
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("agentConfig:Failed to add this source"));
          return;
        }
        Setting.showMessage("success", i18next.t("agentConfig:Source added"));
        onDone();
        navigate("/agent-configs?tab=skill");
      })
      .catch(err => setError(`${err}`))
      .then(() => setBusy(false));
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-[15px]">{i18next.t("link:A place to install skills from")}</CardTitle>
        <CardDescription>{i18next.t("link:Skill source hint")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {error === "" ? null : <MessageAlert title={error} />}

        <DescriptionList
          items={[
            {label: i18next.t("link:Repository"), value: <CodeText copyable>{value.repo}</CodeText>},
            value.ref !== "" && {label: i18next.t("link:Branch"), value: <CodeText>{value.ref}</CodeText>},
            value.subdir !== "" && {
              label: i18next.t("link:Folder"),
              value: <CodeText>{value.subdir}</CodeText>,
            },
          ]}
        />

        <div className="flex gap-2">
          <Button onClick={add} disabled={busy}>
            {i18next.t("link:Import")}
          </Button>
          <Button variant="outline" onClick={onDone} disabled={busy}>
            {i18next.t("link:Discard")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

/**
 * One application a link offers to connect. Nothing is written from here: the
 * link carries no credential, so the only thing to do with it is open the
 * dialog on the Connections page with this application already picked.
 */
function ConnectionImportCard({value, onDone}: {value: ConnectionImport; onDone: () => void}) {
  const navigate = useNavigate();

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-[15px]">{i18next.t("link:An application to connect")}</CardTitle>
        <CardDescription>{i18next.t("link:Connection hint")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="space-y-1">
          <div className="font-medium">{connectorText(value.displayName)}</div>
          <p className="text-muted-foreground text-sm">{connectorText(value.description)}</p>
        </div>
        <div className="flex justify-end">
          <Button
            onClick={() => {
              onDone();
              navigate(`/connections?connect=${encodeURIComponent(value.connector)}`);
            }}
          >
            {i18next.t("connector:Connect")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
