import * as React from "react";

import {cn} from "@/lib/utils";
import {Input} from "@/components/ui/input";

export interface NumberInputProps
  extends Omit<React.ComponentProps<"input">, "value" | "onChange" | "type"> {
  value: number | undefined | null;
  onChange: (value: number) => void;
  /** Suffix rendered inside the field, like antd's `addonAfter`. */
  addonAfter?: React.ReactNode;
}

/**
 * NumberInput is antd's InputNumber. It reports numbers rather than strings and
 * clamps to `min`/`max` so a page never has to re-validate what it stores.
 */
const NumberInput = React.forwardRef<HTMLInputElement, NumberInputProps>(
  ({className, value, onChange, addonAfter, min, max, ...props}, ref) => {
    const clamp = (next: number) => {
      let result = next;
      if (min !== undefined && result < Number(min)) {
        result = Number(min);
      }
      if (max !== undefined && result > Number(max)) {
        result = Number(max);
      }
      return result;
    };

    const field = (
      <Input
        ref={ref}
        type="number"
        min={min}
        max={max}
        value={value ?? ""}
        onChange={event => {
          const parsed = parseInt(event.target.value, 10);
          onChange(isNaN(parsed) ? 0 : parsed);
        }}
        onBlur={event => {
          const parsed = parseInt(event.target.value, 10);
          onChange(clamp(isNaN(parsed) ? 0 : parsed));
        }}
        className={cn(addonAfter ? "rounded-r-none" : "", className)}
        {...props}
      />
    );

    if (!addonAfter) {
      return field;
    }

    return (
      <div className="flex">
        {field}
        <span className="inline-flex h-9 items-center whitespace-nowrap rounded-r-md border border-l-0 border-input bg-muted px-3 text-sm text-muted-foreground">
          {addonAfter}
        </span>
      </div>
    );
  },
);
NumberInput.displayName = "NumberInput";

export {NumberInput};
