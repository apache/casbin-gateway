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
import {TriangleAlert} from "lucide-react";
import i18next from "i18next";

import * as MiscBackend from "@/backend/MiscBackend";
import * as SiteBackend from "@/backend/SiteBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/DataTable";
import {Alert, AlertDescription, AlertTitle} from "@/components/ui/alert";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import {ConfirmButton} from "@/components/ui/confirm-button";
import {Tooltip} from "@/components/ui/tooltip";
import type {Account, Site} from "@/types";

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
  const [gatewayEnabled, setGatewayEnabled] = React.useState<boolean | null>(null);

  const fetchSites = React.useCallback(() => {
    setLoading(true);
    SiteBackend.getSites(account.name).then(res => {
      setLoading(false);
      if (res.status === "ok") {
        setData(res.data ?? []);
      } else {
        Setting.showMessage("error", `${i18next.t("site:Failed to get sites")}: ${res.msg}`);
      }
    });
  }, [account.name]);

  React.useEffect(() => {
    fetchSites();
    // The sites below only do something when the reverse proxy is running, so
    // the page has to say when it is off. Otherwise every site here looks
    // configured and nothing is actually proxied.
    MiscBackend.getGatewayStatus().then(res => {
      if (res.status === "ok") {
        setGatewayEnabled(res.data.gatewayEnabled);
      }
    });
  }, [fetchSites]);

  const addSite = () => {
    const site = newSite(account.name);
    SiteBackend.addSite(site)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("site:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("site:Site added successfully"));
          fetchSites();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("site:Failed to add")}: ${error}`));
  };

  const deleteSite = (site: Site) => {
    SiteBackend.deleteSite(site)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("site:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("site:Site deleted successfully"));
          fetchSites();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("site:Failed to delete")}: ${error}`));
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
              <Badge variant={record.needRedirect ? "secondary" : "processing"}>{domain}</Badge>
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
              <Badge variant="processing">{rule}</Badge>
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
              <Badge variant="blue" key={index}>
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
            let variant: "processing" | "error" | "warning" | "success" =
              node.message === "" ? "processing" : "error";
            if (variant === "processing" && node.provider) {
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
              <Tooltip key={node.name} title={node.message}>
                <span>{badge}</span>
              </Tooltip>
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
          <Button size="sm" onClick={() => navigate(`/sites/${record.owner}/${record.name}`)}>
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmButton
            title={i18next.t("site:Sure to delete site: {name} ?").replace("{name}", record.name)}
            onConfirm={() => deleteSite(record)}
          >
            <Button size="sm" variant="destructive">
              {i18next.t("general:Delete")}
            </Button>
          </ConfirmButton>
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4 p-4 md:p-6">
      {gatewayEnabled === false && (
        <Alert variant="warning">
          <TriangleAlert />
          <AlertTitle>{i18next.t("site:The reverse proxy is not enabled")}</AlertTitle>
          <AlertDescription>
            {i18next.t(
              "site:The sites below will not be proxied. Set gatewayEnabled = true in conf/app.conf and restart Casbin Gateway to enable it.",
            )}
          </AlertDescription>
        </Alert>
      )}
      <DataTable
        columns={columns}
        data={data}
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        pageSize={20}
        title={i18next.t("general:Sites")}
        toolbar={
          <Button size="sm" onClick={addSite}>
            {i18next.t("general:Add")}
          </Button>
        }
      />
    </div>
  );
}
