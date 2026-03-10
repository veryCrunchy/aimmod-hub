import type { ReactNode } from "react";
import { cn } from "../lib/cn";

type SectionHeaderProps = {
  eyebrow: string;
  title: ReactNode;
  body?: ReactNode;
  aside?: ReactNode;
  className?: string;
};

export function SectionHeader({ eyebrow, title, body, aside, className }: SectionHeaderProps) {
  return (
    <div className={cn("mb-[18px] flex items-start justify-between gap-5", className)}>
      <div>
        <div className="text-[12px] uppercase tracking-[0.1em] text-cyan">{eyebrow}</div>
        <h2 className="my-[10px] text-[clamp(24px,3vw,40px)] leading-[1.05]">{title}</h2>
        {body ? <p className="text-sm leading-7 text-muted">{body}</p> : null}
      </div>
      {aside ? <div className="pt-1.5 text-[13px] text-muted">{aside}</div> : null}
    </div>
  );
}
