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
import {useNavigate} from "react-router-dom";
import {
  Check,
  ChevronDown,
  Languages,
  LogOut,
  Moon,
  PanelLeft,
  PanelLeftClose,
  Palette as PaletteIcon,
  Settings,
  Sun,
} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import type {Account, Palette, ThemeAlgorithm} from "@/types";
import {Avatar, AvatarFallback, AvatarImage} from "@/components/ui/avatar";
import {Button} from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {SimpleTooltip} from "@/components/ui/tooltip";
import {cn} from "@/lib/utils";
import {BreadcrumbBar} from "@/components/shared/breadcrumb-bar";
import {CommandPaletteTrigger} from "@/components/shared/command-palette";
import {VersionPanel} from "@/components/shared/version-panel";

function ThemeToggle({
  themeAlgorithm,
  onChange,
}: {
  themeAlgorithm: ThemeAlgorithm;
  onChange: (next: ThemeAlgorithm) => void;
}) {
  const isDark = Setting.isDarkTheme(themeAlgorithm);
  return (
    <SimpleTooltip title={isDark ? i18next.t("general:Light") : i18next.t("general:Dark")}>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={() => onChange(isDark ? ["default"] : ["dark"])}
        aria-label="Toggle theme"
      >
        {isDark ? <Sun className="size-4" /> : <Moon className="size-4" />}
      </Button>
    </SimpleTooltip>
  );
}

// The swatches are literal rather than var(--primary): the menu has to show the
// palette you are not in, and both colours have to read on either background.
const paletteSwatch: Record<Palette, string> = {
  amber: "oklch(0.64 0.16 52)",
  neutral: "oklch(0.62 0 0)",
};

const paletteLabel: Record<Palette, string> = {
  amber: "general:Amber & Ink",
  neutral: "general:Neutral",
};

function PaletteSelect({palette, onChange}: {palette: Palette; onChange: (next: Palette) => void}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label={i18next.t("general:Palette")}>
          <PaletteIcon />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        {Setting.Palettes.map(option => (
          <DropdownMenuItem
            key={option}
            className={cn(palette === option && "font-semibold")}
            onSelect={() => onChange(option)}
          >
            <span className="size-3 shrink-0 rounded-full" style={{backgroundColor: paletteSwatch[option]}} />
            {i18next.t(paletteLabel[option])}
            {palette === option ? <Check className="ml-auto size-3.5" /> : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function LanguageSelect() {
  const [, forceRender] = React.useReducer((count: number) => count + 1, 0);
  const current = Setting.getLanguage();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label="Language">
          <Languages />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-80 overflow-y-auto">
        {Setting.Countries.map(country => (
          <DropdownMenuItem
            key={country.key}
            className={cn(current === country.key && "font-semibold")}
            onSelect={() => {
              Setting.setLanguage(country.key);
              // The shell reads i18next.t directly, which does not re-render on
              // a language change by itself. Forcing a render here is what makes
              // the switch take effect immediately.
              forceRender();
            }}
          >
            <img
              src={`${Setting.StaticBaseUrl}/flag-icons/${country.country}.svg`}
              alt={country.alt}
              className="h-4 w-5 rounded-[2px] object-cover"
            />
            {country.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

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

  const name = account.displayName || account.name || "";
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
        <button type="button" className="hover:bg-accent ml-1 flex items-center gap-2 rounded-md p-1 transition-colors">
          <Avatar>
            {account.avatar ? <AvatarImage src={account.avatar} alt={name} /> : null}
            <AvatarFallback style={{backgroundColor: Setting.getAvatarColor(account.name), color: "#fff"}}>
              {Setting.getShortName(account.name).slice(0, 2).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <span className="hidden max-w-[160px] truncate text-sm md:inline">{name}</span>
          <ChevronDown className="h-3.5 w-3.5 opacity-60" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        <DropdownMenuLabel className="truncate font-normal">
          <div className="text-sm font-medium">{name}</div>
          <div className="text-muted-foreground truncate text-xs">
            {account.owner}/{account.name}
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={openProfile}>
          <Settings />
          {i18next.t("account:My Account")}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onSignout}>
          <LogOut />
          {i18next.t("account:Sign Out")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * The top bar of the content column: where you are on the left, and everything
 * about the viewer themselves on the right. It sticks so the account menu stays
 * reachable on a long list page.
 */
export function AppHeader({
  collapsed,
  onToggleSidebar,
  uri,
  account,
  isAdmin,
  themeAlgorithm,
  onThemeChange,
  colorPalette,
  onColorPaletteChange,
  onSignout,
  onOpenPalette,
}: {
  collapsed: boolean;
  onToggleSidebar: () => void;
  uri: string;
  account: Account | null | undefined;
  isAdmin: boolean;
  themeAlgorithm: ThemeAlgorithm;
  onThemeChange: (next: ThemeAlgorithm) => void;
  colorPalette: Palette;
  onColorPaletteChange: (next: Palette) => void;
  onSignout: () => void;
  onOpenPalette: () => void;
}) {
  return (
    <header className="bg-background/95 supports-[backdrop-filter]:bg-background/80 sticky top-0 z-30 flex h-13 shrink-0 items-center justify-between gap-2 border-b px-2 backdrop-blur">
      <div className="flex min-w-0 items-center gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={onToggleSidebar}
          aria-label={i18next.t("general:Toggle navigation")}
        >
          {collapsed ? <PanelLeft className="size-4" /> : <PanelLeftClose className="size-4" />}
        </Button>
        <BreadcrumbBar uri={uri} />
      </div>

      <div className="flex items-center gap-1.5 pr-1">
        {account ? <CommandPaletteTrigger onOpen={onOpenPalette} /> : null}
        <VersionPanel isAdmin={isAdmin} signedIn={account !== null && account !== undefined} />
        <ThemeToggle themeAlgorithm={themeAlgorithm} onChange={onThemeChange} />
        <PaletteSelect palette={colorPalette} onChange={onColorPaletteChange} />
        <LanguageSelect />
        <AccountMenu account={account} onSignout={onSignout} />
      </div>
    </header>
  );
}
