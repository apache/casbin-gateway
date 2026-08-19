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
import i18next from "i18next";

import * as RecordBackend from "@/backend/RecordBackend";
import * as Setting from "@/Setting";
import {FormRow, PageHeader} from "@/components/FormRow";
import {Button} from "@/components/ui/button";
import {Card, CardContent} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {PageSpinner} from "@/components/ui/spinner";
import type {Record as GatewayRecord} from "@/types";

export default function RecordEditPage() {
  const {owner = "", id = ""} = useParams();
  const navigate = useNavigate();
  const [record, setRecord] = React.useState<GatewayRecord | null>(null);

  React.useEffect(() => {
    RecordBackend.getRecord(owner, id).then(res => {
      if (res.status === "ok") {
        setRecord(res.data);
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to get record")}: ${res.msg}`);
      }
    });
  }, [id, owner]);

  const updateField = <K extends keyof GatewayRecord>(key: K, value: GatewayRecord[K]) => {
    setRecord(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (record === null) {
      return;
    }

    RecordBackend.updateRecord(record.owner, id, Setting.deepCopy(record))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${res.msg}`);
        } else {
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          navigate(`/records/${record.owner}/${record.id}`);
        }
      })
      .catch(error => Setting.showMessage("error", `${i18next.t("general:Failed to save")}: ${error}`));
  };

  if (record === null) {
    return <PageSpinner />;
  }

  const fields: {label: string; key: keyof GatewayRecord}[] = [
    {label: i18next.t("general:Owner"), key: "owner"},
    {label: i18next.t("general:CreatedTime"), key: "createdTime"},
    {label: i18next.t("general:Method"), key: "method"},
    {label: i18next.t("general:Host"), key: "host"},
    {label: i18next.t("general:Path"), key: "path"},
    {label: i18next.t("general:Client ip"), key: "clientIp"},
    {label: i18next.t("general:User-Agent"), key: "userAgent"},
  ];

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("general:Edit Record")}>
        <Button variant="outline" onClick={() => navigate("/records")}>
          {i18next.t("general:Cancel")}
        </Button>
        <Button onClick={save}>{i18next.t("general:Save")}</Button>
      </PageHeader>

      <Card>
        <CardContent className="divide-y py-0">
          {fields.map(field => (
            <FormRow key={String(field.key)} label={field.label}>
              <Input
                value={String(record[field.key] ?? "")}
                onChange={event => updateField(field.key, event.target.value as never)}
              />
            </FormRow>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
