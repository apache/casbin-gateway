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
import i18next from "i18next";

import * as AccountBackend from "@/backend/AccountBackend";
import * as Conf from "@/Conf";
import * as Setting from "@/Setting";
import {findGroupOf, selectedKeyOf} from "@/nav";
import {AppHeader} from "@/components/shared/app-header";
import {AppSidebar, persistOpenKeys, readSavedOpenKeys, useIsDesktop} from "@/components/shared/app-sidebar";
import {CommandPalette, useCommandPalette} from "@/components/shared/command-palette";
import {Loading} from "@/components/shared/loading";
import {Logo} from "@/components/shared/logo";
import {TooltipProvider} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import AccountPage from "@/pages/AccountPage";
import AgentConfigsPage from "@/pages/AgentConfigsPage";
import AgentDetailPage from "@/pages/AgentDetailPage";
import AgentRecordsPage from "@/pages/AgentRecordsPage";
import AgentSessionPage from "@/pages/AgentSessionPage";
import AgentSessionsPage from "@/pages/AgentSessionsPage";
import AgentVersionsPage from "@/pages/AgentVersionsPage";
import AgentsPage from "@/pages/AgentsPage";
import AuthCallback from "@/pages/AuthCallback";
import AuthenticityPage from "@/pages/AuthenticityPage";
import ConnectionsPage from "@/pages/ConnectionsPage";
import HomePage from "@/pages/HomePage";
import ImportLinkPage from "@/pages/ImportLinkPage";
import PermissionsPage from "@/pages/PermissionsPage";
import ProviderEditPage from "@/pages/ProviderEditPage";
import ProviderListPage from "@/pages/ProviderListPage";
import LlmRecordsPage from "@/pages/LlmRecordsPage";
import ModelRoutesPage from "@/pages/ModelRoutesPage";
import UsagePage from "@/pages/UsagePage";
import PricingPage from "@/pages/PricingPage";
import SettingPage from "@/pages/SettingPage";
import SigninPage from "@/pages/SigninPage";
import type {Account, Palette, ThemeAlgorithm} from "@/types";

Setting.initCasdoorSdk(Conf.AuthConfig);

const collapsedKey = "siderCollapsed";

