import type { PropsWithChildren } from "react";
import { cn } from "../../lib/cn";
import { Card } from "./Card";

export function PageSection({ children, className }: PropsWithChildren<{ className?: string }>) {
  return <Card className={cn("p-6", className)}>{children}</Card>;
}
