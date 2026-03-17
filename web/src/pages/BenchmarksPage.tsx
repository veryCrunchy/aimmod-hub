import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { BenchmarkSummary, GetProfileResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { SectionHeader } from "../components/SectionHeader";
import { fetchProfile } from "../lib/api";
import { groupBenchmarks, type BenchmarkGroup } from "../lib/benchmarkGroups";

function hasRank(rankName?: string | null) {
  const n = rankName?.trim().toLowerCase();
  return Boolean(n && n !== "no rank");
}

type BenchmarkGroupSummary = BenchmarkGroup<BenchmarkSummary>;

// ─── cards ────────────────────────────────────────────────────────────────────

/** Single benchmark (no grouping needed). */
function BenchmarkCard({ benchmark, handle }: { benchmark: BenchmarkSummary; handle: string }) {
  const rank = benchmark.overallRank;
  const hasR = hasRank(rank?.rankName);

  return (
    <Link
      to={`/profiles/${handle}/benchmarks/${benchmark.benchmarkId}`}
      className="group flex flex-col gap-3 rounded-[18px] border border-line bg-white/2.5 p-4 transition-all hover:border-cyan/30 hover:bg-white/4"
    >
      <div className="flex items-start gap-3 min-w-0">
        {benchmark.benchmarkIconUrl ? (
          <img src={benchmark.benchmarkIconUrl} alt="" className="h-10 w-10 shrink-0 rounded-[10px] border border-white/10 object-cover" />
        ) : (
          <div className="h-10 w-10 shrink-0 rounded-[10px] border border-line bg-white/5 flex items-center justify-center text-[18px] text-muted/40">◈</div>
        )}
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-text leading-tight truncate group-hover:text-cyan transition-colors">
            {benchmark.benchmarkName}
          </p>
          {benchmark.benchmarkType && (
            <p className="mt-0.5 text-[10px] text-muted/60 uppercase tracking-widest">{benchmark.benchmarkType}</p>
          )}
          {benchmark.benchmarkAuthor && (
            <p className="mt-0.5 text-[10px] text-muted/50">by {benchmark.benchmarkAuthor}</p>
          )}
        </div>
      </div>
      <div className="mt-auto flex items-center gap-2.5 rounded-xl border border-white/6 bg-black/20 px-3 py-2">
        {rank?.iconUrl && <img src={rank.iconUrl} alt="" className="h-8 w-8 rounded-lg border border-white/10 object-cover shrink-0" />}
        <div className="min-w-0">
          {hasR ? (
            <>
              <p className="text-[12px] font-medium text-text truncate">{rank?.rankName}</p>
              <p className="text-[9px] text-muted/50 uppercase tracking-widest">Current rank</p>
            </>
          ) : (
            <p className="text-[11px] text-muted/40 italic">No rank yet</p>
          )}
        </div>
        <span className="ml-auto text-[10px] text-muted/40 group-hover:text-cyan transition-colors shrink-0">→</span>
      </div>
    </Link>
  );
}

/** Multiple difficulty variants of the same benchmark series. */
function BenchmarkGroupCard({ group, handle }: { group: BenchmarkGroupSummary; handle: string }) {
  return (
    <div className="flex flex-col rounded-[18px] border border-line bg-white/2.5 overflow-hidden">
      {/* header */}
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

      {/* variant rows */}
      <div className="border-t border-white/6 divide-y divide-white/4">
        {group.variants.map(({ item, difficulty }) => {
          const rank = item.overallRank;
          const hasR = hasRank(rank?.rankName);
          return (
            <Link
              key={item.benchmarkId}
              to={`/profiles/${handle}/benchmarks/${item.benchmarkId}`}
              className="flex items-center gap-3 px-4 py-2.5 hover:bg-white/3 transition-colors group"
            >
              {/* difficulty label */}
              <span className="w-24 shrink-0 text-[10px] text-muted/60 uppercase tracking-widest truncate">
                {difficulty ?? item.benchmarkName}
              </span>

              {/* rank */}
              <div className="flex items-center gap-1.5 min-w-0 flex-1">
                {rank?.iconUrl && (
                  <img src={rank.iconUrl} alt="" className="h-5 w-5 shrink-0 rounded-md border border-white/10 object-cover" />
                )}
                <span className={`text-[11px] font-medium truncate ${hasR ? "text-text" : "text-muted/40 italic"}`}>
                  {hasR ? rank?.rankName : "No rank yet"}
                </span>
              </div>

              <span className="shrink-0 text-[10px] text-muted/30 group-hover:text-cyan transition-colors">→</span>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

// ─── page ─────────────────────────────────────────────────────────────────────

export function BenchmarksPage() {
  const { handle = "" } = useParams();
  const [profile, setProfile] = useState<GetProfileResponse | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setProfile(null);
    setError(null);
    void fetchProfile(handle)
      .then((next) => { if (!cancelled) setProfile(next); })
      .catch((err) => { if (!cancelled) setError(err instanceof Error ? err.message : "Could not load benchmarks."); });
    return () => { cancelled = true; };
  }, [handle]);

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <EmptyState title="Could not load benchmarks" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!profile) {
    return (
      <PageStack>
        <PageSection>
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="mb-3 h-8 w-64" />
          <Skeleton className="h-4 w-48" />
        </PageSection>
        <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-32" />
          ))}
        </Grid>
      </PageStack>
    );
  }

  const ranked = profile.benchmarks.filter((b) => hasRank(b.overallRank?.rankName));
  const unranked = profile.benchmarks.filter((b) => !hasRank(b.overallRank?.rankName));
  const rankedGroups = groupBenchmarks(ranked);
  const unrankedGroups = groupBenchmarks(unranked);

  function renderGroup(group: BenchmarkGroupSummary) {
    if (group.variants.length === 1) {
      return (
        <BenchmarkCard
          key={group.variants[0].item.benchmarkId}
          benchmark={group.variants[0].item}
          handle={profile!.userHandle}
        />
      );
    }
    return (
      <BenchmarkGroupCard
        key={group.base + group.iconUrl}
        group={group}
        handle={profile!.userHandle}
      />
    );
  }

  return (
    <PageStack>
      <PageSection>
        <Breadcrumb
          crumbs={[
            { label: "Community", to: "/community" },
            { label: `@${profile.userHandle}`, to: `/profiles/${profile.userHandle}` },
            { label: "Benchmarks" },
          ]}
        />
        <SectionHeader
          eyebrow="Benchmarks"
          title={`${profile.userDisplayName || profile.userHandle}'s benchmarks`}
          body={
            ranked.length > 0
              ? `Ranked in ${ranked.length} benchmark${ranked.length !== 1 ? "s" : ""}.`
              : "No benchmark ranks yet."
          }
        />
      </PageSection>

      {rankedGroups.length > 0 ? (
        <PageSection>
          <p className="mb-4 text-[10px] uppercase tracking-widest text-cyan">
            Ranked — {ranked.length}
          </p>
          <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            {rankedGroups.map(renderGroup)}
          </Grid>
        </PageSection>
      ) : (
        <PageSection>
          <EmptyState
            title="No ranked benchmarks"
            body={`@${profile.userHandle} hasn't ranked in any benchmarks yet.`}
          />
        </PageSection>
      )}

      {unrankedGroups.length > 0 && (
        <PageSection>
          <p className="mb-4 text-[10px] uppercase tracking-widest text-muted/50">
            Unranked — {unranked.length}
          </p>
          <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            {unrankedGroups.map(renderGroup)}
          </Grid>
        </PageSection>
      )}
    </PageStack>
  );
}
