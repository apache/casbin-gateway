import {Loader2} from "lucide-react";

import {cn} from "@/lib/utils";

export function Spinner({className}: {className?: string}) {
  return <Loader2 className={cn("h-4 w-4 animate-spin", className)} />;
}

export function PageSpinner({tip}: {tip?: string}) {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center gap-3 text-muted-foreground">
      <Spinner className="h-8 w-8" />
      {tip ? <span className="text-sm">{tip}</span> : null}
    </div>
  );
}
