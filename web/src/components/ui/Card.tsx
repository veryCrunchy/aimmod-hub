import type { HTMLAttributes } from "react";
import { cn } from "../../lib/cn";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      {...props}
      className={cn(
        "min-h-0 min-w-0 overflow-hidden rounded-[18px] border border-line bg-panel shadow-panel max-[720px]:rounded-[16px]",
        className
      )}
    />
  );
}
