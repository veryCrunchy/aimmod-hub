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

function hasRank(rankName?: string | null) {
  const n = rankName?.trim().toLowerCase();
  return Boolean(n && n !== "no rank");
}

function BenchmarkCard({ benchmark, handle }: { benchmark: BenchmarkSummary; handle: string }) {
  const rank = benchmark.overallRank;
  const hasR = hasRank(rank?.rankName);

  return (
    <Link
      to={`/profiles/${handle}/benchmarks/${benchmark.benchmarkId}`}
      className="group flex flex-col gap-3 rounded-[18px] border border-line bg-white/[0.025] p-4 transition-all hover:border-cyan/30 hover:bg-white/[0.04]"
    >
      {/* top row: icon + name */}
      <div className="flex items-start gap-3 min-w-0">
        {benchmark.benchmarkIconUrl ? (
          <img
            src={benchmark.benchmarkIconUrl}
            alt=""
            className="h-10 w-10 shrink-0 rounded-[10px] border border-white/10 object-cover"
          />
        ) : (
          <div className="h-10 w-10 shrink-0 rounded-[10px] border border-line bg-white/5 flex items-center justify-center text-[18px] text-muted/40">
            ◈
          </div>
        )}
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-text leading-tight truncate group-hover:text-cyan transition-colors">
            {benchmark.benchmarkName}
          </p>
          {benchmark.benchmarkType && (
            <p className="mt-0.5 text-[10px] text-muted/60 uppercase tracking-widest">
              {benchmark.benchmarkType}
            </p>
          )}
          {benchmark.benchmarkAuthor && (
            <p className="mt-0.5 text-[10px] text-muted/50">
              by {benchmark.benchmarkAuthor}
            </p>
          )}
        </div>
      </div>

      {/* rank row */}
      <div className="mt-auto flex items-center gap-2.5 rounded-[12px] border border-white/6 bg-black/20 px-3 py-2">
        {rank?.iconUrl ? (
          <img
            src={rank.iconUrl}
            alt=""
            className="h-8 w-8 rounded-lg border border-white/10 object-cover shrink-0"
          />
        ) : null}
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
        <span className="ml-auto text-[10px] text-muted/40 group-hover:text-cyan transition-colors shrink-0">
          →
        </span>
      </div>
    </Link>
  );
}

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

      {ranked.length > 0 ? (
        <PageSection>
          <p className="mb-4 text-[10px] uppercase tracking-widest text-cyan">
            Ranked — {ranked.length}
          </p>
          <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            {ranked.map((b) => (
              <BenchmarkCard key={b.benchmarkId} benchmark={b} handle={profile.userHandle} />
            ))}
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

      {unranked.length > 0 && (
        <PageSection>
          <p className="mb-4 text-[10px] uppercase tracking-widest text-muted/50">
            Unranked — {unranked.length}
          </p>
          <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            {unranked.map((b) => (
              <BenchmarkCard key={b.benchmarkId} benchmark={b} handle={profile.userHandle} />
            ))}
          </Grid>
        </PageSection>
      )}
    </PageStack>
  );
}
