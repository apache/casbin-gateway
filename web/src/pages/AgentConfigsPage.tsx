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
import {
  AlertTriangle,
  ArrowDownToLine,
  Check,
  Download,
  Eye,
  Link2,
  Minus,
  Package,
  Pencil,
  Plus,
  RefreshCw,
  ScanSearch,
  Send,
  Trash2,
} from "lucide-react";
import i18next from "i18next";

import * as AgentConfigBackend from "@/backend/AgentConfigBackend";
import * as Setting from "@/Setting";
import {AgentIcon} from "@/components/AgentIcon";
import {AddMcpDialog} from "@/components/agent-config/add-mcp-dialog";
import {CopyDialog} from "@/components/agent-config/copy-dialog";
import {DetailDialog, type DetailTarget} from "@/components/agent-config/detail-dialog";
import {InstallSkillDialog} from "@/components/agent-config/install-skill-dialog";
import {PromptDialog, type PromptTarget} from "@/components/agent-config/prompt-dialog";
import {TrashDialog} from "@/components/agent-config/trash-dialog";
import {UnmanagedSkillsDialog} from "@/components/agent-config/unmanaged-skills-dialog";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {EmptyState} from "@/components/shared/empty-state";
import {CodeText, UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {
  blockedReason,
  canUpdate,
  compareVersions,
  copiesOf,
  counted,
  displayName,
  endpointOf,
  formatBytes,
  formatModified,
  inventoryKey,
  itemsOf,
  locationOf,
  newerHolders,
  originTitle,
  selectable as canSelect,
  sharedName,
  supports,
  updateBadge,
  useAgentConfigs,
  type VersionState,
} from "@/lib/agent-configs";
import type {Account, AgentConfigInventory, AgentConfigItem, AgentConfigKind} from "@/types";

/** The agent picker doubles as the overview: every row carries its own counts. */
function SourcePicker({
  inventories,
  selectedKey,
  onSelect,
}: {
  inventories: AgentConfigInventory[];
  selectedKey: string;
  onSelect: (inventory: AgentConfigInventory) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {inventories.map(inventory => {
        const active = inventoryKey(inventory) === selectedKey;
        return (
          <button
            key={inventoryKey(inventory)}
            type="button"
            onClick={() => onSelect(inventory)}
            className={cn(
              "flex items-center gap-2.5 rounded-md border px-3 py-2 text-left transition-colors",
              active ? "border-primary bg-primary/10" : "hover:bg-accent",
            )}
          >
            <AgentIcon agent={inventory.name} size={20} fallback={<Package className="size-5" />} />
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5 text-sm leading-tight font-medium">
                {inventory.name}
                {inventory.installed ? null : (
                  <SimpleTooltip title={i18next.t("agentConfig:Not installed detail")}>
                    <Badge variant="muted">{i18next.t("agentConfig:Not installed")}</Badge>
                  </SimpleTooltip>
                )}
              </span>
              <span className="text-muted-foreground text-xs leading-tight">
                {i18next.t("agentConfig:Skills")} {inventory.skills.length}
                {" · "}
                {i18next.t("agentConfig:MCP")} {inventory.mcpServers.length}
                {inventory.promptSupported
                  ? ` · ${i18next.t("agentConfig:Prompts")} ${inventory.prompts.filter(prompt => !prompt.missing).length}`
                  : ""}
              </span>
            </span>
          </button>
        );
      })}
    </div>
  );
}

/**
 * What one other agent holds of the item on this row. A tick means the same
 * content; a warning means the same name with different content, which is the
 * version question the page exists to answer.
 */
function PeerCell({state}: {state: VersionState | "missing"}) {
  if (state === "missing") {
    return <Minus className="text-muted-foreground/40 mx-auto size-4" />;
  }
  if (state === "same") {
    return (
      <SimpleTooltip title={i18next.t("agentConfig:Same version")}>
        <Check className="text-success mx-auto size-4" />
      </SimpleTooltip>
    );
  }

  const title =
    state === "newer"
      ? i18next.t("agentConfig:Newer version here")
      : state === "older"
        ? i18next.t("agentConfig:Older version here")
        : i18next.t("agentConfig:Different version here");
  return (
    <SimpleTooltip title={title}>
      <AlertTriangle className="text-warning mx-auto size-4" />
    </SimpleTooltip>
  );
}

export default function AgentConfigsPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const {inventories, loading, error, scanned, refresh} = useAgentConfigs();

  const [kind, setKind] = React.useState<AgentConfigKind>("skill");
  const [sourceKey, setSourceKey] = React.useState("");
  const [selected, setSelected] = React.useState<string[]>([]);
  const [detail, setDetail] = React.useState<DetailTarget | null>(null);
  const [editing, setEditing] = React.useState<PromptTarget | null>(null);
  const [copyOpen, setCopyOpen] = React.useState(false);
  const [addOpen, setAddOpen] = React.useState(false);
  const [installOpen, setInstallOpen] = React.useState(false);
  const [trashOpen, setTrashOpen] = React.useState(false);
  const [untrackedOpen, setUntrackedOpen] = React.useState(false);
  const [deleting, setDeleting] = React.useState("");
  const [updating, setUpdating] = React.useState("");

  // The scan replaces every inventory, so the chosen source is re-resolved by
  // key rather than held as an object that would go stale on every refresh.
  const source = inventories.find(inventory => inventoryKey(inventory) === sourceKey) ?? inventories[0];

  // The first agent with something in it is a better landing page than the
  // first one alphabetically, but it is chosen once and then committed: left
  // derived, a copy that fills an empty agent would move the page off the
  // source the reader is working from.
  React.useEffect(() => {
    if (sourceKey === "" && inventories.length > 0) {
      const landing =
        inventories.find(inventory => itemsOf(inventory, kind).some(canSelect)) ?? inventories[0];
      setSourceKey(inventoryKey(landing));
    }
  }, [inventories, kind, sourceKey]);

  React.useEffect(() => {
    setSelected([]);
  }, [sourceKey, kind]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const peers = source ? inventories.filter(inventory => inventoryKey(inventory) !== inventoryKey(source)) : [];
  const items = source ? itemsOf(source, kind) : [];
  const copies = copiesOf(inventories, kind);
  const selectable = items.filter(canSelect);
  const agentNames = new Map(inventories.map(inventory => [inventory.agentId, inventory.name]));
  const allSelected = selectable.length > 0 && selected.length === selectable.length;

  const toggleAll = () => {
    setSelected(allSelected ? [] : selectable.map(item => item.name));
  };

  const toggleOne = (name: string) => {
    setSelected(previous =>
      previous.includes(name) ? previous.filter(item => item !== name) : [...previous, name],
    );
  };

  const deleteItem = (item: AgentConfigItem) => {
    setDeleting(item.name);
    return AgentConfigBackend.deleteAgentConfigItem(item.agentId, item.owner, item.kind, item.name)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", `${i18next.t("agentConfig:Moved to the recycle bin")}: ${item.name}`);
          refresh();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to delete"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setDeleting(""));
  };

  const updateItem = (item: AgentConfigItem) => {
    setUpdating(item.name);
    return AgentConfigBackend.updateAgentConfigSkill(item.agentId, item.owner, item.name)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", `${i18next.t("agentConfig:Updated")}: ${item.name}`);
          refresh();
        } else {
          Setting.showMessage("error", res.msg || i18next.t("agentConfig:Failed to update"));
        }
      })
      .catch(err => Setting.showMessage("error", err.message || String(err)))
      .then(() => setUpdating(""));
  };

  const columns: Column<AgentConfigItem>[] = [
    {
      key: "selected",
      width: "40px",
      title: (
        <input
          type="checkbox"
          className="accent-primary size-4 align-middle"
          aria-label={i18next.t("agentConfig:Select all")}
          checked={allSelected}
          disabled={selectable.length === 0}
          onChange={toggleAll}
        />
      ),
      render: (_value, record) =>
        !canSelect(record) ? null : (
          <input
            type="checkbox"
            className="accent-primary size-4 align-middle"
            aria-label={record.name}
            checked={selected.includes(record.name)}
            onChange={() => toggleOne(record.name)}
          />
        ),
    },
    {
      key: "name",
      dataIndex: "name",
      title: i18next.t("general:Name"),
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (_value, record) => {
        const outdated = newerHolders(record, copies, peers);
        const version = updateBadge(record);
        return (
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="truncate font-medium">{displayName(record)}</span>
              {record.origin ? (
                <SimpleTooltip title={originTitle(record)}>
                  <Badge variant="muted">{record.origin}</Badge>
                </SimpleTooltip>
              ) : null}
              {version ? (
                <SimpleTooltip title={version.title}>
                  <Badge variant={version.variant}>{version.label}</Badge>
                </SimpleTooltip>
              ) : null}
              {outdated.length > 0 ? (
                <SimpleTooltip
                  title={i18next
                    .t("agentConfig:Out of date detail")
                    .replace("{agents}", outdated.join(", "))}
                >
                  <Badge variant="warning">{i18next.t("agentConfig:Out of date")}</Badge>
                </SimpleTooltip>
              ) : null}
              {record.link ? (
                <SimpleTooltip
                  title={i18next.t("agentConfig:Linked detail").replace("{target}", record.link)}
                >
                  <Badge variant="info">
                    <Link2 className="size-3" />
                    {i18next.t("agentConfig:Linked")}
                  </Badge>
                </SimpleTooltip>
              ) : null}
              {record.managed ? (
                <SimpleTooltip title={i18next.t("agentConfig:Managed by Gateway detail")}>
                  <Badge variant="info">{i18next.t("agentConfig:Managed by Gateway")}</Badge>
                </SimpleTooltip>
              ) : null}
              {record.missing ? (
                <SimpleTooltip title={i18next.t("agentConfig:Not written yet detail")}>
                  <Badge variant="muted">{i18next.t("agentConfig:Not written yet")}</Badge>
                </SimpleTooltip>
              ) : null}
            </div>
            {record.description ? (
              <p className="text-muted-foreground line-clamp-2 text-xs">{record.description}</p>
            ) : null}
          </div>
        );
      },
    },
    {
      key: "summary",
      width: "220px",
      title:
        kind === "mcp" ? i18next.t("agentConfig:Endpoint") : i18next.t("agentConfig:Contents"),
      render: (_value, record) =>
        kind === "prompt" ? (
          <span className="text-muted-foreground text-xs">
            {record.missing || !record.bytes
              ? i18next.t("agentConfig:Nothing here")
              : formatBytes(record.bytes)}
            {formatModified(record.modified) ? (
              <>
                <br />
                {`${i18next.t("agentConfig:Changed")} ${formatModified(record.modified)}`}
              </>
            ) : null}
          </span>
        ) : kind === "skill" ? (
          <span className="text-muted-foreground text-xs">
            {counted(record.files ?? 0, "agentConfig:1 file", "agentConfig:{files} files", "{files}")}
            {record.bytes ? ` · ${formatBytes(record.bytes)}` : ""}
            {formatModified(record.modified) ? (
              <>
                <br />
                {`${i18next.t("agentConfig:Changed")} ${formatModified(record.modified)}`}
              </>
            ) : null}
          </span>
        ) : (
          <div className="flex min-w-0 items-center gap-1.5">
            {record.transport ? <Badge variant="muted">{record.transport}</Badge> : null}
            <span className="text-muted-foreground truncate font-mono text-xs" title={endpointOf(record)}>
              {endpointOf(record)}
            </span>
          </div>
        ),
    },
    ...peers.map<Column<AgentConfigItem>>(peer => ({
      key: `peer-${inventoryKey(peer)}`,
      width: "110px",
      align: "center",
      title: (
        <SimpleTooltip title={peer.name}>
          <span className="inline-flex items-center gap-1.5">
            <AgentIcon agent={peer.name} size={14} />
            <span className="truncate">{peer.name}</span>
          </span>
        </SimpleTooltip>
      ),
      render: (_value, record) => {
        if (!supports(peer, kind)) {
          return <span className="text-muted-foreground/50 text-xs">—</span>;
        }
        const other = copies.get(sharedName(record))?.get(inventoryKey(peer));
        // An instruction file is listed even when nobody has written it, so a
        // peer that has none holds nothing rather than a different version.
        const held = other && !other.missing && !record.missing ? other : undefined;
        return <PeerCell state={held ? compareVersions(record, held) : "missing"} />;
      },
    })),
    {
      key: "actions",
      width: "110px",
      align: "right",
      title: i18next.t("general:Action"),
      render: (_value, record) => (
        <div className="flex justify-end gap-1">
          {canUpdate(record) ? (
            <ConfirmDialog
              title={i18next.t("agentConfig:Update this skill?")}
              description={
                <span className="flex flex-col gap-1">
                  <span>{i18next.t("agentConfig:Update description")}</span>
                  <code className="text-foreground font-mono text-xs break-all">{record.update?.source}</code>
                  <span>{i18next.t("agentConfig:Update undo hint")}</span>
                </span>
              }
              onConfirm={() => updateItem(record)}
              disabled={updating === record.name}
            >
              <SimpleTooltip title={i18next.t("agentConfig:Update from source")}>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="text-warning"
                  aria-label={i18next.t("agentConfig:Update from source")}
                  disabled={updating === record.name}
                >
                  <ArrowDownToLine className="size-4" />
                </Button>
              </SimpleTooltip>
            </ConfirmDialog>
          ) : null}
          {kind === "prompt" ? (
            <SimpleTooltip title={i18next.t("general:Edit")}>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={i18next.t("general:Edit")}
                onClick={() => setEditing({item: record, agentName: source?.name ?? record.agentId})}
              >
                <Pencil className="size-4" />
              </Button>
            </SimpleTooltip>
          ) : (
            <SimpleTooltip title={i18next.t("agentConfig:View")}>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={i18next.t("agentConfig:View")}
                onClick={() => setDetail({item: record, agentName: source?.name ?? record.agentId})}
              >
                <Eye className="size-4" />
              </Button>
            </SimpleTooltip>
          )}
          <ConfirmDialog
            title={i18next.t(
              kind === "skill"
                ? "agentConfig:Delete this skill?"
                : kind === "prompt"
                  ? "agentConfig:Delete these instructions?"
                  : "agentConfig:Delete this MCP server?",
            )}
            description={
              <span className="flex flex-col gap-1">
                <span>{i18next.t("agentConfig:Delete description")}</span>
                <code className="text-foreground font-mono text-xs break-all">{record.path}</code>
                <span>{i18next.t("agentConfig:Delete undo hint")}</span>
              </span>
            }
            onConfirm={() => deleteItem(record)}
            disabled={record.managed || Boolean(record.readOnly) || deleting === record.name}
          >
            <SimpleTooltip title={record.readOnly || i18next.t("general:Delete")}>
              <Button
                variant="ghost"
                size="icon-sm"
                className="text-destructive"
                aria-label={i18next.t("general:Delete")}
                disabled={record.managed || Boolean(record.readOnly) || deleting === record.name}
              >
                <Trash2 className="size-4" />
              </Button>
            </SimpleTooltip>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  const location = source ? locationOf(source, kind) : "";
  const blocked = source ? blockedReason(source, kind) : "";

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("agentConfig:Skills, MCP & Prompts")}
        description={i18next.t("agentConfig:Page description")}
        actions={
          <>
            {kind === "skill" && inventories.length > 0 ? (
              <Button onClick={() => setInstallOpen(true)}>
                <Download className="size-4" />
                {i18next.t("agentConfig:Install skills")}
              </Button>
            ) : null}
            {kind === "mcp" && inventories.length > 0 ? (
              <Button onClick={() => setAddOpen(true)}>
                <Plus className="size-4" />
                {i18next.t("agentConfig:Add MCP server")}
              </Button>
            ) : null}
            {kind === "skill" && inventories.length > 0 ? (
              <Button variant="outline" onClick={() => setUntrackedOpen(true)}>
                <ScanSearch className="size-4" />
                {i18next.t("agentConfig:Untracked skills")}
              </Button>
            ) : null}
            {inventories.length > 0 ? (
              <Button variant="outline" onClick={() => setTrashOpen(true)}>
                <Trash2 className="size-4" />
                {i18next.t("agentConfig:Recycle bin")}
              </Button>
            ) : null}
            <Button variant="outline" onClick={() => refresh(true)} disabled={loading}>
              <RefreshCw className={cn("size-4", loading && "animate-spin")} />
              {i18next.t("general:Refresh")}
            </Button>
          </>
        }
      />

      {error ? <MessageAlert description={error} /> : null}

      {scanned && !error && inventories.length === 0 ? (
        <EmptyState
          icon={Package}
          title={i18next.t("agentConfig:No agents found")}
          description={i18next.t("agentConfig:No agents found detail")}
        />
      ) : null}

      {source ? (
        <>
          <SourcePicker
            inventories={inventories}
            selectedKey={inventoryKey(source)}
            onSelect={inventory => setSourceKey(inventoryKey(inventory))}
          />

          {source.errors?.length ? <MessageAlert description={source.errors.join("; ")} /> : null}

          <div className="flex flex-wrap items-center justify-between gap-3">
            <Tabs value={kind} onValueChange={value => setKind(value as AgentConfigKind)}>
              <TabsList>
                <TabsTrigger value="skill">
                  {i18next.t("agentConfig:Skills")}
                  <Badge variant="muted">{source.skills.length}</Badge>
                </TabsTrigger>
                <TabsTrigger value="mcp">
                  {i18next.t("agentConfig:MCP servers")}
                  <Badge variant="muted">{source.mcpServers.length}</Badge>
                </TabsTrigger>
                <TabsTrigger value="prompt">
                  {i18next.t("agentConfig:Prompts")}
                  <Badge variant="muted">{source.prompts.filter(prompt => !prompt.missing).length}</Badge>
                </TabsTrigger>
              </TabsList>
            </Tabs>

            {location ? (
              <span className="flex min-w-0 items-center gap-2 text-xs">
                <span className="text-muted-foreground shrink-0">{i18next.t("agentConfig:Read from")}</span>
                <CodeText copyable>{location}</CodeText>
                {kind === "skill" && (source.skillsDirs?.length ?? 0) > 1 ? (
                  <SimpleTooltip
                    title={
                      <span className="flex flex-col gap-0.5">
                        {source.skillsDirs?.map(dir => <span key={dir}>{dir}</span>)}
                      </span>
                    }
                  >
                    <Badge variant="muted">
                      {i18next
                        .t("agentConfig:and {count} more folders")
                        .replace("{count}", String((source.skillsDirs?.length ?? 1) - 1))}
                    </Badge>
                  </SimpleTooltip>
                ) : null}
              </span>
            ) : null}
          </div>

          {source.sharedWith?.length ? (
            <p className="text-muted-foreground text-xs">
              {i18next.t("agentConfig:Shared with").replace("{agents}", source.sharedWith.join(", "))}
            </p>
          ) : null}

          {blocked ? (
            <MessageAlert variant="info" description={blocked} />
          ) : (
            <DataTable<AgentConfigItem>
              columns={columns}
              dataSource={items}
              rowKey="name"
              loading={loading && !scanned}
              searchable
              searchPlaceholder={i18next.t("agentConfig:Search by name")}
              pageSize={20}
              emptyIcon={Package}
              emptyText={i18next.t(
                kind === "skill"
                  ? "agentConfig:No skills yet"
                  : kind === "prompt"
                    ? "agentConfig:No instructions yet"
                    : "agentConfig:No MCP servers yet",
              )}
              toolbar={
                <Button disabled={selected.length === 0} onClick={() => setCopyOpen(true)}>
                  <Send className="size-4" />
                  {selected.length === 0
                    ? i18next.t("agentConfig:Copy to other agents")
                    : i18next
                      .t("agentConfig:Copy {count} to other agents")
                      .replace("{count}", String(selected.length))}
                </Button>
              }
            />
          )}

          <CopyDialog
            open={copyOpen}
            onOpenChange={setCopyOpen}
            kind={kind}
            source={source}
            inventories={inventories}
            names={selected}
            onDone={refresh}
          />
        </>
      ) : null}

      {source ? (
        <InstallSkillDialog
          open={installOpen}
          onOpenChange={setInstallOpen}
          inventories={inventories}
          source={source}
          onDone={refresh}
        />
      ) : null}

      {source ? (
        <AddMcpDialog
          open={addOpen}
          onOpenChange={setAddOpen}
          inventories={inventories}
          source={source}
          onDone={refresh}
        />
      ) : null}

      <UnmanagedSkillsDialog
        open={untrackedOpen}
        onOpenChange={setUntrackedOpen}
        agentNames={agentNames}
        onDone={refresh}
      />

      <TrashDialog
        open={trashOpen}
        onOpenChange={setTrashOpen}
        owner={source?.owner ?? ""}
        agentNames={agentNames}
        onRestored={refresh}
      />

      <DetailDialog target={detail} onOpenChange={open => !open && setDetail(null)} />

      <PromptDialog
        target={editing}
        onOpenChange={open => !open && setEditing(null)}
        onSaved={refresh}
      />
    </PageContainer>
  );
}
