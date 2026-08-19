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
import copy from "copy-to-clipboard";
import FileSaver from "file-saver";
import i18next from "i18next";

import * as CertBackend from "@/backend/CertBackend";
import * as Setting from "@/Setting";
import {FormRow, PageHeader} from "@/components/FormRow";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {PageSpinner} from "@/components/ui/spinner";
import {Textarea} from "@/components/ui/textarea";
import type {Cert} from "@/types";

export default function CertEditPage() {
  const {owner = "", certName = ""} = useParams();
  const navigate = useNavigate();
  const [cert, setCert] = React.useState<Cert | null>(null);

  React.useEffect(() => {
    CertBackend.getCert(owner, certName).then(res => {
      if (res.status === "ok") {
        setCert(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("cert:Failed to get cert")}: ${res.msg}`);
      }
    });
  }, [certName, owner]);

  const updateField = <K extends keyof Cert>(key: K, value: Cert[K]) => {
    setCert(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (cert === null) {
      return;
    }

    CertBackend.updateCert(cert.owner, certName, Setting.deepCopy(cert))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("cert:Failed to save")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("cert:Successfully saved"));
          navigate(`/certs/${cert.owner}/${cert.name}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("cert:Failed to save")}: ${error}`));
  };

  if (cert === null) {
    return <PageSpinner />;
  }

  const download = (text: string, filename: string) => {
    FileSaver.saveAs(new Blob([text], {type: "text/plain;charset=utf-8"}), filename);
  };

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("cert:Edit Cert")}>
        <Button variant="outline" onClick={() => navigate("/certs")}>
          {i18next.t("general:Cancel")}
        </Button>
        <Button onClick={save}>{i18next.t("general:Save")}</Button>
      </PageHeader>

      <Card>
        <CardContent className="divide-y py-0">
          <FormRow label={i18next.t("general:Name")}>
            <Input value={cert.name} onChange={event => updateField("name", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("cert:Type")}>
            <Select value={cert.type} onValueChange={value => updateField("type", value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="SSL">SSL</SelectItem>
              </SelectContent>
            </Select>
          </FormRow>
          <FormRow label={i18next.t("cert:Crypto algorithm")}>
            <Select
              value={cert.cryptoAlgorithm}
              onValueChange={value => updateField("cryptoAlgorithm", value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="RSA">RSA</SelectItem>
                <SelectItem value="ECC">ECC</SelectItem>
              </SelectContent>
            </Select>
          </FormRow>
          <FormRow label={i18next.t("cert:Expire time")}>
            <Input value={Setting.getFormattedDate(cert.expireTime) ?? ""} disabled />
          </FormRow>
          <FormRow label={i18next.t("cert:Domain expire")}>
            <Input value={Setting.getFormattedDate(cert.domainExpireTime) ?? ""} disabled />
          </FormRow>
          <FormRow label={i18next.t("cert:Provider")}>
            <Select value={cert.provider} onValueChange={value => updateField("provider", value)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="GoDaddy">GoDaddy</SelectItem>
                <SelectItem value="Aliyun">Aliyun</SelectItem>
              </SelectContent>
            </Select>
          </FormRow>
          <FormRow label={i18next.t("cert:Account")}>
            <Input value={cert.account} onChange={event => updateField("account", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("cert:Access key")}>
            <Input
              value={cert.accessKey}
              onChange={event => updateField("accessKey", event.target.value)}
            />
          </FormRow>
          <FormRow label={i18next.t("cert:Access secret")}>
            <Input
              value={cert.accessSecret}
              onChange={event => updateField("accessSecret", event.target.value)}
            />
          </FormRow>
        </CardContent>
      </Card>

      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <Card>
          <CardContent className="space-y-2 pt-4">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{i18next.t("cert:Certificate")}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  copy(cert.certificate);
                  Setting.showMessage(
                    "success",
                    i18next.t("cert:Certificate copied to clipboard successfully"),
                  );
                }}
              >
                {i18next.t("cert:Copy certificate")}
              </Button>
              <Button size="sm" onClick={() => download(cert.certificate, "token_jwt_key.pem")}>
                {i18next.t("cert:Download certificate")}
              </Button>
            </div>
            <Textarea
              className="h-[420px] font-mono text-xs"
              value={cert.certificate}
              onChange={event => updateField("certificate", event.target.value)}
            />
          </CardContent>
        </Card>

        <Card>
          <CardContent className="space-y-2 pt-4">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{i18next.t("cert:Private key")}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  copy(cert.privateKey);
                  Setting.showMessage(
                    "success",
                    i18next.t("cert:Private key copied to clipboard successfully"),
                  );
                }}
              >
                {i18next.t("cert:Copy private key")}
              </Button>
              <Button size="sm" onClick={() => download(cert.privateKey, "token_jwt_key.key")}>
                {i18next.t("cert:Download private key")}
              </Button>
            </div>
            <Textarea
              className="h-[420px] font-mono text-xs"
              value={cert.privateKey}
              onChange={event => updateField("privateKey", event.target.value)}
            />
          </CardContent>
        </Card>
      </div>

      <div className="mt-4">
        <Button size="lg" onClick={save}>
          {i18next.t("general:Save")}
        </Button>
      </div>
    </div>
  );
}
