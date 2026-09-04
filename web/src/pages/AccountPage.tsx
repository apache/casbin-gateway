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
import {Field, FormDialog} from "@/components/shared/form-dialog";
import {PageContainer, PageHeader, Section} from "@/components/shared/page-header";
import {PasswordInput} from "@/components/shared/password-input";
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
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
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
          // A full load, so the nav picks up the new name and avatar.
          Setting.goToLink("/");
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
          Setting.showMessage("success", i18next.t("general:Successfully saved"));
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
    <PageContainer>
      <PageHeader
        title={i18next.t("account:My Account")}
        description={account.name}
        actions={<Button onClick={save}>{i18next.t("general:Save")}</Button>}
      />

      <Section title={i18next.t("account:Profile")} columns={2}>
        <Field label={i18next.t("general:Name")} htmlFor="account-name">
          <Input id="account-name" value={account.name} disabled />
        </Field>
        <Field label={i18next.t("general:Display name")} htmlFor="account-display-name">
          <Input
            id="account-display-name"
            value={displayName}
            onChange={event => setDisplayName(event.target.value)}
          />
        </Field>
        <Field label={i18next.t("account:Avatar")} htmlFor="account-avatar" className="md:col-span-2">
          <div className="flex items-center gap-3">
            <Avatar className="size-16">
              {isImageUrl ? <AvatarImage src={avatar} alt={account.name} /> : null}
              <AvatarFallback style={{backgroundColor: Setting.getAvatarColor(account.name), color: "#fff"}}>
                {Setting.getShortName(account.name).slice(0, 2).toUpperCase()}
              </AvatarFallback>
            </Avatar>
            <Input
              id="account-avatar"
              value={avatar}
              placeholder={i18next.t("account:Avatar image URL, optional")}
              onChange={event => setAvatar(event.target.value)}
            />
          </div>
        </Field>
      </Section>

      <Section title={i18next.t("general:Password")} columns={1}>
        <div>
          <Button variant="outline" onClick={() => setPasswordOpen(true)}>
            {i18next.t("account:Modify password...")}
          </Button>
        </div>
      </Section>

      <FormDialog
        open={passwordOpen}
        onOpenChange={setPasswordOpen}
        title={i18next.t("account:Modify password")}
        submitText={i18next.t("account:Set Password")}
        onSubmit={setPassword}
      >
        <Field label={i18next.t("account:Old Password")} htmlFor="account-old-password">
          <PasswordInput
            id="account-old-password"
            value={currentPassword}
            onChange={event => setCurrentPassword(event.target.value)}
          />
        </Field>
        <Field label={i18next.t("account:New Password")} htmlFor="account-new-password">
          <PasswordInput
            id="account-new-password"
            value={newPassword}
            onChange={event => setNewPassword(event.target.value)}
          />
        </Field>
      </FormDialog>
    </PageContainer>
  );
}
