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

import type {buttonVariants} from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";

/**
 * The guard in front of every destructive row action. The trigger is passed as
 * children so a call site stays a single element:
 *
 *   <ConfirmDialog title="Delete agent?" onConfirm={...}>
 *     <Button variant="outline" size="sm">Delete</Button>
 *   </ConfirmDialog>
 */
export function ConfirmDialog({
  children,
  title,
  description,
  confirmText,
  cancelText,
  variant = "destructive",
  onConfirm,
  disabled = false,
  open,
  onOpenChange,
}: {
  children: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  confirmText?: React.ReactNode;
  cancelText?: React.ReactNode;
  variant?: VariantProps<typeof buttonVariants>["variant"];
  onConfirm: () => void | Promise<unknown>;
  disabled?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [pending, setPending] = React.useState(false);
  const [uncontrolledOpen, setUncontrolledOpen] = React.useState(false);
  const isOpen = open ?? uncontrolledOpen;

  const setOpen = (next: boolean) => {
    if (open === undefined) {
      setUncontrolledOpen(next);
    }
    onOpenChange?.(next);
  };

  const handleConfirm = async(event: React.MouseEvent) => {
    // Hold the dialog open while an async handler runs, then close it here, so a
    // failure surfaces (through the handler's own toast) before it disappears.
    event.preventDefault();
    setPending(true);
    try {
      await onConfirm();
    } finally {
      setPending(false);
      setOpen(false);
    }
  };

  return (
    <AlertDialog open={isOpen} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild disabled={disabled}>
        {children}
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          {description ? <AlertDialogDescription>{description}</AlertDialogDescription> : null}
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{cancelText ?? i18next.t("general:Cancel")}</AlertDialogCancel>
          <AlertDialogAction variant={variant} onClick={handleConfirm} disabled={pending}>
            {confirmText ?? i18next.t("general:OK")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
