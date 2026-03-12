import { Link, useNavigate } from "react-router-dom";
import type { BenchmarkSummary, ScenarioBenchmarkRank } from "../gen/aimmod/hub/v1/hub_pb";
import { ScrollArea } from "./ui/ScrollArea";

function imageUrl(value?: string) {
  return value?.trim() ? value : "";
}

function hasRank(rank?: { rankName?: string | undefined | null } | null) {
  const name = rank?.rankName?.trim();
  return Boolean(name && name.toLowerCase() !== "no rank");
}

// ─── BenchmarkSummaryGrid ─────────────────────────────────────────────────────

export function BenchmarkSummaryGrid({
  benchmarks,
  handle,
}: {
  benchmarks: BenchmarkSummary[];
  handle?: string;
}) {
  const visibleBenchmarks = benchmarks.filter((b) => hasRank(b.overallRank));
  if (visibleBenchmarks.length === 0) return null;

  return (
    <ScrollArea className="max-h-[min(56vh,640px)] pr-1">
      <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3">
        {visibleBenchmarks.map((benchmark) => {
          const rank = benchmark.overallRank;
          const body = (
            <>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-[10px] uppercase tracking-widest text-cyan">Benchmark</p>
                  <p className="mt-1 break-words text-sm font-medium text-text">{benchmark.benchmarkName}</p>
                  {benchmark.benchmarkType && (
                    <p className="mt-0.5 text-[10px] text-muted/60">{benchmark.benchmarkType}</p>
                  )}
                </div>
                {imageUrl(benchmark.benchmarkIconUrl) && (
                  <img
                    src={benchmark.benchmarkIconUrl}
                    alt=""
                    className="h-10 w-10 rounded-xl border border-white/10 bg-black/20 object-cover shrink-0"
                  />
                )}
              </div>
              <div className="mt-3 flex items-center gap-2.5 rounded-xl border border-white/8 bg-black/15 px-3 py-2">
                {imageUrl(rank?.iconUrl) && (
                  <img
                    src={rank?.iconUrl}
                    alt=""
                    className="h-8 w-8 rounded-lg border border-white/10 bg-black/20 object-cover shrink-0"
                  />
                )}
                <div className="min-w-0">
                  <p className="truncate text-[12px] font-medium text-text">{rank?.rankName}</p>
                  <p className="mt-0.5 text-[9px] uppercase tracking-widest text-muted/50">Current rank</p>
                </div>
                {handle && (
                  <span className="ml-auto text-[10px] text-muted/40 shrink-0">→</span>
                )}
              </div>
            </>
          );

          return handle ? (
            <Link
              key={`${benchmark.benchmarkId}:${benchmark.benchmarkName}`}
              to={`/profiles/${handle}/benchmarks/${benchmark.benchmarkId}`}
              className="block rounded-[14px] border border-white/8 bg-white/3 px-4 py-3 transition-colors hover:border-cyan/30 hover:bg-white/[0.05]"
            >
              {body}
            </Link>
          ) : (
            <div key={`${benchmark.benchmarkId}:${benchmark.benchmarkName}`} className="rounded-[14px] border border-white/8 bg-white/3 px-4 py-3">
              {body}
            </div>
          );
        })}
      </div>
    </ScrollArea>
  );
}

// ─── ScenarioBenchmarkRankList ────────────────────────────────────────────────
// Shown on PlayerScenarioPage and RunPage — compact table of ranks across
// all benchmarks that include this scenario.

export function ScenarioBenchmarkRankList({
  ranks,
  handle,
}: {
  title: string;
  ranks: ScenarioBenchmarkRank[];
  handle?: string;
}) {
  const navigate = useNavigate();
  const visibleRanks = ranks.filter((r) => hasRank(r.scenarioRank));
  if (visibleRanks.length === 0) return null;

  return (
    <div className="overflow-x-auto rounded-[14px] border border-line bg-white/2">
      <table className="w-full text-left border-collapse">
        <thead>
          <tr className="border-b border-line bg-white/2">
            <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal">Benchmark</th>
            <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal">Category</th>
            <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-right">Score</th>
            <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-center">Rank</th>
            <th className="px-3 py-2 text-[9px] uppercase tracking-widest text-muted/50 font-normal text-right">LB</th>
          </tr>
        </thead>
        <tbody>
          {visibleRanks.map((rank) => {
            const row = (
              <>
                {/* Benchmark name + icon */}
                <td className="px-3 py-2">
                  <div className="flex items-center gap-2 min-w-0">
                    {imageUrl(rank.benchmarkIconUrl) && (
                      <img
                        src={rank.benchmarkIconUrl}
                        alt=""
                        className="h-5 w-5 rounded-md border border-white/10 object-cover shrink-0"
                      />
                    )}
                    <span className="text-[11.5px] font-medium text-text/85 truncate">
                      {rank.benchmarkName}
                    </span>
                  </div>
                </td>
                {/* Category */}
                <td className="px-3 py-2 text-[11px] text-muted/70 whitespace-nowrap">
                  {rank.categoryName || "—"}
                </td>
                {/* Score */}
                <td className="px-3 py-2 text-right">
                  <span className="text-[12px] font-medium text-text tabular-nums">
                    {Math.round(rank.scenarioScore).toLocaleString()}
                  </span>
                </td>
                {/* Rank badge */}
                <td className="px-3 py-2">
                  <div className="flex items-center justify-center gap-1.5">
                    {imageUrl(rank.scenarioRank?.iconUrl) ? (
                      <img
                        src={rank.scenarioRank?.iconUrl}
                        alt={rank.scenarioRank?.rankName ?? ""}
                        title={rank.scenarioRank?.rankName ?? undefined}
                        className="h-5 w-5 rounded-md border border-white/10 object-cover"
                      />
                    ) : (
                      <span className="text-[10px] text-muted">{rank.scenarioRank?.rankName || "—"}</span>
                    )}
                  </div>
                </td>
                {/* Leaderboard rank */}
                <td className="px-3 py-2 text-right">
                  <span className="text-[10px] text-muted/60 tabular-nums">
                    {rank.leaderboardRank > 0 ? `#${rank.leaderboardRank.toLocaleString()}` : "—"}
                  </span>
                </td>
              </>
            );

            return handle ? (
              <tr
                key={`${rank.benchmarkId}:${rank.categoryName}:${rank.benchmarkName}`}
                className="border-b border-white/4 last:border-0 hover:bg-white/[0.018] transition-colors cursor-pointer"
                onClick={() => navigate(`/profiles/${handle}/benchmarks/${rank.benchmarkId}`)}
              >
                {row}
              </tr>
            ) : (
              <tr
                key={`${rank.benchmarkId}:${rank.categoryName}:${rank.benchmarkName}`}
                className="border-b border-white/4 last:border-0"
              >
                {row}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
