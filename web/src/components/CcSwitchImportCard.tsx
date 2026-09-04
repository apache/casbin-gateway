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
import {ArrowDownToLine, Check, ChevronDown, FolderSearch} from "lucide-react";
import i18next from "i18next";

import * as CcSwitchBackend from "@/backend/CcSwitchBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {ProviderIcon} from "@/components/ProviderIcon";
import {ActionBadge} from "@/components/agent-config/action-badge";
import {CodeText} from "@/components/shared/misc";
import {Loading} from "@/components/shared/loading";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";
import {Label} from "@/components/ui/label";
import {Switch} from "@/components/ui/switch";
import {cn} from "@/lib/utils";
import type {CcSwitchImport, CcSwitchOutcome, CcSwitchResult} from "@/types";

/** The four kinds of entry, which are also the fields of a selection. */
const kinds = ["providers", "mcps", "prompts", "skills"] as const;
type Kind = (typeof kinds)[number];

type Selection = Record<Kind, string[]>;

const emptySelection = (): Selection => ({providers: [], mcps: [], prompts: [], skills: []});

/**
 * What is ticked when the list first appears. Everything is, except the
 * instructions: those are a single file per agent, and importing one writes
 * over whatever that agent reads today.
 */
function defaultSelection(found: CcSwitchImport): Selection {
  return {
    providers: found.providers.map(item => item.key),
    mcps: found.mcps.filter(item => item.targets.length > 0).map(item => item.key),
    prompts: [],
    skills: found.skills.map(item => item.key),
  };
}

function kindTitle(kind: Kind) {
  return {
    providers: i18next.t("link:Providers"),
    mcps: i18next.t("link:MCP servers"),
    prompts: i18next.t("link:Instructions"),
    skills: i18next.t("link:Skill sources"),
  }[kind];
}

/** One tickable entry: a row that reads as itself, and a tick that follows it. */
function Row({
  selected,
  onToggle,
  icon,
  title,
  detail,
  badges,
  disabled = false,
}: {
  selected: boolean;
  onToggle: () => void;
  icon: React.ReactNode;
  title: React.ReactNode;
  detail?: React.ReactNode;
  badges?: React.ReactNode;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onToggle}
      className={cn(
        "flex w-full items-center gap-3 rounded-md border px-3 py-2 text-left transition-colors",
        selected ? "border-primary bg-primary/10" : "hover:bg-accent",
        disabled && "cursor-not-allowed opacity-50",
      )}
    >
      <span
        className={cn(
          "flex size-4 shrink-0 items-center justify-center rounded-[4px] border",
          selected ? "border-primary bg-primary text-primary-foreground" : "border-input",
        )}
      >
        {selected ? <Check className="size-3" /> : null}
      </span>
      <span className="shrink-0">{icon}</span>
      <span className="min-w-0 flex-1">
        <span className="flex flex-wrap items-center gap-2 text-sm font-medium">
          {title}
          {badges}
        </span>
        {detail ? <span className="text-muted-foreground block truncate text-xs">{detail}</span> : null}
      </span>
    </button>
  );
}

/** The agents an entry lands on, named by their own marks. */
function Targets({targets}: {targets: string[]}) {
  return (
    <>
      {targets.map(agentId => (
        <span key={agentId} className="text-muted-foreground inline-flex items-center gap-1 text-xs font-normal">
          <AgentIcon agent={agentId} size={12} />
          {agentId}
        </span>
      ))}
    </>
  );
}

function Section({title, count, children}: {title: string; count: number; children: React.ReactNode}) {
  if (count === 0) {
    return null;
  }
  return (
    <div className="grid gap-2">
      <Label className="text-muted-foreground text-xs font-medium uppercase tracking-wide">
        {title} · {count}
      </Label>
      <div className="grid gap-2">{children}</div>
    </div>
  );
}

/** What the import did, entry by entry, once it has run. */
function Outcomes({title, items}: {title: string; items: CcSwitchOutcome[]}) {
  if (items.length === 0) {
    return null;
  }
  return (
    <div className="grid gap-2">
      <Label className="text-muted-foreground text-xs font-medium uppercase tracking-wide">{title}</Label>
      {items.map((item, index) => (
        <div key={`${item.key}-${item.agent ?? ""}-${index}`} className="flex flex-wrap items-center gap-2 text-sm">
          <ActionBadge action={item.action} />
          <span className="truncate">{item.name}</span>
          {item.agent ? <span className="text-muted-foreground text-xs">{item.agent}</span> : null}
          {item.reason ? <span className="text-muted-foreground truncate text-xs">{item.reason}</span> : null}
        </div>
      ))}
    </div>
  );
}

