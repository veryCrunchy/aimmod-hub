import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";

export function PageStack({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <div className={cn("grid gap-4 md:gap-5", className)}>{children}</div>;
}

export function Grid({
  children,
  className
}: PropsWithChildren<{ className?: string }>) {
  return <div className={cn("grid gap-3 md:gap-4", className)}>{children}</div>;
}
