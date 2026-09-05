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
import {ChevronDown} from "lucide-react";

import {cn} from "@/lib/utils";

/**
 * A block of the page that stays folded until it is asked for, named by the one
 * line that says what is inside it. What is worth a whole screen to somebody
 * looking for it is worth a line to everybody else.
 */
export function Fold({
  title,
  description,
  defaultOpen = false,
  className,
  children,
}: {
  title: React.ReactNode;
  /** Shown only once the block is open, where it has something to describe. */
  description?: React.ReactNode;
  defaultOpen?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  const [open, setOpen] = React.useState(defaultOpen);

  return (
    <section className={cn("flex flex-col gap-3", className)}>
      <button
        type="button"
        onClick={() => setOpen(!open)}
        aria-expanded={open}
        className="text-muted-foreground hover:text-foreground -mx-1 flex w-fit items-center gap-1.5 px-1 text-xs transition-colors"
      >
        <ChevronDown className={cn("size-3.5 transition-transform", open && "rotate-180")} />
        {title}
      </button>

      {open ? (
        <>
          {description ? <p className="text-muted-foreground -mt-1 text-xs">{description}</p> : null}
          {children}
        </>
      ) : null}
    </section>
  );
}
