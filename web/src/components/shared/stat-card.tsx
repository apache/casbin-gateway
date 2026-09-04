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
import {Progress} from "@/components/ui/progress";

/**
 * A single number with its label. `tone` colours the value itself rather than
 * the card, so a wall of these still reads as one surface.
 */
export function StatCard({
  label,
  value,
  suffix,
  icon: Icon,
  tone = "default",
  hint,
  percent,
  className,
  onClick,
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  suffix?: React.ReactNode;
  icon?: React.ComponentType<{className?: string}>;
  tone?: "default" | "success" | "warning" | "danger" | "info";
  hint?: React.ReactNode;
  percent?: number | null;
  className?: string;
  onClick?: () => void;
}) {
  const toneClass = {
    default: "text-foreground",
    success: "text-success",
    warning: "text-warning",
    danger: "text-destructive",
    info: "text-info",
  }[tone];

  const Wrapper = onClick ? "button" : "div";

  return (
    <Wrapper
      type={onClick ? "button" : undefined}
      onClick={onClick}
      className={cn(
        "bg-card flex flex-col gap-2 rounded-xl border p-4 text-left shadow-sm",
        onClick && "hover:border-ring/50 transition-colors",
        className,
      )}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="text-muted-foreground truncate text-xs font-medium">{label}</span>
        {Icon ? <Icon className="text-muted-foreground size-4 shrink-0" /> : null}
      </div>
      <div className="flex items-baseline gap-1">
        <span className={cn("text-2xl font-semibold tracking-tight tabular-nums", toneClass)}>{value}</span>
        {suffix ? <span className="text-muted-foreground text-xs">{suffix}</span> : null}
      </div>
      {percent !== undefined && percent !== null ? (
        <Progress
          value={Math.min(100, Math.max(0, Number(percent) || 0))}
          tone={percent >= 90 ? "danger" : percent >= 75 ? "warning" : "default"}
          className="h-1.5"
        />
      ) : null}
      {hint ? <span className="text-muted-foreground text-xs">{hint}</span> : null}
    </Wrapper>
  );
}
