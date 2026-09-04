import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

type EmptyStateProps = {
  title: string;
  body: string;
  children?: ReactNode;
  className?: string;
};

export function EmptyState({ title, body, children, className }: EmptyStateProps) {
  return (
    <div role="status" className={cn("min-w-0 border-y border-line py-8 px-4 text-center", className)}>
      <strong className="mb-2 block text-sm text-text">{title}</strong>
      <p className="mx-auto max-w-prose break-words text-sm leading-6 text-muted">{body}</p>
      {children ? <div className="mt-3">{children}</div> : null}
    </div>
  );
}
