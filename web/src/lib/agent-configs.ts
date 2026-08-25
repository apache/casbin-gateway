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
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import type {
  AgentConfigInventory,
  AgentConfigItem,
  AgentConfigKind,
  AgentConfigUpdateState,
} from "@/types";
import type {BadgeVariant} from "@/components/ui/badge";

/** An agent's configuration belongs to an account, so an id alone is not unique. */
export function inventoryKey(inventory: Pick<AgentConfigInventory, "agentId" | "owner">) {
  return `${inventory.owner}:${inventory.agentId}`;
}

export function itemsOf(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  if (kind === "skill") {
    return inventory.skills;
  }
  return kind === "prompt" ? inventory.prompts : inventory.mcpServers;
}

export function supports(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  if (kind === "skill") {
    return inventory.skillsSupported;
  }
  return kind === "prompt" ? inventory.promptSupported : inventory.mcpSupported;
}

/** Where the items of one kind live, shown under the agent it belongs to. */
export function locationOf(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  if (kind === "skill") {
    return inventory.skillsDir ?? "";
  }
  return (kind === "prompt" ? inventory.promptFile : inventory.mcpFile) ?? "";
}

/**
 * Why an agent cannot be a copy target, in the reader's words. Empty when it
 * can be one.
 */
export function blockedReason(inventory: AgentConfigInventory, kind: AgentConfigKind) {
  if (!supports(inventory, kind)) {
    return i18next.t("agentConfig:Gateway does not know where this agent keeps these");
  }
  if (kind === "mcp" && !inventory.mcpWritable) {
    return inventory.mcpReadOnly || i18next.t("agentConfig:This file is read-only for Gateway");
  }
  return "";
}

/**
 * The name two agents' copies of one item share. A skill from a plugin or from
 * a group inside the skills directory carries that in its name, but it is the
 * same skill once copied, so comparisons key on the last part. An item the
 * server matches some other way says so itself.
 */
export function sharedName(item: AgentConfigItem) {
  if (item.shared) {
    return item.shared;
  }
  const cut = Math.max(item.name.lastIndexOf(":"), item.name.lastIndexOf("/"));
  return cut < 0 ? item.name : item.name.slice(cut + 1);
}

/** What a row is titled: the file or folder, not the key it is matched by. */
export function displayName(item: AgentConfigItem) {
  const cut = Math.max(item.name.lastIndexOf(":"), item.name.lastIndexOf("/"));
  return cut < 0 ? item.name : item.name.slice(cut + 1);
}

/** The item of each name every agent holds, keyed by shared name then agent. */
export function copiesOf(inventories: AgentConfigInventory[], kind: AgentConfigKind) {
  const copies = new Map<string, Map<string, AgentConfigItem>>();
  inventories.forEach(inventory => {
    itemsOf(inventory, kind).forEach(item => {
      const holders = copies.get(sharedName(item)) ?? new Map<string, AgentConfigItem>();
      if (!holders.has(inventoryKey(inventory))) {
        holders.set(inventoryKey(inventory), item);
      }
      copies.set(sharedName(item), holders);
    });
  });
  return copies;
}

/** How another agent's copy of one item relates to the one on screen. */
export type VersionState = "same" | "newer" | "older" | "differs";

export function compareVersions(item: AgentConfigItem, other: AgentConfigItem): VersionState {
  if (item.digest && item.digest === other.digest) {
    return "same";
  }
  if (item.modified && other.modified && item.modified !== other.modified) {
    return other.modified > item.modified ? "newer" : "older";
  }
  return "differs";
}

/** The agents carrying a newer version of this item than the source does. */
export function newerHolders(
  item: AgentConfigItem,
  copies: Map<string, Map<string, AgentConfigItem>>,
  peers: AgentConfigInventory[],
) {
  const holders = copies.get(sharedName(item));
  if (!holders) {
    return [];
  }
  return peers
    .filter(peer => {
      const other = holders.get(inventoryKey(peer));
      return Boolean(other) && compareVersions(item, other as AgentConfigItem) === "newer";
    })
    .map(peer => peer.name);
}

