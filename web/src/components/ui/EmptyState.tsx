import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

type EmptyStateProps = {
  title: string;
  body?: string;
  /** A single useful next step, such as a link or button. */
  children?: ReactNode;
  className?: string;
};

export function EmptyState({ title, body, children, className }: EmptyStateProps) {
  return (
    <div role="status" className={cn("min-w-0 rounded-md border border-dashed border-line px-4 py-8 text-center", className)}>
      <strong className="block text-sm text-text">{title}</strong>
      {body ? <p className="mx-auto mt-1.5 max-w-prose break-words text-sm leading-6 text-muted">{body}</p> : null}
      {children ? <div className="mt-3 flex flex-wrap justify-center gap-2">{children}</div> : null}
    </div>
  );
}
