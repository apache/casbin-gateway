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
import {useNavigate, useParams} from "react-router-dom";
import {Link2} from "lucide-react";
import i18next from "i18next";

import * as CertBackend from "@/backend/CertBackend";
import * as MiscBackend from "@/backend/MiscBackend";
import * as NodeBackend from "@/backend/NodeBackend";
import * as RuleBackend from "@/backend/RuleBackend";
import * as SiteBackend from "@/backend/SiteBackend";
import * as Setting from "@/Setting";
import {FormRow, PageHeader} from "@/components/FormRow";
import {NodeTable} from "@/components/NodeTable";
import {SiteRuleTable} from "@/components/rules/SiteRuleTable";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Combobox} from "@/components/ui/combobox";
import {Input} from "@/components/ui/input";
import {NumberInput} from "@/components/ui/number-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {PageSpinner} from "@/components/ui/spinner";
import {Switch} from "@/components/ui/switch";
import {TagsInput} from "@/components/ui/tags-input";
import type {Account, Application, Cert, Node, Rule, Site} from "@/types";

const sslModes = ["HTTP", "HTTPS and HTTP", "HTTPS Only", "Static Folder"];
const statuses = ["Active", "Inactive"];

export default function SiteEditPage({account}: {account: Account}) {
  const {owner = "", siteName = ""} = useParams();
  const navigate = useNavigate();

  const [site, setSite] = React.useState<Site | null>(null);
  const [certs, setCerts] = React.useState<Cert[]>([]);
  const [rules, setRules] = React.useState<Rule[]>([]);
  const [applications, setApplications] = React.useState<Application[]>([]);
  const [providers, setProviders] = React.useState<string[]>([]);
  const [nodes, setNodes] = React.useState<Node[]>([]);

  const getSite = React.useCallback(() => {
    SiteBackend.getSite(owner, siteName).then(res => {
      if (res.status === "ok") {
        setSite(res.data);
      } else {
        Setting.showMessage("error", `Failed to get site: ${res.msg}`);
      }
    });
  }, [owner, siteName]);

  React.useEffect(() => {
    getSite();
    CertBackend.getCerts(account.name).then(res => {
      if (res.status === "ok") {
        setCerts(res.data ?? []);
      }
    });
    RuleBackend.getRules(account.name).then(res => {
      if (res.status === "ok") {
        setRules(res.data ?? []);
      }
    });
    MiscBackend.getApplications(account.name).then(res => {
      if (res.status === "ok") {
        setApplications(res.data ?? []);
      }
    });
    MiscBackend.getProviders().then(res => {
      if (res.status === "ok") {
        // Only the notification providers can be alert targets.
        setProviders(
          (res.data ?? [])
            .filter(provider => provider.category === "SMS" || provider.category === "Email")
            .map(provider => `${provider.category}/${provider.name}`),
        );
      }
    });
    NodeBackend.getNodes(account.name).then(res => {
      if (res.status === "ok") {
        setNodes(res.data ?? []);
      }
    });
  }, [account.name, getSite]);

  const updateField = <K extends keyof Site>(key: K, value: Site[K]) => {
    setSite(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (site === null) {
      return;
    }

    SiteBackend.updateSite(site.owner, siteName, Setting.deepCopy(site))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to save: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Successfully saved");
          navigate(`/sites/${site.owner}/${site.name}`);
        }
      })
      .catch(error => Setting.showMessage("error", `failed to save: ${error}`));
  };

  if (site === null) {
    return <PageSpinner />;
  }

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("site:Edit Site")}>
        <Button variant="outline" onClick={() => navigate("/sites")}>
          {i18next.t("general:Cancel")}
        </Button>
        <Button onClick={save}>{i18next.t("general:Save")}</Button>
      </PageHeader>

      <Card>
        <CardContent className="divide-y py-0">
          <FormRow label={i18next.t("general:Name")}>
            <Input value={site.name} onChange={event => updateField("name", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("general:Display name")}>
            <Input
              value={site.displayName}
              onChange={event => updateField("displayName", event.target.value)}
            />
          </FormRow>
          <FormRow label={i18next.t("general:Tag")}>
            <Input value={site.tag ?? ""} onChange={event => updateField("tag", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("site:Domain")}>
            <Input value={site.domain} onChange={event => updateField("domain", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("site:Other domains")}>
            <TagsInput
              value={site.otherDomains}
              onChange={value => updateField("otherDomains", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Need redirect")}>
            <Switch
              checked={site.needRedirect}
              onCheckedChange={checked => updateField("needRedirect", checked)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Disable verbose")}>
            <Switch
              checked={site.disableVerbose}
              onCheckedChange={checked => updateField("disableVerbose", checked)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Rules")}>
            <SiteRuleTable
              title={i18next.t("general:Rules")}
              account={account}
              sources={rules}
              rules={site.rules}
              onUpdateRules={value => updateField("rules", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Enable alert")}>
            <Switch
              checked={site.enableAlert}
              onCheckedChange={checked => updateField("enableAlert", checked)}
            />
          </FormRow>
          {site.enableAlert && (
            <FormRow label={i18next.t("site:Alert interval")}>
              <NumberInput
                min={1}
                className="max-w-xs"
                value={site.alertInterval}
                addonAfter={i18next.t("usage:seconds")}
                onChange={value => updateField("alertInterval", value)}
              />
            </FormRow>
          )}
          {site.enableAlert && (
            <FormRow label={i18next.t("site:Alert try times")}>
              <NumberInput
                min={1}
                className="max-w-xs"
                value={site.alertTryTimes}
                onChange={value => updateField("alertTryTimes", value)}
              />
            </FormRow>
          )}
          {site.enableAlert && (
            <FormRow label={i18next.t("site:Alert providers")}>
              <TagsInput
                value={site.alertProviders}
                suggestions={providers}
                onChange={value => updateField("alertProviders", value)}
              />
            </FormRow>
          )}
          <FormRow label={i18next.t("site:Challenges")}>
            <TagsInput value={site.challenges} onChange={value => updateField("challenges", value)} />
          </FormRow>
          <FormRow label={i18next.t("site:Host")}>
            <div className="relative">
              <Link2 className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                className="pl-9"
                value={site.host}
                onChange={event => updateField("host", event.target.value)}
              />
            </div>
          </FormRow>
          <FormRow label={i18next.t("site:Port")}>
            <NumberInput
              min={0}
              max={65535}
              className="max-w-xs"
              value={site.port}
              onChange={value => updateField("port", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Hosts")}>
            <TagsInput value={site.hosts} onChange={value => updateField("hosts", value)} />
          </FormRow>
          <FormRow label={i18next.t("site:Public IP")}>
            <Input value={site.publicIp} disabled />
          </FormRow>
          <FormRow label={i18next.t("site:Node")}>
            <Input value={site.node} disabled />
          </FormRow>
          <FormRow label={i18next.t("site:Mode")}>
            <Select value={site.sslMode} onValueChange={value => updateField("sslMode", value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {sslModes.map(mode => (
                  <SelectItem key={mode} value={mode}>
                    {mode}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormRow>
          <FormRow label={i18next.t("site:SSL cert")}>
            {/* The certificate is issued and attached by the server. */}
            <Combobox
              disabled
              value={site.sslCert}
              options={certs.map(cert => ({value: cert.name}))}
              onChange={value => updateField("sslCert", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Casdoor app")}>
            <Combobox
              value={site.casdoorApplication}
              options={applications.map(application => ({value: application.name}))}
              onChange={value => updateField("casdoorApplication", value)}
            />
          </FormRow>
          <FormRow label={i18next.t("site:Status")}>
            <Select value={site.status} onValueChange={value => updateField("status", value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {statuses.map(status => (
                  <SelectItem key={status} value={status}>
                    {status}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormRow>
          <FormRow label={i18next.t("site:Nodes")}>
            <NodeTable
              title={i18next.t("site:Nodes")}
              table={site.nodes}
              siteName={site.name}
              account={account}
              nodes={nodes}
              onUpdateTable={value => updateField("nodes", value)}
            />
          </FormRow>
        </CardContent>
      </Card>

      <div className="mt-4">
        <Button size="lg" onClick={save}>
          {i18next.t("general:Save")}
        </Button>
      </div>
    </div>
  );
}
