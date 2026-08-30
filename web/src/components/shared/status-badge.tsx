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

import {cn} from "@/lib/utils";
import {Badge, type BadgeVariant} from "@/components/ui/badge";

// Pages that came from the antd build picked Tag colours by name. Keeping a
// translation table means those per-page colour maps port across unchanged
// instead of each page inventing its own palette.
const colorToVariant: Record<string, BadgeVariant> = {
  green: "success",
  success: "success",
  lime: "success",
  cyan: "info",
  blue: "info",
  geekblue: "info",
  processing: "info",
  gold: "warning",
  orange: "warning",
  warning: "warning",
  yellow: "warning",
  volcano: "danger",
  red: "danger",
  error: "danger",
  magenta: "danger",
  purple: "secondary",
  default: "muted",
};

export function tagVariant(color?: string): BadgeVariant {
  return (color && colorToVariant[color]) || "muted";
}

/** A Badge addressed by colour name, for one-to-one ports of Tag cells. */
export function ColorTag({
  color,
  className,
  children,
  ...props
}: React.ComponentProps<"span"> & {color?: string}) {
  return (
    <Badge variant={tagVariant(color)} className={className} {...props}>
      {children}
    </Badge>
  );
}

// States that recur across agents and providers. A page with
// a genuinely local vocabulary still passes its own map via `variants`.
const defaultStatusVariants: Record<string, BadgeVariant> = {
  Active: "success",
  Running: "success",
  Ready: "success",
  Healthy: "success",
  Connected: "success",
  Enabled: "success",
  Valid: "success",
  Pending: "warning",
  Starting: "warning",
  Expiring: "warning",
  Unknown: "muted",
  Disabled: "muted",
  Stopped: "muted",
  Failed: "danger",
  Error: "danger",
  Expired: "danger",
  Disconnected: "danger",
  "": "muted",
};

export function StatusBadge({
  status,
  variants,
  className,
  fallback = "Unknown",
  ...props
}: React.ComponentProps<"span"> & {
  status?: React.ReactNode;
  variants?: Record<string, BadgeVariant>;
  fallback?: string;
}) {
  const label = status === undefined || status === null || status === "" ? fallback : String(status);
  const variant = variants?.[label] ?? defaultStatusVariants[label] ?? "muted";
  return (
    <Badge variant={variant} className={cn("font-medium", className)} {...props}>
      {label}
    </Badge>
  );
}
