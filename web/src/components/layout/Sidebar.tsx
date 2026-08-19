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
import {Link, useLocation} from "react-router-dom";
import {
  Bot,
  ChartColumn,
  ChevronDown,
  ChevronRight,
  FileSearch,
  Globe,
  LayoutDashboard,
  ListFilter,
  MessageSquare,
  Plug,
  ScrollText,
  Server,
  Settings,
  ShieldCheck,
  X,
} from "lucide-react";
import i18next from "i18next";
import {useTranslation} from "react-i18next";

import * as Setting from "@/Setting";
import {cn} from "@/lib/utils";
import type {Account} from "@/types";
import {Button} from "@/components/ui/button";
import {Tooltip} from "@/components/ui/tooltip";

const advancedOpenKey = "sidebarAdvancedOpen";

interface MenuEntry {
  key: string;
  label: string;
  icon: React.ReactNode;
  adminOnly?: boolean;
}

/** The agent tools this product is built around, in the order they are used. */
function getMainEntries(): MenuEntry[] {
  return [
    {key: "/", label: i18next.t("general:Dashboard"), icon: <LayoutDashboard />},
    {key: "/agents", label: i18next.t("agent:Agents"), icon: <Bot />, adminOnly: true},
    {
      key: "/agent-sessions",
      label: i18next.t("agent:Agent Sessions"),
      icon: <MessageSquare />,
      adminOnly: true,
    },
    {
      key: "/agent-records",
      label: i18next.t("agent:Agent Records"),
      icon: <FileSearch />,
      adminOnly: true,
    },
    {key: "/channels", label: i18next.t("channel:Channels"), icon: <Plug />},
  ];
}

/**
 * The reverse-proxy gateway pages. They are a full feature set, but an
 * individual running Gateway to manage the agents on their own machine never
 * needs them, so they live behind one collapsed group instead of competing for
 * attention with the agent pages above.
 */
function getAdvancedEntries(): MenuEntry[] {
  return [
    {key: "/sites", label: i18next.t("general:Sites"), icon: <Globe />},
    {key: "/certs", label: i18next.t("general:Certs"), icon: <ShieldCheck />},
    {key: "/rules", label: i18next.t("general:Rules"), icon: <ListFilter />},
    {key: "/nodes", label: i18next.t("general:Nodes"), icon: <Server />},
    {key: "/records", label: i18next.t("general:Records"), icon: <ScrollText />},
    {key: "/dashboard", label: i18next.t("general:Gateway Analytics"), icon: <ChartColumn />},
  ];
}

/**
 * The dashboard is at "/", which every other path also starts with, so it only
 * counts as selected on an exact match.
 */
function isSelected(pathname: string, key: string) {
  return key === "/" ? pathname === "/" : pathname.startsWith(key);
}

function readFlag(key: string, fallback: boolean) {
  const stored = localStorage.getItem(key);
  return stored === null ? fallback : stored === "true";
}

