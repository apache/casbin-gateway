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
import {CircleAlert, CircleX, SearchX, ShieldX} from "lucide-react";
import i18next from "i18next";

import {Button} from "@/components/ui/button";

const icons = {
  "403": <ShieldX className="h-14 w-14 text-destructive" />,
  "404": <SearchX className="h-14 w-14 text-muted-foreground" />,
  error: <CircleX className="h-14 w-14 text-destructive" />,
  warning: <CircleAlert className="h-14 w-14 text-warning" />,
};

export function Result({
  status = "error",
  title,
  subTitle,
  extra,
}: {
  status?: keyof typeof icons;
  title: React.ReactNode;
  subTitle?: React.ReactNode;
  extra?: React.ReactNode;
}) {
  return (
    <div className="flex min-h-[50vh] flex-col items-center justify-center gap-4 px-6 text-center">
      {icons[status]}
      <div className="text-xl font-semibold">{title}</div>
      {subTitle ? <div className="max-w-xl text-sm text-muted-foreground">{subTitle}</div> : null}
      {extra ? <div className="flex gap-2">{extra}</div> : null}
    </div>
  );
}

/** The 403 every admin-only page falls back to, matching the antd build's wording. */
export function UnauthorizedResult() {
  return (
    <Result
      status="403"
      title="403 Unauthorized"
      subTitle={i18next.t(
        "general:Sorry, you do not have permission to access this page or logged in status invalid.",
      )}
      extra={
        <Button asChild>
          <a href="/">{i18next.t("general:Back Home")}</a>
        </Button>
      }
    />
  );
}
