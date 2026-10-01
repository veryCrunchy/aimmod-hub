import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { BenchmarkLeaderboardEntry, BenchmarkListItem } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { PageSkeleton } from "../components/ui/Skeleton";
import { PageHeader } from "../components/ui/PageHeader";
import { Button } from "../components/ui/Button";
import { Helmet } from "../lib/helmet";
import { PageStack } from "../components/ui/Stack";
import { fetchBenchmarkLeaderboard, fetchBenchmarkList } from "../lib/api";
import { groupBenchmarks, extractDifficulty } from "../lib/benchmarkGroups";

export function BenchmarkLeaderboardPage() {
  const { benchmarkId = "" } = useParams();
  const navigate = useNavigate();
  const [entries, setEntries] = useState<BenchmarkLeaderboardEntry[] | null>(null);
  const [benchmarkName, setBenchmarkName] = useState("");
  const [allBenchmarks, setAllBenchmarks] = useState<BenchmarkListItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Load leaderboard and benchmark list in parallel
  useEffect(() => {
    if (!benchmarkId) return;
    let cancelled = false;
    setEntries(null);
    setError(null);

    const leaderboardP = fetchBenchmarkLeaderboard(Number(benchmarkId))
      .then((res) => {
        if (!cancelled) {
          setEntries(res.entries ?? []);
          setBenchmarkName(res.benchmarkName || `Benchmark #${benchmarkId}`);
        }
      });

    const listP = fetchBenchmarkList()
      .then((res) => { if (!cancelled) setAllBenchmarks(res.benchmarks ?? []); })
      .catch(() => { /* sibling tabs are optional */ });

    Promise.all([leaderboardP, listP]).catch((err) => {
      if (!cancelled) setError(err instanceof Error ? err.message : "Could not load leaderboard.");
    });

    return () => { cancelled = true; };
  }, [benchmarkId]);

  // Find sibling benchmarks in the same series
  const siblings = useMemo(() => {
    if (!allBenchmarks || !benchmarkName) return null;
    const groups = groupBenchmarks(allBenchmarks);
    const group = groups.find((g) =>
      g.variants.some((v) => String(v.item.benchmarkId) === benchmarkId)
    );
    return group && group.variants.length > 1 ? group : null;
  }, [allBenchmarks, benchmarkName, benchmarkId]);

  if (error) {
    return (
      <PageStack>
        <PageHeader title="Benchmark" />
        <EmptyState title="This benchmark leaderboard could not be loaded.">
          <Button to="/benchmarks">All benchmarks</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!entries) {
    return <PageStack><PageSkeleton stats={0} rows={8} label="Loading benchmark leaderboard" /></PageStack>;
  }

  const { difficulty: currentDifficulty } = extractDifficulty(benchmarkName);
  const title = siblings ? siblings.base : (benchmarkName || `Benchmark #${benchmarkId}`);
  // How the ranked players spread across ranks, highest rank first.
  const rankSpread = [...entries.reduce((map, e) => {
    const key = e.overallRankName || "Unranked";
    const current = map.get(key) ?? { name: key, icon: e.overallRankIconUrl, index: e.overallRankIndex, count: 0 };
    current.count += 1;
    return map.set(key, current);
  }, new Map<string, { name: string; icon: string; index: number; count: number }>()).values()].sort((a, b) => b.index - a.index);

  return (
    <PageStack>
      <Helmet><title>{`${title} · Benchmark · AimMod Hub`}</title></Helmet>
      <PageSection>
        <PageHeader
          before={<Breadcrumb crumbs={[{ label: "Benchmarks", to: "/benchmarks" }, { label: title }]} />}
          title={currentDifficulty && siblings ? `${title} · ${currentDifficulty}` : title}
          meta={`${entries.length.toLocaleString()} ranked Hub ${entries.length === 1 ? "player" : "players"}`}
        />
        {/* Difficulty tabs — only shown when the benchmark has variants */}
        {siblings && (
          <div className="mt-4 flex flex-wrap gap-2">
            {siblings.variants.map(({ item, difficulty }) => {
              const isActive = String(item.benchmarkId) === benchmarkId;
              const label = difficulty ?? item.benchmarkName;
              return (
                <button
                  key={item.benchmarkId}
                  onClick={() => navigate(`/benchmarks/${item.benchmarkId}`)}
                  className={[
                    "rounded-full border px-4 py-1.5 text-[11px] font-medium uppercase tracking-[0.06em] transition-colors",
                    isActive
                      ? "border-cyan/40 bg-cyan/10 text-cyan"
                      : "border-line text-muted-2 hover:border-line/80 hover:text-text",
                  ].join(" ")}
                >
                  {label}
                  {item.playerCount > 0 && (
                    <span className={`ml-1.5 text-[11px] ${isActive ? "text-cyan/70" : "text-muted-2"}`}>
                      {item.playerCount}
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        )}
      </PageSection>

      {entries.length === 0 ? (
        <PageSection>
          <EmptyState title="No Hub players are ranked here yet." body="Ranks appear once a player links their Steam or KovaaK's account.">
            <Button to="/account">Link an account</Button>
          </EmptyState>
        </PageSection>
      ) : (
        <PageSection className="grid gap-4">
          {rankSpread.length > 1 && (
            <ul className="flex flex-wrap gap-2" aria-label="Players per rank">
              {rankSpread.map((rank) => (
                <li key={rank.name} className="flex items-center gap-2 rounded-md border border-line bg-panel px-3 py-1.5 text-sm">
                  {rank.icon && <img src={rank.icon} alt="" className="h-5 w-5 rounded object-cover" />}
                  <span>{rank.name}</span>
                  <span className="tabular-nums text-muted">{rank.count}</span>
                </li>
              ))}
            </ul>
          )}
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-line bg-white/2">
                  <th className="px-3 py-2 text-xs text-muted font-medium w-10">#</th>
                  <th className="px-3 py-2 text-xs text-muted font-medium">Player</th>
                  <th className="px-3 py-2 text-xs text-muted font-medium text-center">Rank</th>
                  <th className="px-3 py-2 text-xs text-muted font-medium text-right">Detail</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry, i) => (
                  <tr
                    key={entry.userHandle}
                    className="border-b border-white/4 last:border-0 hover:bg-white/[0.018] transition-colors"
                  >
                    <td className="px-3 py-2 text-sm text-muted-2 tabular-nums">{i + 1}</td>
                    <td className="px-3 py-2">
                      <Link
                        to={`/profiles/${entry.userHandle}`}
                        className="flex items-center gap-2 min-w-0 hover:text-cyan transition-colors"
                      >
                        {entry.avatarUrl && (
                          <img src={entry.avatarUrl} alt="" className="h-6 w-6 rounded-full border border-white/10 object-cover shrink-0" />
                        )}
                        <span className="text-sm font-medium text-text truncate">
                          {entry.displayName || entry.userHandle}
                        </span>
                        <span className="text-xs text-muted-2 shrink-0">@{entry.userHandle}</span>
                      </Link>
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex items-center justify-center gap-1.5">
                        {entry.overallRankIconUrl && (
                          <img src={entry.overallRankIconUrl} alt={entry.overallRankName} title={entry.overallRankName} className="h-5 w-5 rounded-md border border-white/10 object-cover" />
                        )}
                        <span className="text-sm text-text">{entry.overallRankName}</span>
                      </div>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <Link
                        to={`/profiles/${entry.userHandle}/benchmarks/${benchmarkId}`}
                        className="text-xs text-cyan hover:underline"
                      >
                        Scenario scores
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </PageSection>
      )}
    </PageStack>
  );
}
