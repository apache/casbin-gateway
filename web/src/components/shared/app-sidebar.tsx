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
import {Link} from "react-router-dom";
import {ChevronDown, type LucideIcon} from "lucide-react";
import i18next from "i18next";

import * as Setting from "@/Setting";
import {cn} from "@/lib/utils";
import {navGroups, type NavGroup} from "@/nav";
import {SimpleTooltip} from "@/components/ui/tooltip";

const OPEN_KEYS_STORAGE = "siderMenuOpenKeys";
// Nothing is expanded until the reader is inside a group: nine sections of
// Settings unfolded on every page would bury the rest of the rail.
const DEFAULT_OPEN_KEYS: string[] = [];

export function readSavedOpenKeys(): string[] {
  try {
    const raw = localStorage.getItem(OPEN_KEYS_STORAGE);
    if (!raw) {
      return DEFAULT_OPEN_KEYS;
    }
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed.filter((key: unknown) => typeof key === "string") : DEFAULT_OPEN_KEYS;
  } catch {
    return DEFAULT_OPEN_KEYS;
  }
}

export function persistOpenKeys(keys: string[]) {
  try {
    localStorage.setItem(OPEN_KEYS_STORAGE, JSON.stringify(keys));
  } catch {
    // Private-mode storage failures must not take the navigation down.
  }
}

