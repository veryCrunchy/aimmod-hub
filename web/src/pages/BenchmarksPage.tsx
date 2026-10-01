import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { BenchmarkSummary, GetProfileResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageSkeleton } from "../components/ui/Skeleton";
import { Grid, PageStack } from "../components/ui/Stack";
import { hasRank as hasRealRank } from "../components/BenchmarkCards";
import { PageHeader } from "../components/ui/PageHeader";
import { Button } from "../components/ui/Button";
import { Helmet } from "../lib/helmet";
import { fetchBenchmarkList, fetchProfile } from "../lib/api";
import { useUrlState } from "../lib/urlState";
import { groupBenchmarks, type BenchmarkGroup } from "../lib/benchmarkGroups";

function hasRank(rankName?: string | null) {
  return hasRealRank({ rankName });
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
          <div className="h-10 w-10 shrink-0 rounded-[10px] border border-line bg-white/5 flex items-center justify-center text-[18px] text-muted-2">◈</div>
        )}
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-text leading-tight truncate group-hover:text-cyan transition-colors">
            {benchmark.benchmarkName}
          </p>
          {benchmark.benchmarkType && (
            <p className="mt-0.5 text-[10px] text-muted-2 uppercase tracking-widest">{benchmark.benchmarkType}</p>
          )}
          {benchmark.benchmarkAuthor && (
            <p className="mt-0.5 text-[10px] text-muted-2">by {benchmark.benchmarkAuthor}</p>
          )}
        </div>
      </div>
      <div className="mt-auto flex items-center gap-2.5 rounded-xl border border-white/6 bg-black/20 px-3 py-2">
        {rank?.iconUrl && <img src={rank.iconUrl} alt="" className="h-8 w-8 rounded-lg border border-white/10 object-cover shrink-0" />}
        <div className="min-w-0">
          {hasR ? (
            <>
              <p className="text-[12px] font-medium text-text truncate">{rank?.rankName}</p>
              <p className="text-[11px] text-muted-2 uppercase tracking-widest">Current rank</p>
            </>
          ) : (
            <p className="text-[11px] text-muted-2 italic">No rank yet</p>
          )}
        </div>
        <span className="ml-auto text-[10px] text-muted-2 group-hover:text-cyan transition-colors shrink-0">→</span>
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
          <div className="h-10 w-10 shrink-0 rounded-[10px] border border-line bg-white/5 flex items-center justify-center text-[18px] text-muted-2">◈</div>
        )}
        <div className="min-w-0">
          <p className="text-[12px] font-medium text-text leading-tight truncate">{group.base}</p>
          {group.type && (
            <p className="mt-0.5 text-[10px] text-muted-2 uppercase tracking-widest">{group.type}</p>
          )}
          {group.author && (
            <p className="mt-0.5 text-[10px] text-muted-2">by {group.author}</p>
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
              <span className="w-24 shrink-0 text-[10px] text-muted-2 uppercase tracking-widest truncate">
                {difficulty ?? item.benchmarkName}
              </span>

              {/* rank */}
              <div className="flex items-center gap-1.5 min-w-0 flex-1">
                {rank?.iconUrl && (
                  <img src={rank.iconUrl} alt="" className="h-5 w-5 shrink-0 rounded-md border border-white/10 object-cover" />
                )}
                <span className={`text-[11px] font-medium truncate ${hasR ? "text-text" : "text-muted-2 italic"}`}>
                  {hasR ? rank?.rankName : "No rank yet"}
                </span>
              </div>

              <span className="shrink-0 text-[10px] text-muted-2 group-hover:text-cyan transition-colors">→</span>
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
  const [hiddenIds, setHiddenIds] = useState<Set<number>>(new Set());
  const [state, setState] = useUrlState({ all: "" });

  useEffect(() => {
    let cancelled = false;
    void fetchBenchmarkList().then((r) => { if (!cancelled) setHiddenIds(new Set(r.benchmarks.filter((b) => b.hidden).map((b) => b.benchmarkId))); }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

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
        <PageHeader title="Benchmarks" />
        <EmptyState title="This player could not be found.">
          <Button to="/benchmarks">All benchmarks</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!profile) {
    return <PageStack><PageSkeleton stats={0} label="Loading benchmarks" /></PageStack>;
  }

  const allRanked = profile.benchmarks.filter((b) => hasRank(b.overallRank?.rankName));
  const hiddenCount = allRanked.filter((b) => hiddenIds.has(b.benchmarkId)).length;
  const ranked = state.all ? allRanked : allRanked.filter((b) => !hiddenIds.has(b.benchmarkId));
  const rankedGroups = groupBenchmarks(ranked);

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
      <Helmet><title>{`${profile.userDisplayName || profile.userHandle} · Benchmarks · AimMod Hub`}</title></Helmet>
      <PageHeader
        before={<Breadcrumb crumbs={[{ label: profile.userDisplayName || profile.userHandle, to: `/profiles/${profile.userHandle}` }, { label: "Benchmarks" }]} />}
        title="Benchmark ranks"
        meta={ranked.length > 0 ? `${profile.userDisplayName || profile.userHandle} is ranked in ${ranked.length} ${ranked.length === 1 ? "benchmark" : "benchmarks"}` : undefined}
        actions={hiddenCount ? (
          <button type="button" className="text-sm text-cyan hover:underline" onClick={() => setState({ all: state.all ? "" : "1" })}>
            {state.all ? "Hide test and empty benchmarks" : `Show all (${hiddenCount} hidden)`}
          </button>
        ) : null}
      />

      {rankedGroups.length > 0 ? (
        <PageSection>
          <Grid className="grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            {rankedGroups.map(renderGroup)}
          </Grid>
        </PageSection>
      ) : (
        <PageSection>
          <EmptyState title="No benchmark ranks yet." body="Ranks come from KovaaK's once a Steam or KovaaK's account is linked.">
            <Button to="/benchmarks">Browse benchmarks</Button>
          </EmptyState>
        </PageSection>
      )}
    </PageStack>
  );
}
