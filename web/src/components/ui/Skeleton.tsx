export function Skeleton({ className = "" }: { className?: string }) {
  return <div aria-hidden="true" className={`animate-pulse rounded-md bg-white/[0.05] ${className}`} />;
}

/** Placeholder for a page that is loading: a heading, a stat row and a table. */
export function PageSkeleton({ stats = 4, rows = 6, label = "Loading" }: { stats?: number; rows?: number; label?: string }) {
  return (
    <div role="status" aria-label={label} className="grid gap-5">
      <div className="grid gap-2"><Skeleton className="h-8 w-64 max-w-full" /><Skeleton className="h-4 w-48 max-w-full" /></div>
      {stats > 0 && <div className="grid grid-cols-2 gap-3 md:grid-cols-4">{Array.from({ length: stats }, (_, i) => <Skeleton key={i} className="h-[76px]" />)}</div>}
      <TableSkeleton rows={rows} />
    </div>
  );
}

export function TableSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div aria-hidden="true" className="grid gap-2">
      {Array.from({ length: rows }, (_, i) => <Skeleton key={i} className="h-10" />)}
    </div>
  );
}
