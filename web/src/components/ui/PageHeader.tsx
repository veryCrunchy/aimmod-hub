import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

type PageHeaderProps = {
  title: ReactNode;
  /** One short line of context, such as counts or a date. */
  meta?: ReactNode;
  actions?: ReactNode;
  /** Shown above the title, usually a breadcrumb. */
  before?: ReactNode;
  className?: string;
};

/** The plain heading every app page starts with. */
export function PageHeader({ title, meta, actions, before, className }: PageHeaderProps) {
  return (
    <header className={cn("flex flex-wrap items-end justify-between gap-x-6 gap-y-3 pt-1", className)}>
      <div className="min-w-0">
        {before}
        <h1 className="break-words text-2xl font-semibold leading-tight md:text-3xl">{title}</h1>
        {meta ? <p className="mt-1.5 text-sm text-muted">{meta}</p> : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  );
}
