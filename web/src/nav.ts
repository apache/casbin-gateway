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

import {
  ArrowDownToLine,
  Blocks,
  Bot,
  Cable,
  ChartColumn,
  CircleDollarSign,
  FileSearch,
  Logs,
  MessageSquare,
  PackageCheck,
  Plug,
  Settings,
  ShieldCheck,
  ShieldHalf,
  Table2,
  type LucideIcon,
} from "lucide-react";

export interface NavLeaf {
  key: string;
  label: string;
  path: string;
  icon?: LucideIcon;
  adminOnly?: boolean;
  /** Reached from a page rather than the rail, but still named by the breadcrumb. */
  hidden?: boolean;
}

export interface NavGroup {
  key: string;
  label: string;
  icon?: LucideIcon;
  path?: string;
  adminOnly?: boolean;
  hidden?: boolean;
  children?: NavLeaf[];
}

/**
 * One description of the navigation, read by both the sidebar and the
 * breadcrumb. Keeping them on the same tree is what stops the two from
 * disagreeing about which section a page belongs to.
 *
 * `label` is an i18next key, resolved at render time so a language switch does
 * not need the tree rebuilt.
 */
export const navGroups: NavGroup[] = [
  // The setup pages come first — agents, the models behind them, and the gates
  // they pass through — then what agents ran and what it cost, settings last.
  // A group's children are places inside its page, so the rail can land on the
  // right tab or section instead of only on the page's top.
  {key: "/", label: "agent:Agents", icon: Bot, path: "/"},
  // The table of installations is the home screen's own advanced view, linked
  // from it rather than competing with it in the rail.
  {key: "/agents", label: "agent:Advanced view", icon: Table2, path: "/agents", adminOnly: true, hidden: true},
  {
    key: "/agent-versions",
    label: "agent:Agent versions",
    icon: PackageCheck,
    path: "/agent-versions",
    adminOnly: true,
  },
  {key: "/providers", label: "provider:Providers", icon: Plug, path: "/providers"},
  // Under Providers, because bringing a CC Switch installation over is how
  // somebody arriving with one gets their providers, and it is not a page they
  // would think to look for behind the add-provider dialog.
  {key: "/import", label: "link:Import settings", icon: ArrowDownToLine, path: "/import", adminOnly: true},
  {
    key: "/agent-configs",
    label: "agentConfig:Skills, MCP & Prompts",
    icon: Blocks,
    path: "/agent-configs",
    adminOnly: true,
    children: [
      {key: "/agent-configs?tab=skill", label: "agentConfig:Skills", path: "/agent-configs?tab=skill"},
      {key: "/agent-configs?tab=mcp", label: "agentConfig:MCP servers", path: "/agent-configs?tab=mcp"},
      {key: "/agent-configs?tab=prompt", label: "agentConfig:Prompts", path: "/agent-configs?tab=prompt"},
    ],
  },
  // Connections sit beside Skills, MCP & Prompts because a connection is an MCP
  // server with its credentials kept here rather than in each agent's file.
  {key: "/connections", label: "connector:Connections", icon: Cable, path: "/connections", adminOnly: true},
  {
    key: "/authenticity",
    label: "audit:Authenticity",
    icon: ShieldCheck,
    path: "/authenticity",
    adminOnly: true,
    children: [
      {key: "/authenticity?tab=report", label: "audit:Report", path: "/authenticity?tab=report"},
      {key: "/authenticity?tab=cases", label: "audit:Test cases", path: "/authenticity?tab=cases"},
    ],
  },
  {key: "/permissions", label: "agent:Permissions", icon: ShieldHalf, path: "/permissions", adminOnly: true},
  {
    key: "/agent-sessions",
    label: "agent:Agent Sessions",
    icon: MessageSquare,
    path: "/agent-sessions",
    adminOnly: true,
  },
  {key: "/agent-records", label: "agent:Agent Records", icon: FileSearch, path: "/agent-records", adminOnly: true},
  {key: "/llm-records", label: "llm:LLM Records", icon: Logs, path: "/llm-records", adminOnly: true},
  {
    key: "/usage",
    label: "usage:Usage",
    icon: ChartColumn,
    path: "/usage",
    adminOnly: true,
    children: [
      {key: "/usage?tab=agents", label: "usage:Agent spend", path: "/usage?tab=agents"},
      {key: "/usage?tab=relayed", label: "usage:Relayed spend", path: "/usage?tab=relayed"},
    ],
  },
  {key: "/pricing", label: "usage:Model pricing", icon: CircleDollarSign, path: "/pricing", adminOnly: true},
  {
    key: "/settings",
    label: "setting:Settings",
    icon: Settings,
    path: "/settings",
    adminOnly: true,
    // One long page, so the children are its sections rather than tabs.
    children: [
      {key: "/settings#startup", label: "setting:Startup", path: "/settings#startup"},
      {key: "/settings#llm-records", label: "setting:LLM records", path: "/settings#llm-records"},
      {key: "/settings#probes", label: "setting:Channel probes", path: "/settings#probes"},
      {key: "/settings#agents", label: "setting:Agents", path: "/settings#agents"},
      {key: "/settings#signin", label: "setting:Sign-in", path: "/settings#signin"},
      {key: "/settings#security", label: "setting:Security", path: "/settings#security"},
      {key: "/settings#network", label: "setting:Network", path: "/settings#network"},
      {key: "/settings#backups", label: "setting:Backups", path: "/settings#backups"},
      {key: "/settings#cloud-sync", label: "setting:Cloud sync", path: "/settings#cloud-sync"},
      {key: "/settings#import-export", label: "setting:Import and export", path: "/settings#import-export"},
    ],
  },
];

/** All leaf entries, flattened, for lookups by first path segment. */
export const navLeaves: NavLeaf[] = navGroups.flatMap(group =>
  group.children ? group.children : [{...group, path: group.path ?? group.key} as NavLeaf],
);

export function findLeaf(segmentKey: string): NavLeaf | undefined {
  const leaf = navLeaves.find(entry => entry.key === segmentKey);
  if (leaf) {
    return leaf;
  }
  // A grouped page contributes its children to the leaves, not itself, so the
  // breadcrumb still has to be able to name the page they live on.
  const group = navGroups.find(entry => entry.key === segmentKey);
  return group ? {key: group.key, label: group.label, path: group.path ?? group.key} : undefined;
}

export function findGroupOf(segmentKey: string) {
  return navGroups.find(group => group.children?.some(child => child.key === segmentKey));
}

/**
 * The key the sidebar treats as selected. A page with children is selected at
 * one of them: the tab named by `?tab=`, the section named by the hash, or the
 * first child, which is what the page shows when neither is set.
 */
export function selectedKeyOf(pathname: string, search = "", hash = "") {
  const firstSegment = pathname.split("/").filter(Boolean)[0];
  const base = firstSegment === undefined ? "/" : `/${firstSegment}`;
  const group = navGroups.find(entry => entry.key === base);
  if (!group?.children?.length) {
    return base;
  }
  const tab = new URLSearchParams(search).get("tab");
  const suffix = hash || (tab ? `?tab=${tab}` : "");
  const match = group.children.find(child => child.key === `${base}${suffix}`);
  return (match ?? group.children[0]).key;
}
