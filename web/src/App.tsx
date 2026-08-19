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
import {Navigate, Route, Routes, useLocation} from "react-router-dom";
import {useTranslation} from "react-i18next";

import * as AccountBackend from "@/backend/AccountBackend";
import * as Conf from "@/Conf";
import * as Setting from "@/Setting";
import {Footer} from "@/components/layout/Footer";
import {Header} from "@/components/layout/Header";
import {Sidebar} from "@/components/layout/Sidebar";
import {PageSpinner} from "@/components/ui/spinner";
import {TooltipProvider} from "@/components/ui/tooltip";
import AccountPage from "@/pages/AccountPage";
import AgentDashboardPage from "@/pages/AgentDashboardPage";
import AgentDetailPage from "@/pages/AgentDetailPage";
import AgentRecordsPage from "@/pages/AgentRecordsPage";
import AgentSessionsPage from "@/pages/AgentSessionsPage";
import AgentsPage from "@/pages/AgentsPage";
import AuthCallback from "@/pages/AuthCallback";
import CertEditPage from "@/pages/CertEditPage";
import CertListPage from "@/pages/CertListPage";
import ChannelEditPage from "@/pages/ChannelEditPage";
import ChannelListPage from "@/pages/ChannelListPage";
import NodeEditPage from "@/pages/NodeEditPage";
import NodeListPage from "@/pages/NodeListPage";
import RecordEditPage from "@/pages/RecordEditPage";
import RecordListPage from "@/pages/RecordListPage";
import RuleEditPage from "@/pages/RuleEditPage";
import RuleListPage from "@/pages/RuleListPage";
import SigninPage from "@/pages/SigninPage";
import SiteEditPage from "@/pages/SiteEditPage";
import SiteListPage from "@/pages/SiteListPage";
import type {Account} from "@/types";

// The gateway analytics page is the only one that draws charts, and the
// charting runtime is by far the largest dependency here, so it is fetched only
// when that page is opened.
const DashboardPage = React.lazy(() => import("@/pages/DashboardPage"));

Setting.initCasdoorSdk(Conf.AuthConfig);

const collapsedKey = "sidebarCollapsed";

