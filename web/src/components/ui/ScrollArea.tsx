import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";

export function ScrollArea({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <div className={cn("hub-scroll min-h-0 max-w-full overflow-auto overscroll-contain", className)}>{children}</div>;
}
