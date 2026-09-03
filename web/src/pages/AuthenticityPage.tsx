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

import * as ProviderBackend from "@/backend/ProviderBackend";
import * as Setting from "@/Setting";
import {AuthenticityOverview} from "@/components/AuthenticityOverview";
import {ProbeCaseList} from "@/components/ProbeCaseList";
import {ProviderAuditPanel} from "@/components/ProviderAuditPanel";
import {UnauthorizedResult} from "@/components/shared/misc";
import {PageContainer, PageHeader} from "@/components/shared/page-header";
import {Tabs, TabsList, TabsTrigger} from "@/components/ui/tabs";
import type {Account, Provider} from "@/types";

type AuthenticityTab = "report" | "cases";

/**
 * Whether the APIs behind the keys on this machine are what they are sold as.
 * The report is the score every provider holds and the answers it was drawn
 * from; the suite beside it is the questions themselves, which are published
 * and editable because a score whose method is secret is not evidence.
 */
export default function AuthenticityPage({account}: {account: Account}) {
  const isAdmin = Setting.isAdminUser(account);
  const [tab, setTab] = React.useState<AuthenticityTab>("report");
  const [providers, setProviders] = React.useState<Provider[]>([]);

  React.useEffect(() => {
    if (!isAdmin) {
      return;
    }
    ProviderBackend.getProviders(account.name)
      .then(res => setProviders(res.status === "ok" ? (res.data ?? []) : []))
      .catch(() => undefined);
  }, [isAdmin, account.name]);

  if (!isAdmin) {
    return <UnauthorizedResult />;
  }

  return (
    <PageContainer>
      <PageHeader
        title={i18next.t("audit:Authenticity")}
        description={i18next.t("audit:Authenticity description")}
      />

      <AuthenticityOverview providers={providers} showLink={false} />

      <Tabs value={tab} onValueChange={value => setTab(value as AuthenticityTab)}>
        <TabsList>
          <TabsTrigger value="report">{i18next.t("audit:Report")}</TabsTrigger>
          <TabsTrigger value="cases">{i18next.t("audit:Test cases")}</TabsTrigger>
        </TabsList>
      </Tabs>

      {tab === "report" ? <ProviderAuditPanel owner={account.name} /> : <ProbeCaseList />}
    </PageContainer>
  );
}
