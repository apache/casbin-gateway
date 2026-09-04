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
import {Inbox, RefreshCw, TriangleAlert} from "lucide-react";
import i18next from "i18next";

import {Button} from "@/components/ui/button";
import {cn} from "@/lib/utils";

export function EmptyState({
  icon,
  title = "No data",
  description,
  action,
  className,
}: {
  icon?: React.ComponentType<{className?: string}>;
  title?: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  const Icon = icon ?? Inbox;
  return (
    <div className={cn("flex flex-col items-center justify-center gap-2 px-6 py-14 text-center", className)}>
      <div className="bg-muted text-muted-foreground flex size-10 items-center justify-center rounded-full">
        <Icon className="size-5" />
      </div>
      <p className="text-sm font-medium">{title}</p>
      {description ? <p className="text-muted-foreground max-w-sm text-xs">{description}</p> : null}
      {action ? <div className="mt-2">{action}</div> : null}
    </div>
  );
}

/**
 * What a listing shows when it could not be read. A load that failed must never
 * fall through to EmptyState: "you have no providers" and "the providers could
 * not be read" look the same there, and the first one is alarming to be told
 * about an account that holds your API keys.
 */
export function ErrorState({
  title,
  error,
  onRetry,
  className,
}: {
  title?: React.ReactNode;
  error?: React.ReactNode;
  onRetry?: () => void;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center justify-center gap-2 px-6 py-14 text-center", className)}>
      <div className="bg-muted text-destructive flex size-10 items-center justify-center rounded-full">
        <TriangleAlert className="size-5" />
      </div>
      <p className="text-sm font-medium">{title ?? i18next.t("general:Could not load this")}</p>
      {error ? <p className="text-muted-foreground max-w-sm text-xs break-words">{error}</p> : null}
      {onRetry ? (
        <div className="mt-2">
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RefreshCw />
            {i18next.t("general:Retry")}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
