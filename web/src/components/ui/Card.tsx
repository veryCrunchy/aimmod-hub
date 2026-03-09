import type { HTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      {...props}
      className={cn(
        "min-h-0 rounded-[22px] border border-line bg-panel shadow-panel",
        className
      )}
    />
  );
}
