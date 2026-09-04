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
import {Bot, CornerDownLeft, Search} from "lucide-react";
import i18next from "i18next";

import * as AgentBackend from "@/backend/AgentBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import {agentDetailPath} from "@/lib/agents";
import {navGroups} from "@/nav";
import type {Account, Agent, Provider} from "@/types";
import {AgentIcon} from "@/components/AgentIcon";
import {Button} from "@/components/ui/button";
import {ProviderIcon} from "@/components/ProviderIcon";
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";

/** Owns the ⌘K / Ctrl+K shortcut, so the shell can hand `open` to both the palette and its trigger. */
export function useCommandPalette() {
  const [open, setOpen] = React.useState(false);

  React.useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen(previous => !previous);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return {open, setOpen};
}

/**
 * The lists behind the object hits. Neither endpoint takes a search term, so
 * they are fetched once the palette is first opened and filtered here — both
 * lists are the size of one machine's setup, not of a directory.
 */
function useSearchable(open: boolean, account: Account | null | undefined, isAdmin: boolean) {
  const [agents, setAgents] = React.useState<Agent[]>([]);
  const [providers, setProviders] = React.useState<Provider[]>([]);
  const loadedRef = React.useRef(false);

  React.useEffect(() => {
    if (!open || !account || loadedRef.current) {
      return;
    }
    loadedRef.current = true;

    ProviderBackend.getProviders(account.name)
      .then(res => setProviders(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => setProviders([]));

    if (isAdmin) {
      // The scan only an admin may run, so everyone else searches pages and
      // providers alone.
      AgentBackend.getAgents()
        .then(res => setAgents(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => setAgents([]));
    }
  }, [open, account, isAdmin]);

  return {agents, providers};
}

function agentValue(agent: Agent, agents: Agent[]) {
  const duplicated = agents.filter(candidate => candidate.agentId === agent.agentId).length > 1;
  return duplicated ? `${agent.name} ${agent.agentId} ${agent.path}` : `${agent.name} ${agent.agentId}`;
}

/** Every page in the rail, a group's sections included, as flat rows. */
function pageHits(isAdmin: boolean) {
  return navGroups
    .filter(group => !group.hidden && (!group.adminOnly || isAdmin))
    .flatMap(group => {
      const label = i18next.t(group.label);
      const self = {key: group.key, label: label, hint: "", path: group.path ?? group.key};
      const children = (group.children ?? []).map(child => ({
        key: child.key,
        label: i18next.t(child.label),
        hint: label,
        path: child.path,
      }));
      return [self, ...children];
    });
}

/**
 * Every console page by name, and the agents and providers behind them — the
 * way to reach one of thirty providers without paging through the list.
 */
export function CommandPalette({
  open,
  onOpenChange,
  account,
  isAdmin,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  account: Account | null | undefined;
  isAdmin: boolean;
}) {
  const navigate = useNavigate();
  const {agents, providers} = useSearchable(open, account, isAdmin);

  // Recomputed per render: the labels come from i18next, which changes language
  // without changing this component's inputs.
  const pages = pageHits(isAdmin);

  const go = (to: string) => {
    onOpenChange(false);
    navigate(to);
  };

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title={i18next.t("general:Search")}>
      <CommandInput placeholder={i18next.t("general:Search pages, agents and providers")} />
      <CommandList className="max-h-[380px]">
        <CommandEmpty>{i18next.t("general:No data")}</CommandEmpty>

        <CommandGroup heading={i18next.t("general:Pages")}>
          {pages.map(page => (
            <CommandItem
              key={page.key}
              value={`${page.hint} ${page.label}`}
              onSelect={() => go(page.path)}
            >
              <CornerDownLeft className="opacity-50" />
              <span className="flex-1 truncate">{page.label}</span>
              {page.hint === "" ? null : <span className="text-muted-foreground max-w-[50%] truncate text-xs">{page.hint}</span>}
            </CommandItem>
          ))}
        </CommandGroup>

        {agents.length === 0 ? null : (
          <>
            <CommandSeparator />
            <CommandGroup heading={i18next.t("agent:Agents")}>
              {agents.map(agent => (
                <CommandItem
                  key={`${agent.agentId}/${agent.path}`}
                  // The path only joins the searched text for an agent installed
                  // twice, where the name alone would name two rows.
                  value={agentValue(agent, agents)}
                  onSelect={() => go(agentDetailPath(agent, agents))}
                >
                  <AgentIcon agent={agent.agentId} size={16} fallback={<Bot className="size-4 opacity-50" />} />
                  <span className="flex-1 truncate">{agent.name || agent.agentId}</span>
                  <span className="text-muted-foreground max-w-[50%] truncate text-xs">{agent.path}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}

        {providers.length === 0 ? null : (
          <>
            <CommandSeparator />
            <CommandGroup heading={i18next.t("provider:Providers")}>
              {providers.map(provider => (
                <CommandItem
                  key={`${provider.owner}/${provider.name}`}
                  value={`${provider.displayName} ${provider.name} ${provider.type}`}
                  onSelect={() => go(`/providers/${provider.owner}/${provider.name}`)}
                >
                  <ProviderIcon icon={provider.icon} baseUrl={provider.baseUrl} alt={provider.type} size={16} />
                  <span className="flex-1 truncate">{provider.displayName || provider.name}</span>
                  <span className="text-muted-foreground max-w-[50%] truncate text-xs">{provider.type}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        )}
      </CommandList>
    </CommandDialog>
  );
}

/** The header's search box, which only ever opens the palette. */
export function CommandPaletteTrigger({onOpen}: {onOpen: () => void}) {
  const isMac = typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform);
  return (
    <>
      <button
        type="button"
        onClick={onOpen}
        className="border-input text-muted-foreground hover:bg-accent hover:text-foreground hidden h-8 w-56 items-center gap-2 rounded-md border px-2.5 text-sm transition-colors md:flex lg:w-72"
      >
        <Search className="size-3.5 shrink-0" />
        <span className="flex-1 truncate text-left">{i18next.t("general:Search")}</span>
        <kbd className="bg-muted rounded border px-1.5 py-0.5 text-[10px] leading-none font-medium">
          {isMac ? "⌘" : "Ctrl+"}K
        </kbd>
      </button>
      {/* No shortcut to press on a phone, so the bar becomes a button. */}
      <Button
        variant="ghost"
        size="icon-sm"
        className="md:hidden"
        onClick={onOpen}
        aria-label={i18next.t("general:Search")}
      >
        <Search className="size-4" />
      </Button>
    </>
  );
}