/**
 * What a skill's own version says, as a badge. A skill is a folder of files
 * with no version in it, so this is the answer to the question the folder
 * cannot answer: is this still what its source holds. Skills with no known
 * source get nothing, because nothing is known about them.
 */
export function updateBadge(item: AgentConfigItem): {
  label: string;
  title: string;
  variant: BadgeVariant;
} | null {
  const update = item.update;
  if (!update || item.kind !== "skill") {
    return null;
  }

  const source = update.sourceName || update.source || "";
  const where = update.inferred
    ? i18next.t("agentConfig:Matched by name")
    : i18next.t("agentConfig:Copied by Gateway");
  const from = `${i18next.t("agentConfig:Source")}: ${source} (${where})`;

  const states: Record<AgentConfigUpdateState, {label: string; title: string; variant: BadgeVariant} | null> = {
    current: {
      label: i18next.t("agentConfig:Up to date"),
      title: `${i18next.t("agentConfig:Up to date detail")} ${from}`,
      variant: "success",
    },
    available: {
      label: i18next.t("agentConfig:Update available"),
      title: `${i18next.t("agentConfig:Update available detail")} ${from}`,
      variant: "warning",
    },
    modified: {
      label: i18next.t("agentConfig:Edited here"),
      title: `${i18next.t("agentConfig:Edited here detail")} ${from}`,
      variant: "info",
    },
    diverged: {
      label: i18next.t("agentConfig:Update available"),
      title: `${i18next.t("agentConfig:Diverged detail")} ${from}`,
      variant: "warning",
    },
    unknown: null,
  };
  return states[update.state] ?? null;
}

/** Whether Gateway can pull this skill's source over it. */
export function canUpdate(item: AgentConfigItem) {
  return (
    item.kind === "skill" &&
    !item.readOnly &&
    Boolean(item.update?.source) &&
    (item.update?.state === "available" || item.update?.state === "diverged")
  );
}

/** Where a skill outside the agent's own skills directory comes from. */
export function originTitle(item: AgentConfigItem) {
  if (item.scope === "project") {
    return i18next.t("agentConfig:From a project detail").replace("{project}", item.project || item.origin || "");
  }
  return i18next.t("agentConfig:From a plugin detail").replace("{plugin}", item.origin || "");
}

/** 2000-01-01: below this an mtime was never really set. */
const earliestPlausibleModified = 946684800;

export function formatModified(modified: number | undefined) {
  // Some packaging tools unpack files with the mtime zeroed, and "1/1/1970" as
  // the date of a skill reads as a bug rather than as "nobody recorded one".
  if (!modified || modified < earliestPlausibleModified) {
    return "";
  }
  return new Date(modified * 1000).toLocaleString();
}

/**
 * Picks the singular or the plural wording. English needs a different noun for
 * one item; the other locales map both keys to the same string.
 */
export function counted(count: number, oneKey: string, manyKey: string, token = "{count}") {
  return count === 1 ? i18next.t(oneKey) : i18next.t(manyKey).replace(token, String(count));
}

export function formatBytes(bytes: number | undefined) {
  if (!bytes) {
    return "";
  }
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value >= 10 || unit === 0 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

/** Whether this item can be picked for a copy to other agents. */
export function selectable(item: AgentConfigItem) {
  return !item.managed && !item.missing;
}

/** The one-line summary of an MCP server: what it runs, or what it connects to. */
export function endpointOf(item: AgentConfigItem) {
  return item.command || item.url || "";
}

/**
 * useAgentConfigs owns the host scan behind the Skills, MCP & Prompts page.
 * Deleting, editing and copying all change files this page has already read, so
 * every one of them ends by calling refresh().
 */
export function useAgentConfigs() {
  const [inventories, setInventories] = React.useState<AgentConfigInventory[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [scanned, setScanned] = React.useState(false);

  const refresh = React.useCallback((forceRefresh = false) => {
    setLoading(true);
    setError("");
    AgentConfigBackend.getAgentConfigs(forceRefresh)
      .then(res => {
        if (res.status === "ok") {
          setInventories(res.data ?? []);
        } else {
          setError(res.msg || i18next.t("agentConfig:Failed to read agent configuration"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => {
        setLoading(false);
        setScanned(true);
      });
  }, []);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  return {inventories, loading, error, scanned, refresh};
}
