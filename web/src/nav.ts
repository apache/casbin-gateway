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
  Blocks,
  Bot,
  ChartColumn,
  CircleDollarSign,
  FileSearch,
  Logs,
  MessageSquare,
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
  {key: "/", label: "agent:Agents", icon: Bot, path: "/"},
  // The table of installations is the home screen's own advanced view, linked
  // from it rather than competing with it in the rail.
  {key: "/agents", label: "agent:Advanced view", icon: Table2, path: "/agents", adminOnly: true, hidden: true},
  {key: "/providers", label: "provider:Providers", icon: Plug, path: "/providers"},
  {
    key: "/agent-configs",
    label: "agentConfig:Skills, MCP & Prompts",
    icon: Blocks,
    path: "/agent-configs",
    adminOnly: true,
  },
  {key: "/authenticity", label: "audit:Authenticity", icon: ShieldCheck, path: "/authenticity", adminOnly: true},
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
  {key: "/usage", label: "usage:Usage", icon: ChartColumn, path: "/usage", adminOnly: true},
  {key: "/pricing", label: "usage:Model pricing", icon: CircleDollarSign, path: "/pricing", adminOnly: true},
  {key: "/settings", label: "setting:Settings", icon: Settings, path: "/settings", adminOnly: true},
];

/** All leaf entries, flattened, for lookups by first path segment. */
export const navLeaves: NavLeaf[] = navGroups.flatMap(group =>
  group.children ? group.children : [{...group, path: group.path ?? group.key} as NavLeaf],
);

export function findLeaf(segmentKey: string) {
  return navLeaves.find(leaf => leaf.key === segmentKey);
}

export function findGroupOf(segmentKey: string) {
  return navGroups.find(group => group.children?.some(child => child.key === segmentKey));
}

/** The key the sidebar treats as selected: the first path segment, or "/" at home. */
export function selectedKeyOf(pathname: string) {
  const firstSegment = pathname.split("/").filter(Boolean)[0];
  return firstSegment === undefined ? "/" : `/${firstSegment}`;
}