export default function App() {
  // Pages read their labels through i18next.t() at render time rather than the
  // useTranslation() hook, so subscribing the root to "languageChanged" here
  // re-renders the whole tree on a language switch. That lets changeLanguage
  // update every string in place instead of forcing a full window.reload().
  useTranslation();
  // undefined while the account request is in flight, null when signed out.
  const [account, setAccount] = React.useState<Account | null | undefined>(undefined);
  const [collapsed, setCollapsed] = React.useState(
    () => localStorage.getItem(collapsedKey) === "true",
  );
  const [drawerOpen, setDrawerOpen] = React.useState(false);
  const location = useLocation();

  const toggleCollapsed = () => {
    setCollapsed(value => {
      localStorage.setItem(collapsedKey, String(!value));
      return !value;
    });
  };

  const getAccount = React.useCallback(() => {
    AccountBackend.getAccount().then(res => {
      const user = res.data;
      if (user !== null && user !== undefined) {
        user.hostname = res.data2;
        const language = localStorage.getItem("language");
        if (language && language !== Setting.getLanguage()) {
          Setting.setLanguage(language);
        }
        setAccount(user);
      } else {
        setAccount(null);
      }
    });
  }, []);

  React.useEffect(() => {
    // The auth config lives in app.conf, so ask the backend for it before doing
    // anything that depends on whether Casdoor is configured at all.
    AccountBackend.getSigninOptions()
      .then(res => {
        if (res.status === "ok") {
          Conf.setAuthConfig(res.data.authConfig);
          Setting.initCasdoorSdk(Conf.AuthConfig);
        }
        getAccount();
      })
      .catch(() => getAccount());
  }, [getAccount]);

  const signout = () => {
    AccountBackend.signout().then(res => {
      if (res.status === "ok") {
        setAccount(null);
        Setting.showMessage("success", "Successfully signed out, redirected to homepage");
        Setting.goToLink("/");
      } else {
        Setting.showMessage("error", `Signout failed: ${res.msg}`);
      }
    });
  };

  /** Wraps a page that only makes sense for a signed-in user. */
  const requireSignin = (render: (user: Account) => React.ReactNode) => {
    if (account === undefined) {
      return <PageSpinner />;
    }
    if (account === null) {
      sessionStorage.setItem("from", location.pathname);
      return <Navigate to="/signin" replace />;
    }
    return <>{render(account)}</>;
  };

  const redirectHomeIfSignedIn = (element: React.ReactNode) => {
    if (account !== null && account !== undefined) {
      return <Navigate to="/" replace />;
    }
    return <>{element}</>;
  };

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex min-h-screen">
        <Sidebar
          account={account}
          collapsed={collapsed}
          drawerOpen={drawerOpen}
          onDrawerOpenChange={setDrawerOpen}
        />
        {/* min-w-0 keeps a wide table from stretching the whole layout. */}
        <div className="flex min-w-0 flex-1 flex-col">
          <Header
            account={account}
            collapsed={collapsed}
            onToggleCollapsed={toggleCollapsed}
            onOpenDrawer={() => setDrawerOpen(true)}
            onSignout={signout}
          />
          <main className="flex-1">
            <React.Suspense fallback={<PageSpinner />}>
              <Routes>
                <Route path="/callback" element={<AuthCallback />} />
                <Route
                  path="/"
                  element={requireSignin(user =>
                    Setting.isAdminUser(user) ? (
                      <AgentDashboardPage account={user} />
                    ) : (
                      // The dashboard is about the agents on this host, which
                      // only an admin may see, so everyone else lands on the
                      // first page they can actually use.
                      <Navigate to="/sites" replace />
                    ),
                  )}
                />
                <Route path="/signin" element={redirectHomeIfSignedIn(<SigninPage />)} />
                <Route
                  path="/account"
                  element={requireSignin(user => <AccountPage account={user} />)}
                />
                <Route path="/agents" element={requireSignin(user => <AgentsPage account={user} />)} />
                <Route
                  path="/agents/:agentId"
                  element={requireSignin(user => <AgentDetailPage account={user} />)}
                />
                <Route
                  path="/agent-records"
                  element={requireSignin(user => <AgentRecordsPage account={user} />)}
                />
                <Route
                  path="/agent-sessions"
                  element={requireSignin(user => <AgentSessionsPage account={user} />)}
                />
                <Route path="/nodes" element={requireSignin(user => <NodeListPage account={user} />)} />
                <Route path="/nodes/:owner/:nodeName" element={requireSignin(() => <NodeEditPage />)} />
                <Route path="/sites" element={requireSignin(user => <SiteListPage account={user} />)} />
                <Route
                  path="/sites/:owner/:siteName"
                  element={requireSignin(user => <SiteEditPage account={user} />)}
                />
                <Route path="/certs" element={requireSignin(user => <CertListPage account={user} />)} />
                <Route path="/certs/:owner/:certName" element={requireSignin(() => <CertEditPage />)} />
                <Route
                  path="/records"
                  element={requireSignin(user => <RecordListPage account={user} />)}
                />
                <Route path="/records/:owner/:id" element={requireSignin(() => <RecordEditPage />)} />
                <Route path="/rules" element={requireSignin(user => <RuleListPage account={user} />)} />
                <Route path="/rules/:owner/:ruleName" element={requireSignin(() => <RuleEditPage />)} />
                <Route
                  path="/channels"
                  element={requireSignin(user => <ChannelListPage account={user} />)}
                />
                <Route
                  path="/channels/:owner/:channelName"
                  element={requireSignin(() => <ChannelEditPage />)}
                />
                <Route path="/dashboard" element={requireSignin(() => <DashboardPage />)} />
              </Routes>
            </React.Suspense>
          </main>
          <Footer />
        </div>
      </div>
    </TooltipProvider>
  );
}
