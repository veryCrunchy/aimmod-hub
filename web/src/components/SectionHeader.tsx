import type { ReactNode } from "react";
import { cn } from "../lib/cn";

type SectionHeaderProps = {
  eyebrow: string;
  title: ReactNode;
  body?: ReactNode;
  aside?: ReactNode;
  className?: string;
  level?: 1 | 2 | 3;
};

export function SectionHeader({ eyebrow, title, body, aside, className, level = 2 }: SectionHeaderProps) {
  const Heading = level === 1 ? "h1" : level === 3 ? "h3" : "h2";
  return (
    <div className={cn("mb-4 flex flex-col gap-3 md:mb-[18px] md:flex-row md:items-start md:justify-between md:gap-5", className)}>
      <div className="min-w-0">
        <div className="text-xs font-medium text-muted">{eyebrow}</div>
        <Heading className={cn("my-1.5 break-words font-semibold leading-snug", level === 1 ? "text-3xl" : "text-xl")}>{title}</Heading>
        {body ? <p className="max-w-[72ch] break-words text-[12px] leading-5 text-muted md:text-[13px] md:leading-6">{body}</p> : null}
      </div>
      {aside ? <div className="min-w-0 pt-0 text-[12px] text-muted md:pt-1.5 md:text-[13px]">{aside}</div> : null}
    </div>
  );
}
