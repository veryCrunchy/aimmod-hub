import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";
import { Card } from "./Card";

export function PageSection({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <Card className={cn("min-h-0 min-w-0 p-[14px] md:p-[18px]", className)}>{children}</Card>;
}
