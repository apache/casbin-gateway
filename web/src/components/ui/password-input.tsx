import * as React from "react";
import {Eye, EyeOff} from "lucide-react";

import {cn} from "@/lib/utils";
import {Input} from "@/components/ui/input";

/** PasswordInput is antd's Input.Password: a text field with a reveal toggle. */
const PasswordInput = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(
  ({className, ...props}, ref) => {
    const [visible, setVisible] = React.useState(false);

    return (
      <div className="relative">
        <Input ref={ref} type={visible ? "text" : "password"} className={cn("pr-9", className)} {...props} />
        <button
          type="button"
          tabIndex={-1}
          onClick={() => setVisible(current => !current)}
          className="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
          aria-label={visible ? "Hide password" : "Show password"}
        >
          {visible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </div>
    );
  },
);
PasswordInput.displayName = "PasswordInput";

export {PasswordInput};
