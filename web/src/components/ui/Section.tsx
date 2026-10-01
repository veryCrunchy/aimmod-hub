import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

type SectionProps = {
  title: ReactNode;
  /** Small text or link on the right of the heading. */
  aside?: ReactNode;
  children: ReactNode;
  className?: string;
  id?: string;
};

/** A titled block of content. One line of heading, no filler copy. */
export function Section({ title, aside, children, className, id }: SectionProps) {
  return (
    <section id={id} className={cn("min-w-0", className)} aria-labelledby={id ? `${id}-title` : undefined}>
      <div className="mb-3 flex items-baseline justify-between gap-3">
        <h2 id={id ? `${id}-title` : undefined} className="text-base font-semibold">{title}</h2>
        {aside ? <div className="shrink-0 text-sm text-muted">{aside}</div> : null}
      </div>
      {children}
    </section>
  );
}
