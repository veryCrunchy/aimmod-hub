import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";
import { Card } from "./Card";

export function PageSection({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <Card className={cn("min-h-0 p-4 md:p-5", className)}>{children}</Card>;
}
