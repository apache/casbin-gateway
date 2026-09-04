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
import {Check, ChevronsUpDown, X} from "lucide-react";
import i18next from "i18next";

import {cn} from "@/lib/utils";
import {Button} from "@/components/ui/button";
import {Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList} from "@/components/ui/command";
import {Popover, PopoverContent, PopoverTrigger} from "@/components/ui/popover";
import {Select, SelectContent, SelectItem, SelectTrigger, SelectValue} from "@/components/ui/select";

export interface SelectOption {
  label: React.ReactNode;
  value: string;
  disabled?: boolean;
  /** What a search matches against, for a label that is a brand mark and a name. */
  text?: string;
}

// Radix refuses an item whose value is the empty string, but "" is exactly how
// this app spells "no filter" (any status, any owner). The sentinel keeps that
// vocabulary intact at the call sites and never leaks out of this module.
const EMPTY_VALUE = "__all__";

const toInternal = (value?: string) => (value === "" || value === undefined || value === null ? EMPTY_VALUE : value);
const toExternal = (value: string) => (value === EMPTY_VALUE ? "" : value);

function normalizeOptions(options: (SelectOption | string)[]): SelectOption[] {
  return (options ?? []).map(option => (typeof option === "string" ? {label: option, value: option} : option));
}

/** A plain dropdown with an options array. */
export function SimpleSelect({
  value,
  onChange,
  options,
  placeholder,
  className,
  contentClassName,
  size = "default",
  disabled = false,
  id,
  "aria-label": ariaLabel,
}: {
  value?: string;
  onChange?: (value: string) => void;
  options: (SelectOption | string)[];
  placeholder?: string;
  className?: string;
  contentClassName?: string;
  size?: "sm" | "default";
  disabled?: boolean;
  id?: string;
  "aria-label"?: string;
}) {
  const items = normalizeOptions(options);
  return (
    <Select value={toInternal(value)} onValueChange={next => onChange?.(toExternal(next))} disabled={disabled}>
      <SelectTrigger id={id} size={size} className={cn("w-full", className)} aria-label={ariaLabel}>
        <SelectValue placeholder={placeholder ?? i18next.t("general:Select")} />
      </SelectTrigger>
      <SelectContent className={contentClassName}>
        {items.map(option => (
          <SelectItem key={option.value} value={toInternal(option.value)} disabled={option.disabled}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

/** Combobox for long lists — certificates, applications, base URLs. */
export function SearchSelect({
  value,
  onChange,
  options,
  placeholder,
  searchPlaceholder,
  emptyText,
  className,
  disabled = false,
  allowClear = false,
  allowCustomValue = false,
  id,
}: {
  value?: string;
  onChange?: (value: string) => void;
  options: (SelectOption | string)[];
  placeholder?: string;
  searchPlaceholder?: string;
  emptyText?: React.ReactNode;
  className?: string;
  disabled?: boolean;
  allowClear?: boolean;
  /** Keeps whatever is typed even when it matches no option. */
  allowCustomValue?: boolean;
  id?: string;
}) {
  const [open, setOpen] = React.useState(false);
  const [search, setSearch] = React.useState("");
  const items = normalizeOptions(options);
  const matched = items.find(option => option.value === (value ?? ""));
  // A custom value has no option to read a label from, so it labels itself.
  const selected = matched ?? (allowCustomValue && value ? {label: value, value} : undefined);

  const pick = (next: string) => {
    onChange?.(next);
    setSearch("");
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          id={id}
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={disabled}
          className={cn("w-full justify-between font-normal", !selected && "text-muted-foreground", className)}
        >
          <span className="truncate">{selected ? selected.label : (placeholder ?? i18next.t("general:Select"))}</span>
          <span className="flex items-center gap-1">
            {allowClear && selected ? (
              <X
                className="hover:text-foreground size-3.5 opacity-50"
                onClick={event => {
                  event.stopPropagation();
                  onChange?.("");
                }}
              />
            ) : null}
            <ChevronsUpDown className="size-4 shrink-0 opacity-50" />
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) p-0" align="start">
        <Command>
          <CommandInput
            value={search}
            onValueChange={setSearch}
            placeholder={searchPlaceholder ?? i18next.t("general:Search")}
          />
          <CommandList>
            <CommandEmpty>{emptyText ?? i18next.t("general:No data")}</CommandEmpty>
            <CommandGroup>
              {allowCustomValue && search !== "" && !items.some(option => option.value === search) ? (
                <CommandItem value={search} onSelect={() => pick(search)}>
                  <Check className="size-4 opacity-0" />
                  {search}
                </CommandItem>
              ) : null}
              {items.map(option => (
                <CommandItem
                  key={option.value}
                  value={option.text ?? String(option.label)}
                  onSelect={() => pick(option.value)}
                >
                  <Check className={cn("size-4", option.value === value ? "opacity-100" : "opacity-0")} />
                  {option.label}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}
