import * as React from "react";
import {X} from "lucide-react";

import {cn} from "@/lib/utils";
import {Badge} from "@/components/ui/badge";

export interface TagsInputProps {
  value: string[] | undefined | null;
  onChange: (value: string[]) => void;
  placeholder?: string;
  /** Values offered as a datalist while typing, the way antd Select showed its options. */
  suggestions?: string[];
  disabled?: boolean;
  className?: string;
}

/**
 * TagsInput replaces antd's `Select mode="tags"`: a free-text field that turns
 * whatever is typed into a removable chip. Enter and comma commit the current
 * text, Backspace on an empty field removes the last chip.
 */
export function TagsInput({
  value,
  onChange,
  placeholder,
  suggestions,
  disabled,
  className,
}: TagsInputProps) {
  const [draft, setDraft] = React.useState("");
  const tags = value || [];
  const listId = React.useId();

  const commit = (raw: string) => {
    const text = raw.trim();
    setDraft("");
    if (text === "" || tags.includes(text)) {
      return;
    }
    onChange([...tags, text]);
  };

  const remove = (index: number) => {
    onChange(tags.filter((_, i) => i !== index));
  };

  const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      commit(draft);
    } else if (event.key === "Backspace" && draft === "" && tags.length > 0) {
      remove(tags.length - 1);
    }
  };

  return (
    <div
      className={cn(
        "flex min-h-9 w-full flex-wrap items-center gap-1 rounded-md border border-input bg-transparent px-2 py-1 text-sm shadow-sm focus-within:ring-1 focus-within:ring-ring",
        disabled && "cursor-not-allowed opacity-50",
        className,
      )}
    >
      {tags.map((tag, index) => (
        <Badge key={`${tag}-${index}`} variant="secondary" className="gap-1">
          {tag}
          {!disabled && (
            <button
              type="button"
              onClick={() => remove(index)}
              className="opacity-60 hover:opacity-100"
              aria-label={`Remove ${tag}`}
            >
              <X className="h-3 w-3" />
            </button>
          )}
        </Badge>
      ))}
      <input
        className="h-7 min-w-[8rem] flex-1 bg-transparent outline-none placeholder:text-muted-foreground"
        value={draft}
        disabled={disabled}
        list={suggestions && suggestions.length > 0 ? listId : undefined}
        placeholder={tags.length === 0 ? placeholder : undefined}
        onChange={event => setDraft(event.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => commit(draft)}
      />
      {suggestions && suggestions.length > 0 && (
        <datalist id={listId}>
          {suggestions.map(item => (
            <option key={item} value={item} />
          ))}
        </datalist>
      )}
    </div>
  );
}
