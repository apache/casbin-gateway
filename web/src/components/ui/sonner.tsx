import {Toaster as Sonner} from "sonner";

export function Toaster() {
  return (
    <Sonner
      position="top-center"
      toastOptions={{
        classNames: {
          toast: "group rounded-md border bg-background text-foreground shadow-lg",
          description: "text-muted-foreground",
          error: "border-destructive/40",
          success: "border-success/40",
        },
      }}
    />
  );
}