export function Sidebar({
  account,
  collapsed,
  drawerOpen,
  onDrawerOpenChange,
}: {
  account: Account | null | undefined;
  collapsed: boolean;
  drawerOpen: boolean;
  onDrawerOpenChange: (open: boolean) => void;
}) {
  // Subscribing to the language keeps the menu labels, which are read eagerly
  // through i18next.t, re-rendering when the language changes.
  useTranslation();
  const location = useLocation();
  const [advancedOpen, setAdvancedOpen] = React.useState(() => readFlag(advancedOpenKey, false));

  const isAdmin = Setting.isAdminUser(account);
  const mainEntries = getMainEntries().filter(entry => !entry.adminOnly || isAdmin);
  const advancedEntries = getAdvancedEntries().filter(entry => !entry.adminOnly || isAdmin);

  // The drawer covers the page, so leaving it open across a navigation would
  // hide whatever the viewer just asked for.
  React.useEffect(() => {
    onDrawerOpenChange(false);
  }, [location.pathname, onDrawerOpenChange]);

  const toggleAdvanced = () => {
    setAdvancedOpen(value => {
      localStorage.setItem(advancedOpenKey, String(!value));
      return !value;
    });
  };

  /** `iconOnly` is the collapsed desktop rail; the drawer is always full width. */
  const renderEntry = (entry: MenuEntry, iconOnly: boolean, indented = false) => {
    const link = (
      <Link
        key={entry.key}
        to={entry.key}
        className={cn(
          "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
          iconOnly && "justify-center px-0",
          indented && !iconOnly && "ml-3",
          isSelected(location.pathname, entry.key)
            ? "bg-primary/10 text-primary"
            : "text-muted-foreground hover:bg-accent hover:text-foreground",
        )}
      >
        <span className="[&>svg]:h-[18px] [&>svg]:w-[18px]">{entry.icon}</span>
        {iconOnly ? null : <span className="truncate">{entry.label}</span>}
      </Link>
    );

    if (!iconOnly) {
      return link;
    }
    return (
      <Tooltip key={entry.key} title={entry.label} side="right">
        {link}
      </Tooltip>
    );
  };

  const renderNav = (iconOnly: boolean) => (
    <nav className="flex flex-1 flex-col gap-1 overflow-y-auto p-2">
      {mainEntries.map(entry => renderEntry(entry, iconOnly))}

      {advancedEntries.length > 0 ? (
        <div className="mt-2 border-t pt-2">
          {iconOnly ? (
            <Tooltip title={i18next.t("general:Advanced")} side="right">
              <button
                onClick={toggleAdvanced}
                className="flex w-full items-center justify-center rounded-md px-0 py-2 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
                aria-label={i18next.t("general:Advanced")}
              >
                <Settings className="h-[18px] w-[18px]" />
              </button>
            </Tooltip>
          ) : (
            <button
              onClick={toggleAdvanced}
              className="flex w-full items-center gap-1 rounded-md px-3 py-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground transition-colors hover:text-foreground"
            >
              {advancedOpen ? (
                <ChevronDown className="h-3.5 w-3.5" />
              ) : (
                <ChevronRight className="h-3.5 w-3.5" />
              )}
              {i18next.t("general:Advanced")}
            </button>
          )}
          {advancedOpen
            ? advancedEntries.map(entry => renderEntry(entry, iconOnly, true))
            : null}
        </div>
      ) : null}
    </nav>
  );

  const renderPanel = (iconOnly: boolean) => (
    <>
      <div
        className={cn(
          "flex h-14 shrink-0 items-center gap-2 border-b px-3",
          iconOnly && "justify-center px-0",
        )}
      >
        <Link to="/" className="flex min-w-0 items-center">
          <img
            src={`${Setting.StaticBaseUrl}/img/logo_384x96.png`}
            alt="Casbin Gateway"
            className={cn("w-auto", iconOnly ? "h-5" : "h-6")}
          />
        </Link>
      </div>

      {renderNav(iconOnly)}
    </>
  );

  return (
    <>
      <aside
        className={cn(
          "sticky top-0 hidden h-screen shrink-0 flex-col border-r bg-background md:flex",
          collapsed ? "w-16" : "w-60",
        )}
      >
        {renderPanel(collapsed)}
      </aside>

      {drawerOpen ? (
        <div className="fixed inset-0 z-50 md:hidden">
          <div
            className="absolute inset-0 bg-black/50"
            onClick={() => onDrawerOpenChange(false)}
            aria-hidden
          />
          <div className="absolute inset-y-0 left-0 flex w-64 flex-col border-r bg-background">
            <Button
              variant="ghost"
              size="icon"
              aria-label="Close"
              className="absolute right-2 top-2 z-10"
              onClick={() => onDrawerOpenChange(false)}
            >
              <X className="h-5 w-5" />
            </Button>
            {renderPanel(false)}
          </div>
        </div>
      ) : null}
    </>
  );
}
