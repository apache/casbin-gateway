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
import {Lock, User} from "lucide-react";
import i18next from "i18next";

import * as AccountBackend from "@/backend/AccountBackend";
import * as Conf from "@/Conf";
import * as Setting from "@/Setting";
import {Loading} from "@/components/shared/loading";
import {ResultScreen} from "@/components/shared/misc";
import {PasswordInput} from "@/components/shared/password-input";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Label} from "@/components/ui/label";

// Signing in goes one of two ways: redirect to Casdoor when app.conf configures
// it, otherwise show the built-in username/password form.
export default function SigninPage() {
  const [loading, setLoading] = React.useState(true);
  const [showSignin, setShowSignin] = React.useState(false);
  const [errorMessage, setErrorMessage] = React.useState("");
  const [username, setUsername] = React.useState("admin");
  const [password, setPassword] = React.useState("");

  React.useEffect(() => {
    AccountBackend.getSigninOptions()
      .then(res => {
        if (res.status === "ok" && res.data?.casdoorAvailable) {
          // Do not wait for App to publish the config: this page may well have
          // mounted before that request came back.
          Conf.setAuthConfig(res.data.authConfig);
          Setting.initCasdoorSdk(Conf.AuthConfig);
          window.location.replace(Setting.getSigninUrl());
          return;
        }

        setLoading(false);
        setShowSignin(res.status === "ok" && res.data?.signinAvailable === true);
        setErrorMessage(res.status === "ok" ? "" : res.msg);
        if (res.status === "ok" && res.data?.autoSignin === true) {
          setPassword("123");
        }
      })
      .catch(error => {
        setLoading(false);
        setShowSignin(false);
        setErrorMessage(error.message);
      });
  }, []);

  const onSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    AccountBackend.signinWithPassword(username, password)
      .then(res => {
        if (res.status === "ok") {
          const from = sessionStorage.getItem("from") || "/";
          sessionStorage.removeItem("from");
          window.location.href = from;
        } else {
          Setting.showMessage("error", res.msg);
        }
      })
      .catch(error => Setting.showMessage("error", error.message));
  };

  if (loading) {
    return <Loading type="page" tip={i18next.t("general:Signing in...")} />;
  }

  if (!showSignin) {
    return (
      <div className="bg-muted/30 flex min-h-screen items-center justify-center">
        <ResultScreen
          status="!"
          title={i18next.t("general:Login Error")}
          subTitle={errorMessage || i18next.t("account:Sign in is unavailable")}
        />
      </div>
    );
  }

  return (
    <div className="bg-muted/30 flex min-h-screen items-center justify-center p-6">
      <div className="w-full max-w-sm">
        <div className="mb-9 flex justify-center">
          <img
            src={`${Setting.StaticBaseUrl}/img/logo_384x96.png`}
            alt="Casbin Gateway"
            className="h-10 w-auto max-w-[260px] object-contain dark:invert dark:hue-rotate-180"
          />
        </div>

        <form onSubmit={onSubmit} className="bg-card grid gap-4 rounded-xl border p-6 shadow-sm">
          <div className="grid gap-2">
            <Label htmlFor="signin-username">{i18next.t("account:Username")}</Label>
            <div className="relative">
              <User className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2" />
              <Input
                id="signin-username"
                className="pl-9"
                required
                autoComplete="username"
                value={username}
                placeholder={i18next.t("account:Username")}
                onChange={event => setUsername(event.target.value)}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="signin-password">{i18next.t("general:Password")}</Label>
            <div className="relative">
              <Lock className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2" />
              <PasswordInput
                id="signin-password"
                className="pl-9"
                required
                autoFocus
                autoComplete="current-password"
                value={password}
                placeholder={i18next.t("general:Password")}
                onChange={event => setPassword(event.target.value)}
              />
            </div>
          </div>

          <Button type="submit" className="mt-2 w-full">
            {i18next.t("account:Sign In")}
          </Button>
        </form>
      </div>
    </div>
  );
}
