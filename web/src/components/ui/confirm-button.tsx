import * as React from "react";
import i18next from "i18next";

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
import {buttonVariants} from "@/components/ui/button";
import {cn} from "@/lib/utils";

export interface ConfirmButtonProps {
  title: React.ReactNode;
  description?: React.ReactNode;
  okText?: string;
  cancelText?: string;
  onConfirm: () => void;
  /** The trigger, e.g. a destructive Button. */
  children: React.ReactNode;
  destructive?: boolean;
}

/**
 * ConfirmButton is the shadcn stand-in for antd's Popconfirm: a trigger that
 * opens a modal asking to confirm before the action runs.
 */
export function ConfirmButton({
  title,
  description,
  okText,
  cancelText,
  onConfirm,
  children,
  destructive = true,
}: ConfirmButtonProps) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>{children}</AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          {description ? <AlertDialogDescription>{description}</AlertDialogDescription> : null}
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{cancelText ?? i18next.t("general:Cancel")}</AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            className={cn(destructive && buttonVariants({variant: "destructive"}))}
          >
            {okText ?? i18next.t("general:OK")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
