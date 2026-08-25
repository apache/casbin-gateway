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
import {Link, useNavigate} from "react-router-dom";
import {Globe, Pencil, Plus, RefreshCw, Trash2} from "lucide-react";
import i18next from "i18next";

import * as MiscBackend from "@/backend/MiscBackend";
import * as SettingBackend from "@/backend/SettingBackend";
import * as SiteBackend from "@/backend/SiteBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {MessageAlert} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {NumberInput} from "@/components/shared/number-input";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {Switch} from "@/components/ui/switch";
import type {Account, GatewayStatus, Site} from "@/types";

function newSite(owner: string): Site {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `site_${randomName}`,
    createdTime: new Date().toISOString(),
    displayName: `New Site - ${randomName}`,
    tag: "",
    domain: "door.casdoor.com",
    otherDomains: [],
    needRedirect: false,
    disableVerbose: false,
    rules: [],
    enableAlert: false,
    alertInterval: 60,
    alertTryTimes: 3,
    alertProviders: [],
    challenges: [],
    host: "",
    port: 8000,
    hosts: [],
    sslMode: "HTTPS Only",
    sslCert: "",
    publicIp: "8.131.81.162",
    node: "",
    isSelf: false,
    nodes: [],
    casdoorApplication: "",
  };
}

export default function SiteListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Site[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [gatewayStatus, setGatewayStatus] = React.useState<GatewayStatus | null>(null);
  const [switchingGateway, setSwitchingGateway] = React.useState(false);
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Site>(() => newSite(account.name));
  const [nameError, setNameError] = React.useState("");

  const fetchSites = React.useCallback(() => {
    setLoading(true);
    SiteBackend.getSites(account.name)
      .then(res => {
        if (res.status === "ok") {
          setData(res.data ?? []);
          setError("");
        } else {
          setError(res.msg || i18next.t("general:Failed to get data"));
        }
      })
      .catch(err => setError(err.message || String(err)))
      .then(() => setLoading(false));
  }, [account.name]);

  const loadGatewayStatus = React.useCallback(() => {
    MiscBackend.getGatewayStatus().then(res => {
      if (res.status === "ok") {
        setGatewayStatus(res.data);
      }
    });
  }, []);

  React.useEffect(() => {
    fetchSites();
    // The sites below only do something when the reverse proxy is running, so
    // the page has to say when it is off. Otherwise every site here looks
    // configured and nothing is actually proxied.
    loadGatewayStatus();
  }, [fetchSites, loadGatewayStatus]);

  // A shortcut for the one setting this page is about: the Settings page holds
  // the same field, and the backend stores it and takes the two gateway ports
  // straight away, or gives them back.
  const switchGateway = (enabled: boolean) => {
    setSwitchingGateway(true);
    SettingBackend.getSetting()
      .then(res => {
        if (res.status !== "ok") {
          throw new Error(res.msg);
        }
        return SettingBackend.updateSetting({...res.data, gatewayEnabled: enabled});
      })
      .then(res => {
        setSwitchingGateway(false);
        // A port that refuses to bind still leaves the setting saved, so the
        // banner below is told what the state really is either way.
        loadGatewayStatus();
        if (res.status === "ok") {
          Setting.showMessage(
            "success",
            enabled ? i18next.t("site:The reverse proxy is now running") : i18next.t("site:The reverse proxy is now stopped"),
          );
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(error => {
        setSwitchingGateway(false);
        loadGatewayStatus();
        Setting.showMessage("error", `${error}`);
      });
  };

  const openAddDialog = () => {
    setForm(newSite(account.name));
    setNameError("");
    setAddOpen(true);
  };

  const addSite = () => {
    const name = form.name.trim();
    if (name === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    setAdding(true);
    SiteBackend.addSite({...form, name: name})
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Added successfully"));
          setAddOpen(false);
          fetchSites();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${error}`);
      });
  };

  const deleteSite = (site: Site) => {
    SiteBackend.deleteSite(site)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          fetchSites();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${error}`));
  };

  const columns: Column<Site>[] = [
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
      width: "100px",
      sorter: (a, b) => a.owner.localeCompare(b.owner),
    },
    {
      title: i18next.t("general:Tag"),
      key: "tag",
      dataIndex: "tag",
      width: "130px",
      sorter: (a, b) => (a.tag ?? "").localeCompare(b.tag ?? ""),
      render: (text: string, record) =>
        text ? (
          <Link to={`/nodes/${record.owner}/${text}`} className="text-primary hover:underline">
            {text}
          </Link>
        ) : null,
    },
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "140px",
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (text: string, record) => (
        <Link to={`/sites/${record.owner}/${record.name}`} className="text-primary hover:underline">
          {text}
        </Link>
      ),
    },
    {
      title: i18next.t("general:Display name"),
      key: "displayName",
      dataIndex: "displayName",
      sorter: (a, b) => a.displayName.localeCompare(b.displayName),
    },
    {
      title: i18next.t("site:Domain"),
      key: "domain",
      dataIndex: "domain",
      width: "160px",
      sorter: (a, b) => a.domain.localeCompare(b.domain),
      render: (text: string, record) =>
        record.publicIp === "" ? (
          text
        ) : (
          <a
            target="_blank"
            rel="noreferrer"
            href={`https://${text}`}
            className="text-primary hover:underline"
          >
            {text}
          </a>
        ),
    },
    {
      title: i18next.t("site:Other domains"),
      key: "otherDomains",
      dataIndex: "otherDomains",
      width: "160px",
      render: (_text, record) => (
        <div className="flex flex-wrap gap-1">
          {(record.otherDomains ?? []).map(domain => (
            <a key={domain} target="_blank" rel="noreferrer" href={`https://${domain}`}>
              <Badge variant={record.needRedirect ? "muted" : "info"}>{domain}</Badge>
            </a>
          ))}
        </div>
      ),
    },
    {
      title: i18next.t("general:Rules"),
      key: "rules",
      dataIndex: "rules",
      width: "160px",
      render: (_text, record) => (
        <div className="flex flex-wrap gap-1">
          {(record.rules ?? []).map(rule => (
            <a key={rule} target="_blank" rel="noreferrer" href={`/rules/${rule}`}>
              <Badge variant="info">{rule}</Badge>
            </a>
          ))}
        </div>
      ),
    },
    {
      title: i18next.t("site:Host"),
      key: "host",
      dataIndex: "host",
      width: "120px",
      sorter: (a, b) => a.host.localeCompare(b.host),
      render: (_text, record) => {
        const host = record.host === "" ? String(record.port) : `${record.host}:${record.port}`;
        return record.status === "Active" ? host : <Badge variant="warning">{host}</Badge>;
      },
    },
    {
      title: i18next.t("site:Hosts"),
      key: "hosts",
      dataIndex: "hosts",
      width: "200px",
      sorter: (a, b) => (a.hosts?.length ?? 0) - (b.hosts?.length ?? 0),
      render: (hosts: string[]) =>
        Array.isArray(hosts) ? (
          <div className="flex flex-wrap gap-1">
            {hosts.map((host, index) => (
              <Badge variant="muted" key={index}>
                {host}
              </Badge>
            ))}
          </div>
        ) : null,
    },
    {
      title: i18next.t("site:Nodes"),
      key: "nodes",
      dataIndex: "nodes",
      width: "200px",
      sorter: (a, b) => (a.nodes?.length ?? 0) - (b.nodes?.length ?? 0),
      render: (_text, record) => (
        <div className="flex flex-wrap gap-1">
          {(record.nodes ?? []).map(node => {
            const versionInfo = Setting.getVersionInfo(node.version, record.name);
            let variant: "info" | "danger" | "warning" | "success" = node.message === "" ? "info" : "danger";
            if (variant === "info" && node.provider) {
              variant = node.version === "" ? "warning" : "success";
            }

            const badge =
              versionInfo === null ? (
                <Badge variant={variant}>{node.name}</Badge>
              ) : (
                <a target="_blank" rel="noreferrer" href={versionInfo.link}>
                  <Badge variant={variant}>{`${node.name} (${versionInfo.text})`}</Badge>
                </a>
              );

            return (
              <SimpleTooltip key={node.name} title={node.message}>
                <span>{badge}</span>
              </SimpleTooltip>
            );
          })}
        </div>
      ),
    },
    {
      title: i18next.t("site:SSL cert"),
      key: "sslCert",
      dataIndex: "sslCert",
      width: "140px",
      sorter: (a, b) => a.sslCert.localeCompare(b.sslCert),
      render: (text: string) =>
        text ? (
          <Link to={`/certs/admin/${text}`} className="text-primary hover:underline">
            {text}
          </Link>
        ) : null,
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "180px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={() => navigate(`/sites/${record.owner}/${record.name}`)}>
            <Pencil />
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmDialog
            title={i18next.t("general:Sure to delete {name} ?").replace("{name}", record.name)}
            confirmText={i18next.t("general:Delete")}
            onConfirm={() => deleteSite(record)}
          >
            <Button size="sm" variant="outline" className="text-destructive">
              <Trash2 />
              {i18next.t("general:Delete")}
            </Button>
          </ConfirmDialog>
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("general:Sites")}
        actions={
          <>
            {account.isAdmin && gatewayStatus !== null ? (
              <Label className="text-sm font-normal">
                <Switch checked={gatewayStatus.gatewayEnabled} disabled={switchingGateway} onCheckedChange={switchGateway} />
                {i18next.t("site:Reverse proxy")}
              </Label>
            ) : null}
            <Button onClick={openAddDialog}>
              <Plus />
              {i18next.t("general:Add")}
            </Button>
          </>
        }
      />

      {gatewayStatus?.gatewayEnabled === false && (
        <MessageAlert
          variant="warning"
          title={i18next.t("site:The reverse proxy is not enabled")}
          description={i18next.t("site:The sites below will not be proxied until it is turned on.")}
        />
      )}

      {gatewayStatus?.gatewayEnabled && !gatewayStatus.gatewayRunning && (
        <MessageAlert
          variant="destructive"
          title={i18next.t("site:The reverse proxy could not start")}
          description={[gatewayStatus.gatewayError, i18next.t("site:Free the port and switch it on again, or leave it off to run the management UI only.")]
            .filter(Boolean)
            .join(" ")}
        />
      )}

      <DataTable
        columns={columns}
        dataSource={data}
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        error={error}
        onRetry={() => fetchSites()}
        pageSize={20}
        searchable
        title={i18next.t("general:Sites")}
        description={`${data.length} ${i18next.t("general:Sites")}`}
        emptyIcon={Globe}
        toolbar={
          <Button variant="outline" size="sm" onClick={() => fetchSites()} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("site:New Site")}
        submitting={adding}
        onSubmit={addSite}
      >
        <Field label={i18next.t("general:Name")} htmlFor="site-name" required error={nameError}>
          <Input
            id="site-name"
            value={form.name}
            onChange={event => {
              setForm({...form, name: event.target.value});
              setNameError("");
            }}
          />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="site-display-name">
          <Input
            id="site-display-name"
            value={form.displayName}
            onChange={event => setForm({...form, displayName: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("site:Domain")} htmlFor="site-domain">
          <Input
            id="site-domain"
            value={form.domain}
            onChange={event => setForm({...form, domain: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("site:Host")} htmlFor="site-host">
          <Input
            id="site-host"
            value={form.host}
            onChange={event => setForm({...form, host: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("site:Port")}>
          <NumberInput
            min={0}
            max={65535}
            className="max-w-xs"
            value={form.port}
            onChange={value => setForm({...form, port: value})}
          />
        </Field>
      </FormDialog>
    </PageContainer>
  );
}
