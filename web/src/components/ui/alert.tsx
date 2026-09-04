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
import {cva, type VariantProps} from "class-variance-authority";
import {AlertCircleIcon, CheckCircle2Icon, InfoIcon, TriangleAlertIcon} from "lucide-react";

import {cn} from "@/lib/utils";

const alertVariants = cva(
  "relative grid w-full grid-cols-[0_minmax(0,1fr)] items-start gap-y-0.5 rounded-lg border px-4 py-3 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_minmax(0,1fr)] has-[>svg]:gap-x-3 [&>svg]:size-4 [&>svg]:translate-y-0.5",
  {
    variants: {
      variant: {
        // As shadcn ships it: the surface stays neutral. Something wrong
        // colours its words; something merely worth knowing colours its icon.
        default: "bg-card text-card-foreground",
        destructive: "bg-card text-destructive [&>svg]:text-destructive",
        warning: "bg-card text-warning [&>svg]:text-warning",
        success: "bg-card text-card-foreground [&>svg]:text-success",
        info: "bg-card text-card-foreground [&>svg]:text-info",
      },
    },
    defaultVariants: {variant: "default"},
  },
);

type AlertVariant = NonNullable<VariantProps<typeof alertVariants>["variant"]>;

// data-variant is what lets a test assert "an error is showing" without
// selecting on generated Tailwind classes.
function Alert({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof alertVariants>) {
  return (
    <div
      data-slot="alert"
      data-variant={variant}
      role="alert"
      className={cn(alertVariants({variant}), className)}
      {...props}
    />
  );
}

function AlertTitle({className, ...props}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-title"
      className={cn("col-start-2 line-clamp-1 min-h-4 font-medium tracking-tight", className)}
      {...props}
    />
  );
}

function AlertDescription({className, ...props}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-description"
      className={cn("col-start-2 grid justify-items-start gap-1 text-sm opacity-90 [&_p]:leading-relaxed", className)}
      {...props}
    />
  );
}

const variantIcons: Record<AlertVariant, React.ComponentType<{className?: string}>> = {
  destructive: AlertCircleIcon,
  warning: TriangleAlertIcon,
  success: CheckCircle2Icon,
  info: InfoIcon,
  default: InfoIcon,
};

// Nearly every page renders the same "the request failed, here is why" banner.
// MessageAlert is that shape in one component: an icon picked from the variant,
// a title, and an optional description or action.
function MessageAlert({
  variant = "destructive",
  title,
  description,
  className,
  showIcon = true,
  action,
  ...props
}: React.ComponentProps<"div"> & {
  variant?: AlertVariant;
  title?: React.ReactNode;
  description?: React.ReactNode;
  showIcon?: boolean;
  action?: React.ReactNode;
}) {
  const Icon = variantIcons[variant] ?? InfoIcon;
  return (
    <Alert variant={variant} className={className} {...props}>
      {showIcon ? <Icon /> : null}
      {title ? <AlertTitle>{title}</AlertTitle> : null}
      {description ? <AlertDescription>{description}</AlertDescription> : null}
      {action ? <div className="col-start-2 mt-2">{action}</div> : null}
    </Alert>
  );
}

export {Alert, AlertTitle, AlertDescription, MessageAlert, alertVariants};
