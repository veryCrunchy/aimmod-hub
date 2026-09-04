import type { HTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export function Card({ className, children, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div {...props} className={cn("relative min-h-0 min-w-0 rounded-md border border-line bg-panel", className)}>{children}</div>;
}
