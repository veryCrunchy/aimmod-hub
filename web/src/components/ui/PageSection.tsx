import type { HTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export function PageSection({ children, className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <section {...props} className={cn("hub-section min-h-0 min-w-0 py-3", className)}>{children}</section>;
}
