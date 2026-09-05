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
import {ArrowRight, Pencil, Plus, Route, Trash2} from "lucide-react";
import i18next from "i18next";

import * as ModelRouteBackend from "@/backend/ModelRouteBackend";
import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {RouteEditDialog} from "@/components/llm/route-edit-dialog";
import {RoutePreviewCard} from "@/components/llm/route-preview-card";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {DataTable, type Column} from "@/components/shared/data-table";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {UnauthorizedResult} from "@/components/shared/misc";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {useAgents} from "@/lib/agents";
import type {Account, ModelRoute, Provider, RouteTarget} from "@/types";

/** One rung as the table shows it: the model asked for, and who has to answer. */
function TargetChip({target, providers}: {target: RouteTarget; providers: Provider[]}) {
  const provider = providers.find(candidate => `${candidate.owner}/${candidate.name}` === target.provider);
  const on = provider ? provider.displayName || provider.name : target.provider;

  return (
    <span className="bg-muted/50 inline-flex max-w-full items-center gap-1 rounded-md border px-1.5 py-0.5">
      <span className="truncate font-mono text-xs">{target.model || i18next.t("llm:Same model")}</span>
      {target.provider ? <span className="text-muted-foreground truncate text-xs">@{on}</span> : null}
    </span>
  );
}

/**
 * The rules that decide which model a request is actually sent, and what it
 * steps down to when that cannot answer. The preview beside them is what makes
 * a ladder readable: the rungs below the first are the part nobody sees until
 * an upstream is already failing.
 */
export default function ModelRoutesPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [routes, setRoutes] = React.useState<ModelRoute[]>([]);
  const [providers, setProviders] = React.useState<Provider[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [editing, setEditing] = React.useState<ModelRoute | null>(null);
  const [dialogOpen, setDialogOpen] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const {agents} = useAgents(isAdmin);

  const load = React.useCallback(() => {
    if (!isAdmin) {
      return;
    }
    ModelRouteBackend.getModelRoutes()
      .then(res => {
        if (res.status === "ok") {
          setRoutes(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(failure => setError(failure.message || String(failure)))
      .then(() => setLoading(false));
  }, [isAdmin]);

  React.useEffect(() => {
    load();
    if (isAdmin) {
      ProviderBackend.getProviders(account.name)
        .then(res => setProviders(res.status === "ok" ? (res.data ?? []) : []))
        .catch(() => undefined);
    }
  }, [load, isAdmin, account]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  const save = (route: ModelRoute) => {
    setSaving(true);
    const call = editing
      ? ModelRouteBackend.updateModelRoute(editing.name, route)
      : ModelRouteBackend.addModelRoute(route);
    call
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          setDialogOpen(false);
          load();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${failure}`))
      .then(() => setSaving(false));
  };

  const remove = (name: string) =>
    ModelRouteBackend.deleteModelRoute(name)
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          load();
        } else {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        }
      })
      .catch(failure => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${failure}`));

  const open = (route: ModelRoute | null) => {
    setEditing(route);
    setDialogOpen(true);
  };

  const columns: Column<ModelRoute>[] = [
    {
      key: "match",
      title: i18next.t("llm:When the client asks for"),
      render: (_value, route) => (
        <div className="flex min-w-0 flex-col">
          <span className="truncate font-mono text-xs">{route.match}</span>
          {route.displayName && route.displayName !== route.name ? (
            <span className="text-muted-foreground truncate text-xs">{route.displayName}</span>
          ) : null}
        </div>
      ),
    },
    {
      key: "targets",
      title: i18next.t("llm:Send it to"),
      render: (_value, route) => (
        <div className="flex min-w-0 flex-wrap items-center gap-1">
          {route.targets.map((target, index) => (
            <React.Fragment key={index}>
              {index > 0 ? <ArrowRight className="text-muted-foreground size-3 shrink-0" /> : null}
              <TargetChip target={target} providers={providers} />
            </React.Fragment>
          ))}
        </div>
      ),
    },
    {
      key: "agent",
      title: i18next.t("agent:Agent"),
      width: "140px",
      render: (_value, route) =>
        route.agent ? (
          <span className="truncate font-mono text-xs">{route.agent}</span>
        ) : (
          <span className="text-muted-foreground text-xs">{i18next.t("llm:Every agent")}</span>
        ),
    },
    {
      key: "enabled",
      title: i18next.t("provider:Status"),
      width: "110px",
      render: (_value, route) => (
        <Badge variant={route.enabled ? "success" : "muted"}>
          {i18next.t(route.enabled ? "provider:Enabled" : "provider:Disabled")}
        </Badge>
      ),
    },
    {
      key: "actions",
      title: "",
      width: "96px",
      align: "right",
      render: (_value, route) => (
        <div className="flex items-center justify-end gap-1">
          <SimpleTooltip title={i18next.t("general:Edit")}>
            <Button size="icon" variant="ghost" onClick={() => open(route)}>
              <Pencil />
            </Button>
          </SimpleTooltip>
          <ConfirmDialog
            title={i18next.t("llm:Delete this routing rule?")}
            description={i18next.t("llm:Delete this routing rule detail")}
            onConfirm={() => remove(route.name)}
          >
            <Button size="icon" variant="ghost">
              <Trash2 />
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("llm:Model routing")}
        description={i18next.t("llm:Model routing description")}
        actions={
          <Button onClick={() => open(null)}>
            <Plus />
            {i18next.t("llm:Add a routing rule")}
          </Button>
        }
      />

      {error ? <MessageAlert title={error} /> : null}

      <DataTable
        columns={columns}
        dataSource={routes}
        rowKey={route => route.name}
        loading={loading}
        pageSize={25}
        emptyIcon={Route}
        emptyText={i18next.t("llm:No routing rules yet")}
      />

      <RoutePreviewCard agents={agents} />

      <RouteEditDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        route={editing}
        providers={providers}
        agents={agents}
        onSave={save}
        saving={saving}
      />
    </PageContainer>
  );
}
