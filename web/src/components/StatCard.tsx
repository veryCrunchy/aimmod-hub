import type { ReactNode } from "react";
import { cn } from "../lib/cn";
import { Card } from "./ui/Card";

type StatCardProps = {
  label: string;
  value: ReactNode;
  /** Context for the number, such as a comparison or a unit. Keep it short. */
  detail?: ReactNode;
  accent?: "mint" | "cyan" | "gold" | "violet" | "text";
  className?: string;
};

export function StatCard({ label, value, detail, accent = "text", className }: StatCardProps) {
  const accentClass = {
    mint: "text-mint",
    cyan: "text-cyan",
    gold: "text-gold",
    violet: "text-violet",
    text: "text-text",
  }[accent];

  return (
    <Card className={cn("min-w-0 px-4 py-3", className)}>
      <div className="text-xs text-muted">{label}</div>
      <div className={cn("mt-1 break-words text-xl font-semibold leading-tight tabular-nums md:text-2xl", accentClass)}>{value}</div>
      {detail ? <div className="mt-1 text-xs leading-5 text-muted-2">{detail}</div> : null}
    </Card>
  );
}
