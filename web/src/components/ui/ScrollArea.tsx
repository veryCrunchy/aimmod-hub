import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";

export function ScrollArea({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <div className={cn("hub-scroll min-h-0 overflow-auto", className)}>{children}</div>;
}
