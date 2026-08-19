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

import * as NodeBackend from "@/backend/NodeBackend";
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
import type {Node} from "@/types";

const upgradeModes = ["At Any Time", "No Upgrade", "Half A Hour"];

export default function NodeEditPage() {
  const {owner = "", nodeName = ""} = useParams();
  const navigate = useNavigate();
  const [node, setNode] = React.useState<Node | null>(null);

  React.useEffect(() => {
    NodeBackend.getNode(owner, nodeName).then(res => {
      if (res.status === "ok") {
        setNode(res.data);
      } else {
        Setting.showMessage("error", `Failed to get node: ${res.msg}`);
      }
    });
  }, [nodeName, owner]);

  const updateField = <K extends keyof Node>(key: K, value: Node[K]) => {
    setNode(current => (current === null ? current : {...current, [key]: value}));
  };

  const save = () => {
    if (node === null) {
      return;
    }

    NodeBackend.updateNode(node.owner, nodeName, Setting.deepCopy(node))
      .then(res => {
        if (res.status === "error") {
          Setting.showMessage("error", `Failed to save: ${res.msg}`);
        } else {
          Setting.showMessage("success", "Successfully saved");
          navigate(`/nodes/${node.owner}/${node.name}`);
        }
      })
      .catch(error => Setting.showMessage("error", `Failed to connect to server: ${error}`));
  };

  if (node === null) {
    return <PageSpinner />;
  }

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("node:Edit Node")}>
        <Button variant="outline" onClick={() => navigate("/nodes")}>
          {i18next.t("general:Cancel")}
        </Button>
        <Button onClick={save}>{i18next.t("general:Save")}</Button>
      </PageHeader>

      <Card>
        <CardContent className="divide-y py-0">
          <FormRow label={i18next.t("general:Name")}>
            <Input value={node.name} onChange={event => updateField("name", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("general:Display name")}>
            <Input
              value={node.displayName}
              onChange={event => updateField("displayName", event.target.value)}
            />
          </FormRow>
          <FormRow label={i18next.t("general:Tag")}>
            <Input value={node.tag ?? ""} onChange={event => updateField("tag", event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("general:Client IP")}>
            <Input
              value={node.clientIp}
              onChange={event => updateField("clientIp", event.target.value)}
            />
          </FormRow>
          <FormRow label={i18next.t("general:Upgrade mode")}>
            <Select
              value={node.upgradeMode}
              onValueChange={value => updateField("upgradeMode", value)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {upgradeModes.map(mode => (
                  <SelectItem key={mode} value={mode}>
                    {i18next.t(`general:${mode}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormRow>
        </CardContent>
      </Card>
    </div>
  );
}
