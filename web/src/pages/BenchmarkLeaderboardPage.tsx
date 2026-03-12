import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { BenchmarkLeaderboardEntry } from "../gen/aimmod/hub/v1/hub_pb";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { Skeleton } from "../components/ui/Skeleton";
import { PageStack } from "../components/ui/Stack";
import { SectionHeader } from "../components/SectionHeader";
import { fetchBenchmarkLeaderboard } from "../lib/api";

export function BenchmarkLeaderboardPage() {
  const { benchmarkId = "" } = useParams();
  const [entries, setEntries] = useState<BenchmarkLeaderboardEntry[] | null>(null);
  const [benchmarkName, setBenchmarkName] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!benchmarkId) return;
    let cancelled = false;
    setEntries(null);
    setError(null);

    void fetchBenchmarkLeaderboard(Number(benchmarkId))
      .then((res) => {
        if (!cancelled) {
          setEntries(res.entries ?? []);
          setBenchmarkName(res.benchmarkName || `Benchmark #${benchmarkId}`);
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load leaderboard.");
      });

    return () => { cancelled = true; };
  }, [benchmarkId]);

  if (error) {
    return (
      <PageStack>
        <PageSection>
          <EmptyState title="Could not load leaderboard" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!entries) {
    return (
      <PageStack>
        <PageSection>
          <Skeleton className="mb-3 h-3 w-32" />
          <Skeleton className="mb-3 h-8 w-64" />
          <Skeleton className="h-4 w-48" />
        </PageSection>
        <PageSection>
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="mb-2 h-12" />
          ))}
        </PageSection>
      </PageStack>
    );
  }

  const title = benchmarkName || `Benchmark #${benchmarkId}`;

  return (
    <PageStack>
      <PageSection>
        <Breadcrumb
          crumbs={[
            { label: "Benchmarks", to: "/benchmarks" },
            { label: title },
          ]}
        />
        <SectionHeader
          eyebrow="Leaderboard"
          title={title}
          body={
            entries.length > 0
              ? `${entries.length} ranked player${entries.length !== 1 ? "s" : ""} on this hub.`
              : "No ranked players found for this benchmark."
          }
        />
      </PageSection>

      {entries.length === 0 ? (
        <PageSection>
          <EmptyState
            title="No ranked players"
            body="No hub players with a linked Steam account have ranked in this benchmark yet."
          />
        </PageSection>
      ) : (
        <PageSection>
          <div className="overflow-x-auto rounded-[14px] border border-line bg-white/2">
            <table className="w-full text-left border-collapse">
              <thead>
                <tr className="border-b border-line bg-white/2">
                  <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal w-10">#</th>
                  <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal">Player</th>
                  <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-center">Rank</th>
                  <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-right">Detail</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry, i) => (
                  <tr
                    key={entry.userHandle}
                    className="border-b border-white/4 last:border-0 hover:bg-white/[0.018] transition-colors"
                  >
                    <td className="px-3 py-2 text-[11px] text-muted/40 tabular-nums">{i + 1}</td>
                    <td className="px-3 py-2">
                      <Link
                        to={`/profiles/${entry.userHandle}`}
                        className="flex items-center gap-2 min-w-0 hover:text-cyan transition-colors"
                      >
                        {entry.avatarUrl && (
                          <img
                            src={entry.avatarUrl}
                            alt=""
                            className="h-6 w-6 rounded-full border border-white/10 object-cover shrink-0"
                          />
                        )}
                        <span className="text-[12px] font-medium text-text truncate">
                          {entry.displayName || entry.userHandle}
                        </span>
                        <span className="text-[11px] text-muted/50 shrink-0">@{entry.userHandle}</span>
                      </Link>
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex items-center justify-center gap-1.5">
                        {entry.overallRankIconUrl && (
                          <img
                            src={entry.overallRankIconUrl}
                            alt={entry.overallRankName}
                            title={entry.overallRankName}
                            className="h-5 w-5 rounded-md border border-white/10 object-cover"
                          />
                        )}
                        <span className="text-[11px] text-text/80">{entry.overallRankName}</span>
                      </div>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <Link
                        to={`/profiles/${entry.userHandle}/benchmarks/${benchmarkId}`}
                        className="text-[10px] text-muted/50 hover:text-cyan transition-colors"
                      >
                        View →
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
