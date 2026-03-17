import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { BenchmarkListItem } from "../gen/aimmod/hub/v1/hub_pb";
import { SectionHeader } from "../components/SectionHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { fetchBenchmarkList } from "../lib/api";
import { groupBenchmarks, type BenchmarkGroup } from "../lib/benchmarkGroups";

type BenchmarkGroupItem = BenchmarkGroup<BenchmarkListItem>;

function BenchmarkCard({ b }: { b: BenchmarkListItem }) {
  return (
    <Link
      to={`/benchmarks/${b.benchmarkId}`}
      className="group flex flex-col gap-3 rounded-[18px] border border-line bg-white/2.5 p-4 transition-all hover:border-cyan/30 hover:bg-white/4"
    >
      <div className="flex items-start gap-3 min-w-0">
        {b.benchmarkIconUrl ? (
          <img src={b.benchmarkIconUrl} alt="" className="h-10 w-10 shrink-0 rounded-[10px] border border-white/10 object-cover" />
        ) : (
          <div className="h-10 w-10 shrink-0 rounded-[10px] border border-line bg-white/5 flex items-center justify-center text-[18px] text-muted/40">◈</div>
        )}
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-text leading-tight truncate group-hover:text-cyan transition-colors">
            {b.benchmarkName}
          </p>
          {b.benchmarkType && (
            <p className="mt-0.5 text-[10px] text-muted/60 uppercase tracking-widest">{b.benchmarkType}</p>
          )}
          {b.benchmarkAuthor && (
            <p className="mt-0.5 text-[10px] text-muted/50">by {b.benchmarkAuthor}</p>
          )}
        </div>
      </div>
      <div className="mt-auto flex items-center justify-between rounded-xl border border-white/6 bg-black/20 px-3 py-2">
        <p className="text-[11px] text-muted/60">{b.playerCount} player{b.playerCount !== 1 ? "s" : ""} ranked</p>
        <span className="text-[10px] text-muted/40 group-hover:text-cyan transition-colors">→</span>
      </div>
    </Link>
  );
}

function BenchmarkGroupCard({ group }: { group: BenchmarkGroupItem }) {
  return (
    <div className="flex flex-col rounded-[18px] border border-line bg-white/2.5 overflow-hidden">
      <div className="flex items-start gap-3 p-4 pb-3">
        {group.iconUrl ? (
          <img src={group.iconUrl} alt="" className="h-10 w-10 shrink-0 rounded-[10px] border border-white/10 object-cover" />
        ) : (
          <div className="h-10 w-10 shrink-0 rounded-[10px] border border-line bg-white/5 flex items-center justify-center text-[18px] text-muted/40">◈</div>
        )}
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-text leading-tight truncate">{group.base}</p>
          {group.type && (
            <p className="mt-0.5 text-[10px] text-muted/60 uppercase tracking-widest">{group.type}</p>
          )}
          {group.author && (
            <p className="mt-0.5 text-[10px] text-muted/50">by {group.author}</p>
          )}
        </div>
      </div>
      <div className="border-t border-white/6 divide-y divide-white/4">
        {group.variants.map(({ item, difficulty }) => (
          <Link
            key={item.benchmarkId}
            to={`/benchmarks/${item.benchmarkId}`}
            className="flex items-center gap-3 px-4 py-2.5 hover:bg-white/3 transition-colors group"
          >
            <span className="w-24 shrink-0 text-[10px] text-muted/60 uppercase tracking-widest truncate">
              {difficulty ?? item.benchmarkName}
            </span>
            <span className="flex-1 text-[11px] text-muted/50 tabular-nums">
              {item.playerCount} player{item.playerCount !== 1 ? "s" : ""}
            </span>
            <span className="shrink-0 text-[10px] text-muted/30 group-hover:text-cyan transition-colors">→</span>
          </Link>
        ))}
      </div>
    </div>
  );
}

export function GlobalBenchmarksPage() {
  const [benchmarks, setBenchmarks] = useState<BenchmarkListItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void fetchBenchmarkList()
      .then((res) => { if (!cancelled) setBenchmarks(res.benchmarks ?? []); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load benchmarks."); });
    return () => { cancelled = true; };
  }, []);

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <EmptyState title="Could not load benchmarks" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!benchmarks) {
    return (
      <PageStack>
        <PageSection>
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="mb-3 h-8 w-64" />
          <Skeleton className="h-4 w-48" />
        </PageSection>
        <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-28" />
          ))}
        </Grid>
      </PageStack>
    );
  }

  const groups = groupBenchmarks(benchmarks);

  return (
    <PageStack>
      <PageSection>
        <SectionHeader
          eyebrow="Benchmarks"
          title="All benchmarks"
          body={
            benchmarks.length > 0
              ? `${benchmarks.length} benchmark${benchmarks.length !== 1 ? "s" : ""} tracked across hub players.`
              : "No benchmark data found yet."
          }
        />
      </PageSection>

      {groups.length === 0 ? (
        <PageSection>
          <EmptyState
            title="No benchmarks yet"
            body="Benchmarks appear here once hub players with linked Steam accounts have been ranked."
          />
        </PageSection>
      ) : (
        <PageSection>
          <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            {groups.map((group) =>
              group.variants.length === 1 ? (
                <BenchmarkCard key={group.variants[0].item.benchmarkId} b={group.variants[0].item} />
              ) : (
                <BenchmarkGroupCard key={group.base + group.iconUrl} group={group} />
              )
            )}
          </Grid>
        </PageSection>
      )}
    </PageStack>
  );
}