/** The entries there was nothing to bring over from, folded away until asked for. */
function Skipped({found}: {found: CcSwitchImport}) {
  const [open, setOpen] = React.useState(false);

  if (found.skipped.length === 0) {
    return null;
  }
  return (
    <div className="grid gap-2">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="text-muted-foreground hover:text-foreground flex w-fit items-center gap-1 text-xs"
      >
        <ChevronDown className={cn("size-3.5 transition-transform", open && "rotate-180")} />
        {i18next.t("link:Left out").replace("{count}", `${found.skipped.length}`)}
      </button>
      {open
        ? found.skipped.map((item, index) => (
          <div key={`${item.app}-${item.name}-${index}`} className="flex flex-wrap items-baseline gap-2 text-xs">
            <span className="font-medium">{item.name}</span>
            <Badge variant="muted">{item.app}</Badge>
            <span className="text-muted-foreground">{item.reason}</span>
          </div>
        ))
        : null}
    </div>
  );
}

/**
 * Everything a CC Switch installation on this machine holds, brought over in one
 * go. The list is a preview: the keys in it are masked, and the import reads the
 * real values from CC Switch's own store, so no credential makes the round trip
 * through this page.
 *
 * Instructions start unticked. Everything else is added beside what Gateway
 * already has, but an agent reads one instruction file and importing one writes
 * over it, so that choice is made rather than inherited.
 */