/** Tailwind's md breakpoint, in JS, for the decisions CSS cannot make. */
export function useIsDesktop() {
  const [isDesktop, setIsDesktop] = React.useState(
    () => typeof window === "undefined" || window.matchMedia("(min-width: 768px)").matches,
  );

  React.useEffect(() => {
    const query = window.matchMedia("(min-width: 768px)");
    const update = () => setIsDesktop(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  return isDesktop;
}

function NavLink({
  to,
  active,
  collapsed,
  icon: Icon,
  label,
  nested = false,
}: {
  to: string;
  active: boolean;
  collapsed: boolean;
  icon?: LucideIcon;
  label: string;
  nested?: boolean;
}) {
  const content = (
    <Link
      to={to}
      className={cn(
        "relative flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors",
        collapsed && "justify-center px-0",
        nested && !collapsed && "pl-9 text-[13px]",
        active
          // The amber rule is what marks the page you are on; the tinted fill
          // alone is too quiet to find at a glance down a rail this long.
          ? "bg-sidebar-accent text-sidebar-accent-foreground before:bg-sidebar-primary font-medium before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-[3px] before:rounded-full before:content-['']"
          : "text-sidebar-foreground/70 hover:bg-sidebar-accent/60 hover:text-sidebar-foreground",
      )}
    >
      {Icon ? <Icon className="size-4 shrink-0" /> : null}
      {!collapsed ? <span className="truncate">{label}</span> : null}
    </Link>
  );

  return collapsed ? (
    <SimpleTooltip title={label} side="right">
      {content}
    </SimpleTooltip>
  ) : (
    content
  );
}

/**
 * Fixed navigation rail. When collapsed it drops to icons only and the groups
 * stop expanding in place — an accordion inside a 64px column is unreadable, so
 * the icon links straight to the group's first page instead.
 */
export function AppSidebar({
  collapsed,
  selectedKey,
  openKeys,
  onOpenKeysChange,
  isAdmin,
  mobileOpen,
  onMobileOpenChange,
}: {
  collapsed: boolean;
  selectedKey: string;
  openKeys: string[];
  onOpenKeysChange: (keys: string[]) => void;
  isAdmin: boolean;
  /** On a narrow screen the rail is a drawer instead, and this is whether it is out. */
  mobileOpen: boolean;
  onMobileOpenChange: (open: boolean) => void;
}) {
  // Below md the rail is a full-width drawer, where icons without labels would
  // be a worse menu than the one there is room for.
  const isDesktop = useIsDesktop();
  const railCollapsed = collapsed && isDesktop;

  const toggleGroup = (key: string) => {
    onOpenKeysChange(openKeys.includes(key) ? openKeys.filter(item => item !== key) : [...openKeys, key]);
  };

  const visible = (entry: {adminOnly?: boolean; hidden?: boolean}) =>
    !entry.hidden && (!entry.adminOnly || isAdmin);
  const groups: NavGroup[] = navGroups
    .filter(visible)
    .map(group => (group.children ? {...group, children: group.children.filter(visible)} : group))
    .filter(group => !group.children || group.children.length > 0);

  return (
    <>
      {/* Below md the rail slides over the page, so it needs something to
          dismiss it that is not one of its own links. */}
      <div
        className={cn(
          "fixed inset-0 z-30 bg-black/40 md:hidden",
          mobileOpen ? "block" : "hidden",
        )}
        onClick={() => onMobileOpenChange(false)}
      />
      {/* Slid in and out on "left" rather than a transform: the rail is a
          tooltip anchor, and a transformed ancestor moves those with it. */}
      <aside
        className={cn(
          "bg-sidebar fixed inset-y-0 z-40 flex w-64 flex-col border-r transition-[left,width] duration-200 md:left-0",
          collapsed ? "md:w-16" : "md:w-64",
          mobileOpen ? "left-0" : "-left-64",
        )}
      >
        <div className={cn("flex h-13 shrink-0 items-center border-b px-5", railCollapsed && "justify-center px-0")}>
          <Link to="/" className="flex items-center overflow-hidden">
            <img
              src={`${Setting.StaticBaseUrl}/img/logo_384x96.png`}
              alt="Casbin Gateway"
              // The wordmark is black ink on transparent. Inverting luminance
              // and rotating the hue back lights it up without draining the
              // colour out of the gopher and the shield.
              className={cn(
                "w-auto object-contain transition-all dark:invert dark:hue-rotate-180",
                railCollapsed ? "h-5" : "h-7 max-w-[150px]",
              )}
            />
          </Link>
        </div>

        <nav
          className="scrollbar-thin flex-1 space-y-0.5 overflow-y-auto p-2"
          onClick={() => onMobileOpenChange(false)}
        >
          {groups.map(group => {
            if (!group.children) {
              return (
                <NavLink
                  key={group.key}
                  to={group.path ?? group.key}
                  active={selectedKey === group.key}
                  collapsed={railCollapsed}
                  icon={group.icon}
                  label={i18next.t(group.label)}
                />
              );
            }

            const groupLabel = i18next.t(group.label);
            const isOpen = openKeys.includes(group.key);
            const hasActiveChild = group.children.some(child => child.key === selectedKey);

            if (railCollapsed) {
              return (
                <NavLink
                  key={group.key}
                  to={group.path ?? group.children[0].path}
                  active={hasActiveChild}
                  collapsed
                  icon={group.icon}
                  label={groupLabel}
                />
              );
            }

            const header = cn(
              "flex items-center gap-2.5 text-sm transition-colors",
              hasActiveChild ? "text-sidebar-foreground font-medium" : "text-sidebar-foreground/70",
              "hover:text-sidebar-foreground",
            );

            return (
              <div key={group.key}>
                {/* The label goes to the page, the chevron only unfolds it —
                    a group is a destination as well as a heading. */}
                <div className="hover:bg-sidebar-accent/60 flex items-center rounded-md pr-1 transition-colors">
                  <Link to={group.path ?? group.children[0].path} className={cn(header, "min-w-0 flex-1 px-2.5 py-2")}>
                    {group.icon ? <group.icon className="size-4 shrink-0" /> : null}
                    <span className="truncate text-left">{groupLabel}</span>
                  </Link>
                  <button
                    type="button"
                    onClick={event => {
                      // The rail closes itself on any link tap while it is a
                      // drawer, and unfolding a group is not leaving it.
                      event.stopPropagation();
                      toggleGroup(group.key);
                    }}
                    className={cn(header, "rounded-md p-1.5")}
                    aria-expanded={isOpen}
                    aria-label={groupLabel}
                  >
                    <ChevronDown className={cn("size-3.5 shrink-0 transition-transform", isOpen && "rotate-180")} />
                  </button>
                </div>
                {isOpen ? (
                  <div className="mt-0.5 space-y-0.5">
                    {group.children.map(child => (
                      <NavLink
                        key={child.key}
                        to={child.path}
                        active={selectedKey === child.key}
                        collapsed={false}
                        label={i18next.t(child.label)}
                        nested
                      />
                    ))}
                  </div>
                ) : null}
              </div>
            );
          })}
        </nav>
      </aside>
    </>
  );
}
