import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";

export function PageStack({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <div className={cn("grid gap-5", className)}>{children}</div>;
}

export function Grid({
  children,
  className
}: PropsWithChildren<{ className?: string }>) {
  return <div className={cn("grid gap-[18px]", className)}>{children}</div>;
}
