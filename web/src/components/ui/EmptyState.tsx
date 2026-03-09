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
    <div className={cn("rounded-[18px] border border-dashed border-line-strong bg-white/2 p-4", className)}>
      <strong className="mb-2 block text-sm text-text">{title}</strong>
      <p className="text-sm leading-7 text-muted">{body}</p>
      {children ? <div className="mt-3">{children}</div> : null}
    </div>
  );
}
