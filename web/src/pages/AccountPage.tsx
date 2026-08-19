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

import * as AccountBackend from "@/backend/AccountBackend";
import * as Setting from "@/Setting";
import {FormRow, PageHeader} from "@/components/FormRow";
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar";
import {Button} from "@/components/ui/button";
import {Card, CardContent, CardHeader, CardTitle} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";
import {PasswordInput} from "@/components/ui/password-input";
import type {Account} from "@/types";

// Profile and password editing for the built-in login. Casdoor-backed accounts
// are managed in Casdoor itself, so this page is only reachable without Casdoor.
export default function AccountPage({account}: {account: Account}) {
  const [displayName, setDisplayName] = React.useState(account.displayName ?? "");
  const [avatar, setAvatar] = React.useState(account.avatar ?? "");
  const [passwordOpen, setPasswordOpen] = React.useState(false);
  const [currentPassword, setCurrentPassword] = React.useState("");
  const [newPassword, setNewPassword] = React.useState("");

  const save = () => {
    AccountBackend.updateAccount({displayName: displayName, avatar: avatar})
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", "Successfully saved");
          window.location.reload();
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(error => Setting.showMessage("error", error.message));
  };

  const setPassword = () => {
    AccountBackend.updateAccount({
      displayName: displayName,
      avatar: avatar,
      currentPassword: currentPassword,
      newPassword: newPassword,
    })
      .then(res => {
        if (res.status === "ok") {
          Setting.showMessage("success", "Successfully saved");
          setPasswordOpen(false);
          setCurrentPassword("");
          setNewPassword("");
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(error => Setting.showMessage("error", error.message));
  };

  const isImageUrl =
    avatar.startsWith("http://") || avatar.startsWith("https://") || avatar.startsWith("data:image/");

  return (
    <div className="p-4 md:p-6">
      <PageHeader title={i18next.t("account:My Account")}>
        <Button onClick={save}>{i18next.t("general:Save")}</Button>
      </PageHeader>

      <Card className="mb-4">
        <CardHeader className="border-b py-3">
          <CardTitle className="text-sm">{i18next.t("account:Profile")}</CardTitle>
        </CardHeader>
        <CardContent className="divide-y py-0">
          <FormRow label={i18next.t("general:Name")}>
            <Input value={account.name} disabled />
          </FormRow>
          <FormRow label={i18next.t("general:Display name")}>
            <Input value={displayName} onChange={event => setDisplayName(event.target.value)} />
          </FormRow>
          <FormRow label={i18next.t("account:Avatar")}>
            <div className="flex items-center gap-3">
              <Avatar className="h-16 w-16">
                {isImageUrl ? <AvatarImage src={avatar} alt={account.name} /> : null}
                <AvatarFallback style={{backgroundColor: Setting.getAvatarColor(account.name)}}>
                  <span className="text-white">
                    {Setting.getShortName(account.name).slice(0, 2).toUpperCase()}
                  </span>
                </AvatarFallback>
              </Avatar>
              <Input
                value={avatar}
                placeholder={i18next.t("account:Avatar image URL, optional")}
                onChange={event => setAvatar(event.target.value)}
              />
            </div>
          </FormRow>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="border-b py-3">
          <CardTitle className="text-sm">{i18next.t("general:Password")}</CardTitle>
        </CardHeader>
        <CardContent className="pt-4">
          <Button variant="outline" onClick={() => setPasswordOpen(true)}>
            {i18next.t("account:Modify password...")}
          </Button>
        </CardContent>
      </Card>

      <Dialog open={passwordOpen} onOpenChange={setPasswordOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{i18next.t("account:Modify password")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>{i18next.t("account:Old Password")}</Label>
              <PasswordInput
                value={currentPassword}
                onChange={event => setCurrentPassword(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>{i18next.t("account:New Password")}</Label>
              <PasswordInput
                value={newPassword}
                onChange={event => setNewPassword(event.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPasswordOpen(false)}>
              {i18next.t("general:Cancel")}
            </Button>
            <Button onClick={setPassword}>{i18next.t("account:Set Password")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
