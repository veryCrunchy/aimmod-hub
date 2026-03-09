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
    <Card className="p-5">
      <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">{label}</div>
      <div className={cn("mt-2.5 text-[clamp(24px,3vw,34px)]", accentClass)}>{value}</div>
      {detail ? <p className="mt-3 text-sm leading-7 text-muted">{detail}</p> : null}
    </Card>
  );
}
