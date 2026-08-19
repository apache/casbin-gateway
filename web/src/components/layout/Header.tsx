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

import {Link, useNavigate} from "react-router-dom";
import {
  LogOut,
  Menu as MenuIcon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
} from "lucide-react";
import i18next from "i18next";
import {useTranslation} from "react-i18next";

import * as Setting from "@/Setting";
import type {Account} from "@/types";
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar";
import {Button} from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {LanguageSelect} from "@/components/layout/LanguageSelect";

function AccountMenu({
  account,
  onSignout,
}: {
  account: Account | null | undefined;
  onSignout: () => void;
}) {
  const navigate = useNavigate();

  if (account === undefined) {
    return null;
  }
  if (account === null) {
    return (
      <Button asChild size="sm">
        <a href={Setting.getSigninUrl() || "/signin"}>{i18next.t("account:Sign In")}</a>
      </Button>
    );
  }

  const openProfile = () => {
    const profileUrl = Setting.getMyProfileUrl(account);
    if (profileUrl === "") {
      navigate("/account");
    } else {
      Setting.openLink(profileUrl);
    }
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button className="flex items-center gap-2 rounded-md px-1.5 py-1 transition-colors hover:bg-accent">
          <Avatar className="h-8 w-8 shrink-0">
            {account.avatar ? <AvatarImage src={account.avatar} alt={account.name} /> : null}
            <AvatarFallback style={{backgroundColor: Setting.getAvatarColor(account.name)}}>
              <span className="text-white">
                {Setting.getShortName(account.name).slice(0, 2).toUpperCase()}
              </span>
            </AvatarFallback>
          </Avatar>
          <span className="hidden max-w-[140px] truncate text-sm font-medium sm:inline">
            {Setting.getShortName(account.displayName || account.name)}
          </span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-48">
        <DropdownMenuItem onClick={openProfile}>
          <Settings />
          {i18next.t("account:My Account")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={onSignout}>
          <LogOut />
          {i18next.t("account:Sign Out")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * The top bar of the content column: navigation controls on the left, and
 * everything about the viewer themselves on the right. It sticks so the account
 * menu stays reachable on a long list page.
 */
export function Header({
  account,
  collapsed,
  onToggleCollapsed,
  onOpenDrawer,
  onSignout,
}: {
  account: Account | null | undefined;
  collapsed: boolean;
  onToggleCollapsed: () => void;
  onOpenDrawer: () => void;
  onSignout: () => void;
}) {
  // The labels are read eagerly through i18next.t, so the header has to
  // re-render when the language changes.
  useTranslation();

  return (
    <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center justify-between gap-2 border-b bg-background/95 px-3 backdrop-blur">
      <div className="flex min-w-0 items-center gap-2">
        <Button
          variant="ghost"
          size="icon"
          onClick={onToggleCollapsed}
          aria-label={i18next.t("general:Toggle sidebar")}
          className="hidden md:inline-flex"
        >
          {collapsed ? (
            <PanelLeftOpen className="h-5 w-5" />
          ) : (
            <PanelLeftClose className="h-5 w-5" />
          )}
        </Button>

        <Button variant="ghost" size="icon" aria-label="Menu" onClick={onOpenDrawer} className="md:hidden">
          <MenuIcon className="h-5 w-5" />
        </Button>
        {/* The sidebar, which carries the logo on desktop, is hidden here. */}
        <Link to="/" className="flex items-center md:hidden">
          <img
            src={`${Setting.StaticBaseUrl}/img/logo_384x96.png`}
            alt="Casbin Gateway"
            className="h-6 w-auto"
          />
        </Link>
      </div>

      <div className="flex items-center gap-1">
        <LanguageSelect />
        <AccountMenu account={account} onSignout={onSignout} />
      </div>
    </header>
  );
}
