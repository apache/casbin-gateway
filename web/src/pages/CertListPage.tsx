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
import copy from "copy-to-clipboard";
import {Plus, RefreshCw, ShieldCheck} from "lucide-react";
import i18next from "i18next";

import * as CertBackend from "@/backend/CertBackend";
import * as Setting from "@/Setting";
import {DataTable, type Column} from "@/components/shared/data-table";
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {Button} from "@/components/ui/button";
import {ConfirmDialog} from "@/components/shared/confirm-dialog";
import {Input} from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {Account, Cert} from "@/types";

function newCert(owner: string): Cert {
  const randomName = Setting.getRandomName();
  return {
    owner: owner,
    name: `cert_${randomName}`,
    createdTime: new Date().toISOString(),
    displayName: `New Cert - ${randomName}`,
    type: "SSL",
    cryptoAlgorithm: "RSA",
    expireTime: "",
    domainExpireTime: "",
    provider: "",
    account: "",
    accessKey: "",
    accessSecret: "",
    certificate: "",
    privateKey: "",
  };
}

export default function CertListPage({account}: {account: Account}) {
  const navigate = useNavigate();
  const [data, setData] = React.useState<Cert[]>([]);
  const [total, setTotal] = React.useState(0);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState("");
  const [page, setPage] = React.useState(1);
  const [pageSize, setPageSize] = React.useState(20);
  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [form, setForm] = React.useState<Cert>(() => newCert(account.name));
  const [nameError, setNameError] = React.useState("");

  const fetchCerts = React.useCallback(
    (nextPage = page, nextPageSize = pageSize) => {
      setLoading(true);
      CertBackend.getCerts(account.name, nextPage, nextPageSize)
        .then(res => {
          if (res.status === "ok") {
            setData(res.data ?? []);
            setTotal(res.data2 ?? 0);
            setPage(nextPage);
            setPageSize(nextPageSize);
            setError("");
          } else {
            setError(res.msg || i18next.t("general:Failed to get data"));
          }
        })
        .catch(err => setError(err.message || String(err)))
        .then(() => setLoading(false));
    },
    [account.name, page, pageSize],
  );

  React.useEffect(() => {
    fetchCerts(1, 20);
    // Only the initial load: later fetches are driven by the pager and actions.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [account.name]);

  const openAddDialog = () => {
    setForm(newCert(account.name));
    setNameError("");
    setAddOpen(true);
  };

  const addCert = () => {
    const name = form.name.trim();
    if (name === "") {
      setNameError(i18next.t("general:Name cannot be empty"));
      return;
    }
    setAdding(true);
    CertBackend.addCert({...form, name: name})
      .then(res => {
        setAdding(false);
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Added successfully"));
          setAddOpen(false);
          fetchCerts();
        }
      })
      .catch(error => {
        setAdding(false);
        Setting.showMessage("error", `${i18next.t("general:Failed to add")}: ${error}`);
      });
  };

  const deleteCert = (cert: Cert) => {
    CertBackend.deleteCert(cert)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Deleted successfully"));
          fetchCerts();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to delete")}: ${error}`));
  };

  const refreshCert = (cert: Cert) => {
    CertBackend.refreshDomainExpire(cert.owner, cert.name)
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("cert:Failed to refresh domain expire")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("cert:Domain expire refreshed successfully"));
          fetchCerts();
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("cert:Failed to refresh domain expire")}: ${error}`));
  };

  const copyButton = (text: string, prefix: RegExp, message: string) => (
    <Button
      variant="outline"
      size="sm"
      className="font-mono"
      onClick={() => {
        copy(text);
        Setting.showMessage("success", message);
      }}
    >
      {Setting.getShortText((text ?? "").replace(prefix, ""), 15)}
    </Button>
  );

  const columns: Column<Cert>[] = [
    {
      title: i18next.t("general:Owner"),
      key: "owner",
      dataIndex: "owner",
      width: "120px",
      sorter: (a, b) => a.owner.localeCompare(b.owner),
    },
    {
      title: i18next.t("general:Name"),
      key: "name",
      dataIndex: "name",
      width: "140px",
      sorter: (a, b) => a.name.localeCompare(b.name),
      render: (text: string, record) => (
        <Link to={`/certs/${record.owner}/${record.name}`} className="text-primary hover:underline">
          {text}
        </Link>
      ),
    },
    {
      title: i18next.t("general:Create time"),
      key: "createdTime",
      dataIndex: "createdTime",
      width: "170px",
      sorter: (a, b) => a.createdTime.localeCompare(b.createdTime),
      render: (text: string) => Setting.getFormattedDate(text),
    },
    {
      title: i18next.t("cert:Expire time"),
      key: "expireTime",
      dataIndex: "expireTime",
      width: "170px",
      sorter: (a, b) => (a.expireTime ?? "").localeCompare(b.expireTime ?? ""),
      render: (text: string) => Setting.getFormattedDate(text),
    },
    {
      title: i18next.t("cert:Domain expire"),
      key: "domainExpireTime",
      dataIndex: "domainExpireTime",
      width: "170px",
      sorter: (a, b) => (a.domainExpireTime ?? "").localeCompare(b.domainExpireTime ?? ""),
      render: (text: string) => Setting.getFormattedDate(text),
    },
    {
      title: i18next.t("cert:Provider"),
      key: "provider",
      dataIndex: "provider",
      width: "120px",
      sorter: (a, b) => (a.provider ?? "").localeCompare(b.provider ?? ""),
    },
    {
      title: i18next.t("cert:Account"),
      key: "account",
      dataIndex: "account",
      width: "130px",
      sorter: (a, b) => (a.account ?? "").localeCompare(b.account ?? ""),
    },
    {
      title: i18next.t("cert:Certificate"),
      key: "certificate",
      dataIndex: "certificate",
      width: "180px",
      render: (text: string) =>
        copyButton(
          text,
          /^-----BEGIN CERTIFICATE-----/,
          i18next.t("cert:Certificate copied to clipboard successfully"),
        ),
    },
    {
      title: i18next.t("cert:Private key"),
      key: "privateKey",
      dataIndex: "privateKey",
      width: "180px",
      render: (text: string) =>
        copyButton(
          text,
          /^-----BEGIN RSA PRIVATE KEY-----/,
          i18next.t("cert:Private key copied to clipboard successfully"),
        ),
    },
    {
      title: i18next.t("general:Action"),
      key: "action",
      width: "260px",
      render: (_text, record) => (
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={() => refreshCert(record)}>
            {i18next.t("general:Refresh")}
          </Button>
          <Button size="sm" onClick={() => navigate(`/certs/${record.owner}/${record.name}`)}>
            {i18next.t("general:Edit")}
          </Button>
          <ConfirmDialog
            title={i18next.t("general:Sure to delete {name} ?").replace("{name}", record.name)}
            onConfirm={() => deleteCert(record)}
          >
            <Button size="sm" variant="destructive">
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
        title={i18next.t("general:Certs")}
        actions={
          <Button onClick={openAddDialog}>
            <Plus />
            {i18next.t("general:Add")}
          </Button>
        }
      />

      <DataTable
        columns={columns}
        dataSource={data}
        rowKey={record => `${record.owner}/${record.name}`}
        loading={loading}
        error={error}
        onRetry={() => fetchCerts()}
        serverPagination={{
          page: page,
          pageSize: pageSize,
          total: total,
          onChange: (nextPage, nextPageSize) => fetchCerts(nextPage, nextPageSize),
        }}
        title={i18next.t("general:Certs")}
        description={`${total} ${i18next.t("general:Certs")}`}
        emptyIcon={ShieldCheck}
        toolbar={
          <Button variant="outline" size="sm" onClick={() => fetchCerts()} loading={loading}>
            <RefreshCw />
            {i18next.t("general:Refresh")}
          </Button>
        }
      />

      <FormDialog
        open={addOpen}
        onOpenChange={setAddOpen}
        title={i18next.t("cert:New Cert")}
        submitting={adding}
        onSubmit={addCert}
      >
        <Field label={i18next.t("general:Name")} htmlFor="cert-name" required error={nameError}>
          <Input
            id="cert-name"
            value={form.name}
            onChange={event => {
              setForm({...form, name: event.target.value});
              setNameError("");
            }}
          />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="cert-display-name">
          <Input
            id="cert-display-name"
            value={form.displayName}
            onChange={event => setForm({...form, displayName: event.target.value})}
          />
        </Field>
        <Field label={i18next.t("cert:Crypto algorithm")}>
          <Select
            value={form.cryptoAlgorithm}
            onValueChange={value => setForm({...form, cryptoAlgorithm: value})}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="RSA">RSA</SelectItem>
              <SelectItem value="ECC">ECC</SelectItem>
            </SelectContent>
          </Select>
        </Field>
      </FormDialog>
    </PageContainer>
  );
}
