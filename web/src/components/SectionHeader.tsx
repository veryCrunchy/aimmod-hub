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
    <div className={cn("mb-4 flex flex-col gap-3 md:mb-[18px] md:flex-row md:items-start md:justify-between md:gap-5", className)}>
      <div className="min-w-0">
        <div className="text-[11px] uppercase tracking-[0.1em] text-cyan">{eyebrow}</div>
        <h2 className="my-1.5 break-words text-[clamp(18px,2.3vw,30px)] leading-[1.04] tracking-[-0.03em]">{title}</h2>
        {body ? <p className="max-w-[72ch] break-words text-[12px] leading-5 text-muted md:text-[13px] md:leading-6">{body}</p> : null}
      </div>
      {aside ? <div className="min-w-0 pt-0 text-[12px] text-muted md:pt-1.5 md:text-[13px]">{aside}</div> : null}
    </div>
  );
}
