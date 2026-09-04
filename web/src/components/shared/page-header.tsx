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
import {useLocation} from "react-router-dom";
import {ChevronDown} from "lucide-react";

import {cn} from "@/lib/utils";
import {Card, CardContent, CardDescription, CardHeader, CardTitle} from "@/components/ui/card";

export function PageContainer({className, children}: {className?: string; children: React.ReactNode}) {
  return <div className={cn("flex flex-col gap-4 p-4 md:p-6", className)}>{children}</div>;
}

export function PageHeader({
  title,
  description,
  actions,
  className,
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between", className)}>
      <div className="min-w-0">
        <h1 className="truncate text-xl font-semibold tracking-tight">{title}</h1>
        {description ? <p className="text-muted-foreground mt-1 text-sm">{description}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}

/**
 * One group of fields on an edit page. The fields sit in a responsive grid so a
 * long form reads as a few short sections rather than one tall column.
 */
export function Section({
  id,
  title,
  description,
  columns = 3,
  className,
  collapsible = false,
  children,
}: {
  /** Names the section for the rail, which links to it by hash. */
  id?: string;
  title?: React.ReactNode;
  description?: React.ReactNode;
  columns?: 1 | 2 | 3;
  className?: string;
  /** Starts folded, for a section most people never open. */
  collapsible?: boolean;
  children: React.ReactNode;
}) {
  const [open, setOpen] = React.useState(!collapsible);
  const {hash} = useLocation();
  const targeted = id !== undefined && hash === `#${id}`;

  // Arriving at a folded section by link has to unfold it, or the jump lands on
  // a title with nothing under it.
  React.useEffect(() => {
    if (targeted) {
      setOpen(true);
    }
  }, [targeted]);

  return (
    <Card id={id} className={cn("scroll-mt-20 gap-4 py-5", className)}>
      {title || description ? (
        <CardHeader className="px-5">
          {title ? (
            collapsible ? (
              <button
                type="button"
                onClick={() => setOpen(!open)}
                className="flex w-full items-center gap-1.5 text-left"
              >
                <ChevronDown className={cn("size-4 transition-transform", open && "rotate-180")} />
                <CardTitle className="text-[15px]">{title}</CardTitle>
              </button>
            ) : (
              <CardTitle className="text-[15px]">{title}</CardTitle>
            )
          ) : null}
          {description ? <CardDescription>{description}</CardDescription> : null}
        </CardHeader>
      ) : null}
      <CardContent className={cn("px-5", !open && "hidden")}>
        <div
          className={cn(
            "grid gap-4",
            columns === 2 && "md:grid-cols-2",
            columns === 3 && "md:grid-cols-2 lg:grid-cols-3",
          )}
        >
          {children}
        </div>
      </CardContent>
    </Card>
  );
}
