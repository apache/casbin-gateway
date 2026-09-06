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
import {CircleCheck, ExternalLink, LogIn} from "lucide-react";
import i18next from "i18next";

import * as ProviderBackend from "@/backend/ProviderBackend";
import {Field} from "@/components/shared/form-dialog";
import {Badge} from "@/components/ui/badge";
import {Button} from "@/components/ui/button";
import type {Provider, ProviderSignin} from "@/types";

/** How often the page asks how the sign-in in the browser is getting on. */
const pollInterval = 1500;

/** Who a provider is signed in as, empty when it is not signed in at all. */
export function signedInAs(provider: Provider) {
  return provider.subscriptionAccount ?? "";
}

/**
 * The sign-in row of a subscription provider, which takes the place of the API
 * key field: the credential is granted in the vendor's own browser flow and
 * kept here, so there is nothing to paste.
 */
export function ProviderLoginField({
  provider,
  vendor,
  onSignedIn,
}: {
  provider: Provider;
  /** The vendor whose subscription this is, as the server names it. */
  vendor: string;
  onSignedIn: (session: ProviderSignin) => void;
}) {
  const [session, setSession] = React.useState<ProviderSignin | null>(null);
  const [starting, setStarting] = React.useState(false);
  const [error, setError] = React.useState("");
  // The poll outlives no dialog: a sign-in nobody finished stops being followed
  // as soon as the form it belongs to is gone.
  const timer = React.useRef<number | undefined>(undefined);

  React.useEffect(() => () => window.clearTimeout(timer.current), []);

  const follow = React.useCallback(
    (id: string) => {
      timer.current = window.setTimeout(() => {
        ProviderBackend.getProviderSignin(id)
          .then(res => {
            if (res.status !== "ok" || !res.data) {
              setError(res.msg ?? "");
              setSession(null);
              return;
            }
            setSession(res.data);
            if (res.data.running) {
              follow(id);
              return;
            }
            if (res.data.ok) {
              onSignedIn(res.data);
            } else {
              setError(res.data.error ?? "");
            }
          })
          .catch(reason => {
            setError(String(reason));
            setSession(null);
          });
      }, pollInterval);
    },
    [onSignedIn],
  );

  const start = () => {
    setStarting(true);
    setError("");
    ProviderBackend.signInProvider(vendor)
      .then(res => {
        if (res.status !== "ok" || !res.data) {
          setError(res.msg ?? "");
          return;
        }
        setSession(res.data);
        // The approval happens on the vendor's own site, in a tab of its own,
        // so the half-filled form behind it is still here afterwards.
        window.open(res.data.url, "_blank", "noopener");
        follow(res.data.id);
      })
      .catch(reason => setError(String(reason)))
      .then(() => setStarting(false));
  };

  const account = session?.ok ? (session.account ?? "") : signedInAs(provider);
  const plan = session?.ok ? (session.plan ?? "") : (provider.subscriptionPlan ?? "");
  const waiting = session?.running === true;

  return (
    <Field
      label={i18next.t("provider:Sign-in")}
      hint={account === "" ? i18next.t("provider:Sign-in hint") : i18next.t("provider:Signed in hint")}
    >
      <div className="grid gap-2">
        <div className="flex flex-wrap items-center gap-3">
          <Button type="button" size="sm" variant="outline" loading={starting || waiting} onClick={start}>
            <LogIn />
            {account === "" ? i18next.t("provider:Sign in") : i18next.t("provider:Sign in again")}
          </Button>
          {account === "" ? null : (
            <Badge variant="success" className="w-fit">
              <CircleCheck />
              {plan === "" ? account : `${account} · ${plan}`}
            </Badge>
          )}
        </div>
        {waiting && session ? (
          <p className="text-muted-foreground text-xs">
            {i18next.t("provider:Finish the sign-in in the browser")}{" "}
            <a
              href={session.url}
              target="_blank"
              rel="noreferrer"
              className="text-primary inline-flex items-center gap-1 hover:underline"
            >
              <ExternalLink className="size-3" />
              {i18next.t("provider:Open the sign-in page")}
            </a>
          </p>
        ) : null}
        {error === "" ? null : <p className="text-destructive text-xs">{error}</p>}
      </div>
    </Field>
  );
}
