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
import i18next from "i18next";
import type {VariantProps} from "class-variance-authority";

import {cn} from "@/lib/utils";
import {Button, buttonVariants} from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {Label} from "@/components/ui/label";

/**
 * The "open a modal, fill a short form, POST it" shape that most create flows
 * reduce to. It owns the chrome — title, footer, submit state, Enter to submit —
 * so a page only writes its fields.
 */
export function FormDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  onSubmit,
  submitText,
  cancelText,
  submitting = false,
  submitDisabled = false,
  submitVariant = "default",
  size = "default",
  columns = 1,
  note,
  footer,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: React.ReactNode;
  description?: React.ReactNode;
  children: React.ReactNode;
  onSubmit: () => void;
  submitText?: React.ReactNode;
  cancelText?: React.ReactNode;
  submitting?: boolean;
  submitDisabled?: boolean;
  submitVariant?: VariantProps<typeof buttonVariants>["variant"];
  size?: "default" | "lg" | "xl";
  /** Two columns keep a long form on one screen instead of a scrollbar. */
  columns?: 1 | 2;
  /** A line kept beside the buttons, out of the scrolling part of the form. */
  note?: React.ReactNode;
  footer?: React.ReactNode;
}) {
  const sizeClass = {
    default: "sm:max-w-lg",
    lg: "sm:max-w-2xl",
    xl: "sm:max-w-4xl",
  }[size];

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    onSubmit();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn("flex max-h-[92vh] flex-col overflow-hidden", sizeClass)}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col gap-4">
          <div className="scrollbar-thin -mx-1 min-h-0 flex-1 overflow-y-auto px-1 py-0.5">
            <div className={cn("grid gap-4", columns === 2 && "md:grid-cols-2")}>{children}</div>
          </div>
          <DialogFooter className="sm:items-center">
            {note ? <div className="sm:mr-auto">{note}</div> : null}
            {footer ?? (
              <>
                <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
                  {cancelText ?? i18next.t("general:Cancel")}
                </Button>
                <Button type="submit" variant={submitVariant} loading={submitting} disabled={submitDisabled}>
                  {submitText ?? i18next.t("general:OK")}
                </Button>
              </>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/**
 * A labelled control with optional hint and error text. Deliberately
 * uncontrolled about the input itself so it wraps anything — Input, Textarea,
 * SimpleSelect, a custom editor.
 */
export function Field({
  label,
  htmlFor,
  required = false,
  hint,
  error,
  children,
  className,
}: {
  label?: React.ReactNode;
  htmlFor?: string;
  required?: boolean;
  hint?: React.ReactNode;
  error?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("grid gap-2", className)}>
      {label ? (
        <Label htmlFor={htmlFor}>
          {label}
          {required ? <span className="text-destructive">*</span> : null}
        </Label>
      ) : null}
      {children}
      {hint && !error ? <p className="text-muted-foreground text-xs">{hint}</p> : null}
      {error ? <p className="text-destructive text-xs">{error}</p> : null}
    </div>
  );
}
