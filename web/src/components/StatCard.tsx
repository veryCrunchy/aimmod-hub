import { cn } from "../lib/cn";
import { Card } from "./ui/Card";

type StatCardProps = {
  label: string;
  value: string;
  detail?: string;
  accent?: "mint" | "cyan" | "gold" | "violet";
};

export function StatCard({ label, value, detail, accent = "mint" }: StatCardProps) {
  const accentClass = {
    mint: "text-mint",
    cyan: "text-cyan",
    gold: "text-gold",
    violet: "text-violet"
  }[accent];

  return (
    <Card className="p-4 md:p-5">
      <div className="text-[11px] uppercase tracking-[0.1em] text-cyan">{label}</div>
      <div className={cn("mt-2 text-[clamp(22px,2.6vw,30px)] leading-none", accentClass)}>{value}</div>
      {detail ? <p className="mt-2.5 text-[13px] leading-6 text-muted md:text-sm">{detail}</p> : null}
    </Card>
  );
}
