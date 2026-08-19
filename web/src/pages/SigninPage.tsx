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
import {Result} from "@/components/Result";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {PasswordInput} from "@/components/ui/password-input";
import {PageSpinner} from "@/components/ui/spinner";

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
    return <PageSpinner tip="Signing in..." />;
  }

  if (!showSignin) {
    return (
      <Result
        status="warning"
        title="Login Error"
        subTitle={errorMessage || i18next.t("account:Sign in is unavailable")}
      />
    );
  }

  return (
    <div className="flex min-h-[60vh] items-center justify-center px-4">
      <form onSubmit={onSubmit} className="w-full max-w-sm space-y-4">
        <div className="mb-8 text-center text-2xl font-semibold">{i18next.t("account:Sign In")}</div>
        <div className="relative">
          <User className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-11 pl-9"
            required
            value={username}
            placeholder={i18next.t("account:Username")}
            onChange={event => setUsername(event.target.value)}
          />
        </div>
        <div className="relative">
          <Lock className="absolute left-3 top-1/2 z-10 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <PasswordInput
            className="h-11 pl-9"
            required
            autoFocus
            value={password}
            placeholder={i18next.t("general:Password")}
            onChange={event => setPassword(event.target.value)}
          />
        </div>
        <Button type="submit" className="h-11 w-full">
          {i18next.t("account:Sign In")}
        </Button>
      </form>
    </div>
  );
}
