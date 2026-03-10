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
    <Card className="min-w-0 p-[14px] md:p-[18px]">
      <div className="text-[11px] uppercase tracking-[0.1em] text-cyan">{label}</div>
      <div className={cn("mt-2 break-words text-[clamp(20px,2.2vw,28px)] leading-none", accentClass)}>{value}</div>
      {detail ? <p className="mt-2 text-[12px] leading-5 text-muted md:text-[13px] md:leading-6">{detail}</p> : null}
    </Card>
  );
}
