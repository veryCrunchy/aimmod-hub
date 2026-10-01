import { useEffect, useState } from "react";
import { cn } from "../../lib/cn";

type PagerProps = {
  /** Zero-based page. */
  page: number;
  pageSize: number;
  total: number;
  onPage: (page: number) => void;
  pageSizes?: readonly number[];
  onPageSize?: (size: number) => void;
  className?: string;
  label?: string;
};

export function pageCount(total: number, pageSize: number) {
  return Math.max(1, Math.ceil(total / Math.max(1, pageSize)));
}

const button = "inline-flex min-h-9 min-w-9 items-center justify-center rounded-md border border-line bg-panel px-2 text-sm text-text transition-colors hover:border-line-strong disabled:cursor-not-allowed disabled:opacity-40";

/** First, previous, page input, next and last, with an optional page size. */
export function Pager({ page, pageSize, total, onPage, pageSizes, onPageSize, className, label = "Pages" }: PagerProps) {
  const pages = pageCount(total, pageSize);
  const [draft, setDraft] = useState(String(page + 1));
  useEffect(() => setDraft(String(page + 1)), [page]);
  const go = (next: number) => onPage(Math.min(pages - 1, Math.max(0, next)));
  const first = total === 0 ? 0 : page * pageSize + 1;
  const last = Math.min(total, (page + 1) * pageSize);
  return (
    <nav aria-label={label} className={cn("flex flex-wrap items-center justify-between gap-2 text-sm", className)}>
      <span className="text-muted tabular-nums">{total ? `${first.toLocaleString()}–${last.toLocaleString()} of ${total.toLocaleString()}` : "No entries"}</span>
      <div className="flex flex-wrap items-center gap-1.5">
        {pageSizes && onPageSize ? (
          <label className="mr-1 flex items-center gap-1.5 text-muted">
            <span className="max-sm:sr-only">Show</span>
            <select value={pageSize} onChange={(e) => onPageSize(Number(e.target.value))} className="min-h-9 rounded-md border border-line bg-panel px-2 text-text">
              {pageSizes.map((size) => <option key={size} value={size}>{size}</option>)}
            </select>
          </label>
        ) : null}
        <button type="button" className={button} onClick={() => go(0)} disabled={page <= 0} aria-label="First page">«</button>
        <button type="button" className={button} onClick={() => go(page - 1)} disabled={page <= 0} aria-label="Previous page">‹</button>
        <form onSubmit={(e) => { e.preventDefault(); const n = Number.parseInt(draft, 10); if (Number.isFinite(n)) go(n - 1); }} className="flex items-center gap-1.5 text-muted">
          <input aria-label="Page number" inputMode="numeric" value={draft} onChange={(e) => setDraft(e.target.value.replace(/[^0-9]/g, ""))} onBlur={() => { const n = Number.parseInt(draft, 10); if (Number.isFinite(n) && n - 1 !== page) go(n - 1); else setDraft(String(page + 1)); }}
            className="min-h-9 w-14 rounded-md border border-line bg-panel px-2 text-center text-text tabular-nums" />
          <span className="tabular-nums">of {pages.toLocaleString()}</span>
        </form>
        <button type="button" className={button} onClick={() => go(page + 1)} disabled={page >= pages - 1} aria-label="Next page">›</button>
        <button type="button" className={button} onClick={() => go(pages - 1)} disabled={page >= pages - 1} aria-label="Last page">»</button>
      </div>
    </nav>
  );
}