export function CcSwitchImportCard({onImported}: {onImported?: () => void}) {
  const [found, setFound] = React.useState<CcSwitchImport | null>(null);
  const [selection, setSelection] = React.useState<Selection>(emptySelection);
  const [overwrite, setOverwrite] = React.useState(false);
  const [loading, setLoading] = React.useState(true);
  const [busy, setBusy] = React.useState(false);
  const [result, setResult] = React.useState<CcSwitchResult | null>(null);
  const [error, setError] = React.useState("");

  const scan = React.useCallback(() => {
    setLoading(true);
    setError("");
    setResult(null);
    CcSwitchBackend.getCcSwitchImport()
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("link:CC Switch cannot be read"));
          return;
        }
        setFound(res.data);
        setSelection(defaultSelection(res.data));
      })
      .catch(err => setError(`${err}`))
      .then(() => setLoading(false));
  }, []);

  React.useEffect(scan, [scan]);

  const toggle = (kind: Kind, key: string) => {
    setSelection(previous => ({
      ...previous,
      [kind]: previous[kind].includes(key) ? previous[kind].filter(item => item !== key) : [...previous[kind], key],
    }));
  };

  const total = kinds.reduce((sum, kind) => sum + selection[kind].length, 0);
  const everything = found === null ? 0 : kinds.reduce((sum, kind) => sum + found[kind].length, 0);

  const selectAll = () => {
    if (found === null) {
      return;
    }
    setSelection(
      total === everything
        ? emptySelection()
        : {
          providers: found.providers.map(item => item.key),
          mcps: found.mcps.map(item => item.key),
          prompts: found.prompts.map(item => item.key),
          skills: found.skills.map(item => item.key),
        },
    );
  };

  const runImport = () => {
    setBusy(true);
    setError("");
    CcSwitchBackend.importCcSwitch({...selection, overwrite: overwrite})
      .then(res => {
        if (res.status !== "ok") {
          setError(res.msg || i18next.t("link:This cannot be imported"));
          return;
        }
        setResult(res.data);
        const written = kinds.flatMap(kind => res.data[kind]);
        const failed = written.filter(item => item.action === "failed").length;
        Setting.showMessage(
          failed > 0 ? "error" : "success",
          failed > 0 ? i18next.t("link:Import finished with failures") : i18next.t("link:Imported"),
        );
        onImported?.();
      })
      .catch(err => setError(`${err}`))
      .then(() => setBusy(false));
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-[15px]">
          <ArrowDownToLine className="size-4" />
          {i18next.t("link:Import from CC Switch")}
        </CardTitle>
        <CardDescription>{i18next.t("link:Import from CC Switch hint")}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {error === "" ? null : <MessageAlert title={error} />}

        {loading ? (
          <Loading />
        ) : found === null || !found.found ? (
          <MessageAlert
            variant="info"
            title={i18next.t("link:No CC Switch here")}
            description={i18next.t("link:No CC Switch here hint").replace("{path}", found?.path ?? "")}
            action={
              <Button type="button" size="xs" variant="outline" onClick={scan}>
                <FolderSearch />
                {i18next.t("link:Look again")}
              </Button>
            }
          />
        ) : result !== null ? (
          <>
            <Outcomes title={kindTitle("providers")} items={result.providers} />
            <Outcomes title={kindTitle("mcps")} items={result.mcps} />
            <Outcomes title={kindTitle("prompts")} items={result.prompts} />
            <Outcomes title={kindTitle("skills")} items={result.skills} />
            <div>
              <Button type="button" variant="outline" onClick={scan}>
                {i18next.t("link:Read it again")}
              </Button>
            </div>
          </>
        ) : everything === 0 ? (
          <>
            <MessageAlert variant="info" title={i18next.t("link:Nothing to import")} />
            <Skipped found={found} />
          </>
        ) : (
          <>
            <p className="text-muted-foreground text-xs">
              <CodeText>{found.path}</CodeText>
            </p>

            <Section title={kindTitle("providers")} count={found.providers.length}>
              {found.providers.map(item => (
                <Row
                  key={item.key}
                  selected={selection.providers.includes(item.key)}
                  onToggle={() => toggle("providers", item.key)}
                  icon={<ProviderIcon baseUrl={item.provider.icon || item.provider.baseUrl} size={16} />}
                  title={item.provider.displayName}
                  detail={item.provider.baseUrl || item.provider.type}
                  badges={
                    <>
                      <Badge variant="muted">{item.app}</Badge>
                      {item.current ? <Badge variant="info">{i18next.t("link:In use")}</Badge> : null}
                      {item.taken ? <Badge variant="warning">{i18next.t("link:Name taken")}</Badge> : null}
                    </>
                  }
                />
              ))}
            </Section>

            <Section title={kindTitle("mcps")} count={found.mcps.length}>
              {found.mcps.map(item => (
                <Row
                  key={item.key}
                  disabled={item.targets.length === 0}
                  selected={selection.mcps.includes(item.key)}
                  onToggle={() => toggle("mcps", item.key)}
                  icon={<AgentIcon agent={item.targets[0] ?? ""} size={16} />}
                  title={item.name}
                  detail={
                    item.targets.length === 0 ? i18next.t("link:No agent here reads this") : undefined
                  }
                  badges={<Targets targets={item.targets} />}
                />
              ))}
            </Section>

            <Section title={kindTitle("prompts")} count={found.prompts.length}>
              {found.prompts.map(item => (
                <Row
                  key={item.key}
                  disabled={item.targets.length === 0}
                  selected={selection.prompts.includes(item.key)}
                  onToggle={() => toggle("prompts", item.key)}
                  icon={<AgentIcon agent={item.targets[0] ?? item.app} size={16} />}
                  title={item.name}
                  detail={i18next.t("link:This replaces the instructions")}
                  badges={<Targets targets={item.targets} />}
                />
              ))}
            </Section>

            <Section title={kindTitle("skills")} count={found.skills.length}>
              {found.skills.map(item => (
                <Row
                  key={item.key}
                  selected={selection.skills.includes(item.key)}
                  onToggle={() => toggle("skills", item.key)}
                  icon={<ProviderIcon baseUrl="github.com" size={16} />}
                  title={item.repo}
                  detail={item.ref}
                />
              ))}
            </Section>

            <Skipped found={found} />

            {found.mcps.length === 0 ? null : (
              <div className="flex items-center gap-2">
                <Switch id="ccswitch-overwrite" checked={overwrite} onCheckedChange={setOverwrite} />
                <Label htmlFor="ccswitch-overwrite" className="text-sm font-normal">
                  {i18next.t("link:Replace what an agent already has")}
                </Label>
              </div>
            )}

            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" loading={busy} disabled={total === 0} onClick={runImport}>
                <ArrowDownToLine />
                {i18next.t("link:Import selected").replace("{count}", `${total}`)}
              </Button>
              <Button type="button" variant="ghost" onClick={selectAll}>
                {total === everything ? i18next.t("link:Select none") : i18next.t("link:Select everything")}
              </Button>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