export default function App() {
  // Pages call i18next.t() directly, so re-render the whole tree on a language change.
  useTranslation();
  // undefined while the account request is in flight, null when signed out.
  const [account, setAccount] = React.useState<Account | null | undefined>(undefined);
  const [themeAlgorithm, setThemeAlgorithm] = React.useState<ThemeAlgorithm>(() => {
    const stored = Setting.readThemeAlgorithm();
    // Applied before the first paint so a dark-mode reload never flashes the
    // light palette.
    Setting.applyThemeAlgorithm(stored);
    return stored;
  });
  const [colorPalette, setColorPalette] = React.useState<Palette>(() => {
    const stored = Setting.readPalette();
    Setting.applyPalette(stored);
    return stored;
  });
  const [collapsed, setCollapsed] = React.useState(() => localStorage.getItem(collapsedKey) === "true");
  // Below md the rail is a drawer, so the same header button opens it instead
  // of narrowing a column that is not on screen.
  const [mobileNavOpen, setMobileNavOpen] = React.useState(false);
  const isDesktop = useIsDesktop();
  const location = useLocation();
  const palette = useCommandPalette();

  const selectedKey = selectedKeyOf(location.pathname, location.search, location.hash);
  const wasCollapsedRef = React.useRef(false);
  const [openKeys, setOpenKeys] = React.useState<string[]>(() => {
    if (localStorage.getItem(collapsedKey) === "true") {
      return [];
    }
    const saved = new Set(readSavedOpenKeys());
    const group = findGroupOf(selectedKey);
    if (group) {
      saved.add(group.key);
    }
    return [...saved];
  });

  // Navigating into a collapsed group opens it, and expanding the rail restores
  // whatever the reader last had open rather than the defaults.
  React.useEffect(() => {
    if (collapsed) {
      wasCollapsedRef.current = true;
      setOpenKeys([]);
      return;
    }
    const justExpanded = wasCollapsedRef.current;
    wasCollapsedRef.current = false;
    const group = findGroupOf(selectedKey);

    setOpenKeys(previous => {
      if (justExpanded) {
        const restored = new Set(readSavedOpenKeys());
        if (group) {
          restored.add(group.key);
        }
        return [...restored];
      }
      if (group && !previous.includes(group.key)) {
        return [...previous, group.key];
      }
      return previous;
    });
  }, [selectedKey, collapsed]);

  React.useEffect(() => {
    if (!collapsed) {
      persistOpenKeys(openKeys);
    }
  }, [openKeys, collapsed]);

  const toggleSidebar = () => {
    if (!isDesktop) {
      setMobileNavOpen(value => !value);
      return;
    }
    setCollapsed(value => {
      localStorage.setItem(collapsedKey, String(!value));
      return !value;
    });
  };

  const changeTheme = (next: ThemeAlgorithm) => {
    setThemeAlgorithm(next);
    Setting.saveThemeAlgorithm(next);
  };

  const changeColorPalette = (next: Palette) => {
    setColorPalette(next);
    Setting.savePalette(next);
  };

  const getAccount = React.useCallback(() => {
    AccountBackend.getAccount()
      .then(res => {
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
      })
      // An unreachable backend is not a signed-in session: without this the
      // app waits on account forever behind the full-page spinner.
      .catch(() => setAccount(null));
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
        Setting.showMessage("success", i18next.t("general:Signed out successfully"));
        Setting.goToLink("/");
      } else {
        Setting.showMessage("error", `${i18next.t("general:Failed to sign out")}: ${res.msg}`);
      }
    });
  };

  /** Wraps a page that only makes sense for a signed-in user. */
  const requireSignin = (render: (user: Account) => React.ReactNode) => {
    if (account === undefined) {
      return <Loading type="page" />;
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

  // The sign-in screen is its own full-page layout: no rail, no header.
  if (location.pathname === "/signin" || location.pathname === "/callback") {
    return (
      <TooltipProvider>
        <Routes>
          <Route path="/callback" element={<AuthCallback />} />
          <Route path="/signin" element={redirectHomeIfSignedIn(<SigninPage />)} />
        </Routes>
      </TooltipProvider>
    );
  }

  return (
    <TooltipProvider>
      <div className="bg-muted/30 min-h-screen">
        <AppSidebar
          collapsed={collapsed}
          selectedKey={selectedKey}
          openKeys={openKeys}
          onOpenKeysChange={setOpenKeys}
          isAdmin={Setting.isAdminUser(account)}
          mobileOpen={mobileNavOpen}
          onMobileOpenChange={setMobileNavOpen}
        />

        <div
          className={cn(
            "flex min-h-screen min-w-0 flex-col transition-[margin] duration-200",
            collapsed ? "md:ml-16" : "md:ml-64",
          )}
        >
          <AppHeader
            collapsed={collapsed}
            onToggleSidebar={toggleSidebar}
            uri={location.pathname}
            account={account}
            isAdmin={Setting.isAdminUser(account)}
            themeAlgorithm={themeAlgorithm}
            onThemeChange={changeTheme}
            colorPalette={colorPalette}
            onColorPaletteChange={changeColorPalette}
            onSignout={signout}
            onOpenPalette={() => palette.setOpen(true)}
          />

          {account ? (
            <CommandPalette
              open={palette.open}
              onOpenChange={palette.setOpen}
              account={account}
              isAdmin={Setting.isAdminUser(account)}
            />
          ) : null}

          {/* min-w-0 keeps a wide table from stretching the whole layout. */}
          <main className="flex min-w-0 flex-1 flex-col">
            <React.Suspense fallback={<Loading type="page" />}>
              <Routes>
                <Route
                  path="/"
                  element={requireSignin(user =>
                    Setting.isAdminUser(user) ? (
                      <HomePage account={user} />
                    ) : (
                      // The home screen is about the agents on this host, which
                      // only an admin may see, so everyone else lands on the
                      // first page they can actually use.
                      <Navigate to="/providers" replace />
                    ),
                  )}
                />
                <Route path="/account" element={requireSignin(user => <AccountPage account={user} />)} />
                <Route path="/agents" element={requireSignin(user => <AgentsPage account={user} />)} />
                <Route
                  path="/agent-versions"
                  element={requireSignin(user => <AgentVersionsPage account={user} />)}
                />
                <Route
                  path="/agents/:agentId"
                  element={requireSignin(user => <AgentDetailPage account={user} />)}
                />
                <Route
                  path="/agent-configs"
                  element={requireSignin(user => <AgentConfigsPage account={user} />)}
                />
                <Route
                  path="/agent-records"
                  element={requireSignin(user => <AgentRecordsPage account={user} />)}
                />
                <Route
                  path="/agent-sessions"
                  element={requireSignin(user => <AgentSessionsPage account={user} />)}
                />
                <Route
                  path="/agent-sessions/:sessionKey"
                  element={requireSignin(user => <AgentSessionPage account={user} />)}
                />
                <Route
                  path="/connections"
                  element={requireSignin(user => <ConnectionsPage account={user} />)}
                />
                <Route path="/import" element={requireSignin(() => <ImportLinkPage />)} />
                <Route path="/settings" element={requireSignin(user => <SettingPage account={user} />)} />
                <Route
                  path="/permissions"
                  element={requireSignin(user => <PermissionsPage account={user} />)}
                />
                <Route path="/providers" element={requireSignin(user => <ProviderListPage account={user} />)} />
                <Route
                  path="/authenticity"
                  element={requireSignin(user => <AuthenticityPage account={user} />)}
                />
                <Route
                  path="/providers/:owner/:providerName"
                  element={requireSignin(() => <ProviderEditPage />)}
                />
                <Route
                  path="/llm-records"
                  element={requireSignin(user => <LlmRecordsPage account={user} />)}
                />
                <Route path="/usage" element={requireSignin(user => <UsagePage account={user} />)} />
                <Route path="/pricing" element={requireSignin(user => <PricingPage account={user} />)} />
                <Route
                  path="/model-routes"
                  element={requireSignin(user => <ModelRoutesPage account={user} />)}
                />
              </Routes>
            </React.Suspense>
          </main>

          <footer className="flex items-center justify-center border-t py-5">
            <a target="_blank" rel="noreferrer" href="https://github.com/apache/casbin-gateway">
              <Logo className="h-[30px] w-auto" />
            </a>
          </footer>
        </div>
      </div>
    </TooltipProvider>
  );
}
