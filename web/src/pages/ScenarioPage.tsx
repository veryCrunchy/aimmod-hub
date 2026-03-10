import { useCallback, useEffect, useState } from "react";
import { Helmet } from "react-helmet-async";
import { Link, useParams } from "react-router-dom";
import type { GetScenarioPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { ScoreDistributionChart } from "../components/charts/ScoreDistributionChart";
import { ScenarioTypeBadge } from "../components/ScenarioTypeBadge";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { Skeleton } from "../components/ui/Skeleton";
import { SortableTh } from "../components/ui/SortableTh";
import { Grid, PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { fetchReplayHub, fetchScenarioPage, formatDurationMs, formatRelativeTime, type HubSearchRun } from "../lib/api";

type SortField = "score" | "accuracy" | "date";
type Tab = "recent" | "leaderboard";
const PAGE_SIZE = 15;

function consistencyLabel(cv: number): { label: string; color: string; detail: string } {
  if (cv < 0.08) return { label: "Very consistent", color: "text-mint", detail: "Scores cluster tightly — repeatable on this scenario." };
  if (cv < 0.18) return { label: "Consistent", color: "text-cyan", detail: "Low spread — players tend to hit similar scores." };
  if (cv < 0.32) return { label: "Variable", color: "text-gold", detail: "Moderate spread — results depend on the session." };
  return { label: "Highly variable", color: "text-danger", detail: "Wide spread — scores fluctuate a lot across players." };
}

function ScenarioIntelligence({ page }: { page: GetScenarioPageResponse }) {
  const scores = page.recentRuns.map((r) => r.score);
  const uniquePlayers = new Set(page.recentRuns.map((r) => r.userHandle || r.userDisplayName)).size;
  const sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
  const recentActivity = page.recentRuns.filter((r) => new Date(r.playedAtIso).getTime() >= sevenDaysAgo).length;

  const mean = scores.length > 1 ? scores.reduce((a, b) => a + b, 0) / scores.length : 0;
  const variance = scores.length > 1 ? scores.reduce((a, b) => a + (b - mean) ** 2, 0) / scores.length : 0;
  const stddev = Math.sqrt(variance);
  const cv = mean > 0 ? stddev / mean : 0;
  const consistency = scores.length >= 4 ? consistencyLabel(cv) : null;

  const ceilingPct = page.bestScore > 0 ? Math.round((page.averageScore / page.bestScore) * 100) : null;
  const leader = page.topRuns[0];

  return (
    <div className="grid gap-3">
      {/* stat tiles */}
      <div className="grid grid-cols-2 gap-3 max-[560px]:grid-cols-1">
        <div className="min-w-0 rounded-[14px] border border-line bg-white/2 p-3.5">
          <p className="text-[10px] uppercase tracking-[0.1em] text-muted-2">Active players</p>
          <p className="mt-1.5 text-2xl font-medium text-text">{uniquePlayers}</p>
          <p className="mt-1 text-[11px] text-muted">across last {page.recentRuns.length} runs</p>
        </div>
        <div className="min-w-0 rounded-[14px] border border-line bg-white/2 p-3.5">
          <p className="text-[10px] uppercase tracking-[0.1em] text-muted-2">Last 7 days</p>
          <p className="mt-1.5 text-2xl font-medium text-text">{recentActivity}</p>
          <p className="mt-1 text-[11px] text-muted">{recentActivity === 1 ? "run" : "runs"} recorded</p>
        </div>
      </div>

      {/* consistency */}
      {consistency && (
        <div className="min-w-0 rounded-[14px] border border-line bg-white/2 p-3.5">
          <p className="text-[10px] uppercase tracking-[0.1em] text-muted-2">Score consistency</p>
          <div className="mt-2 flex items-center gap-3">
            <div className="flex-1">
              <div className="mb-1.5 h-1.5 w-full rounded-full bg-white/8 overflow-hidden">
                <div
                  className="h-full rounded-full bg-mint/60 transition-all"
                  style={{ width: `${Math.max(4, Math.round((1 - Math.min(cv, 0.5) / 0.5) * 100))}%` }}
                />
              </div>
              <p className={`text-sm font-medium ${consistency.color}`}>{consistency.label}</p>
            </div>
            <p className="shrink-0 text-right text-[11px] text-muted-2">σ {Math.round(stddev).toLocaleString()}</p>
          </div>
          <p className="mt-2 text-[11px] text-muted">{consistency.detail}</p>
        </div>
      )}

      {/* score ceiling gap */}
      {ceilingPct !== null && page.bestScore > 0 && (
        <div className="min-w-0 rounded-[14px] border border-line bg-white/2 p-3.5">
          <p className="mb-3 text-[10px] uppercase tracking-[0.1em] text-muted-2">Score ceiling gap</p>
          <div className="flex items-end justify-between gap-2">
            <div>
              <p className="text-lg font-medium text-text">{Math.round(page.bestScore).toLocaleString()}</p>
              <p className="text-[11px] text-muted-2">best</p>
            </div>
            <div className="relative mx-3 h-px flex-1 bg-line">
              <div
                className="absolute inset-y-0 left-0 h-px bg-mint/40"
                style={{ width: `${Math.min(100, ceilingPct)}%` }}
              />
            </div>
            <div className="text-right">
              <p className="text-lg font-medium text-gold">{Math.round(page.averageScore).toLocaleString()}</p>
              <p className="text-[11px] text-muted-2">avg</p>
            </div>
          </div>
          <p className="mt-2 text-[11px] text-muted-2">
            Average is {ceilingPct}% of best — {ceilingPct >= 80 ? "ceiling room is tight" : ceilingPct >= 65 ? "moderate room above average" : "large gap to close"}.
          </p>
        </div>
      )}

      {/* leader card */}
      {leader && (
        <Link
          to={`/runs/${leader.runId || leader.sessionId}`}
          className="rounded-[14px] border border-gold/20 bg-[rgba(255,215,0,0.03)] p-4 transition-colors hover:border-gold/40 hover:bg-[rgba(255,215,0,0.05)]"
        >
          <p className="text-[10px] uppercase tracking-[0.1em] text-gold/70">Top score</p>
          <div className="mt-2 flex items-center justify-between gap-3">
            <div>
              <p className="text-2xl font-medium text-gold">{Math.round(leader.score).toLocaleString()}</p>
              <p className="mt-0.5 text-sm text-muted">{leader.userDisplayName || leader.userHandle}</p>
            </div>
            <div className="text-right">
              <p className="text-sm text-text">{leader.accuracy.toFixed(1)}%</p>
              <p className="text-[11px] text-muted-2">accuracy</p>
            </div>
          </div>
        </Link>
      )}
    </div>
  );
}

function ScenarioPlayerLeaders({ page }: { page: GetScenarioPageResponse }) {
  const byPlayer = new Map<string, {
    handle: string;
    displayName: string;
    runs: number;
    bestScore: number;
    averageScore: number;
    averageAccuracy: number;
  }>();

  for (const run of [...page.recentRuns, ...page.topRuns]) {
    const handle = run.userHandle || run.userDisplayName;
    if (!handle) continue;
    const current = byPlayer.get(handle) ?? {
      handle,
      displayName: run.userDisplayName || run.userHandle,
      runs: 0,
      bestScore: 0,
      averageScore: 0,
      averageAccuracy: 0,
    };
    current.runs += 1;
    current.bestScore = Math.max(current.bestScore, run.score);
    current.averageScore += run.score;
    current.averageAccuracy += run.accuracy;
    byPlayer.set(handle, current);
  }

  const leaders = [...byPlayer.values()]
    .map((player) => ({
      ...player,
      averageScore: player.runs > 0 ? player.averageScore / player.runs : 0,
      averageAccuracy: player.runs > 0 ? player.averageAccuracy / player.runs : 0,
    }))
    .sort((a, b) => b.bestScore - a.bestScore || b.runs - a.runs)
    .slice(0, 8);

  if (leaders.length === 0) {
    return <EmptyState title="No player leaders yet" body="Player breakdowns will show up once more runs are uploaded." />;
  }

  return (
    <ScrollArea className="max-h-[360px] overflow-auto rounded-[18px] border border-line bg-white/2">
      <table className="min-w-full text-left text-sm">
        <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
          <tr>
            <th className="px-4 py-3">Player</th>
            <th className="px-4 py-3">Runs</th>
            <th className="px-4 py-3">Best</th>
            <th className="px-4 py-3">Avg</th>
            <th className="px-4 py-3">Acc</th>
          </tr>
        </thead>
        <tbody>
          {leaders.map((player) => (
            <tr key={player.handle} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015]">
              <td className="px-4 py-3 text-text">
                <Link className="text-cyan underline underline-offset-3" to={`/profiles/${player.handle}`}>
                  {player.displayName || player.handle}
                </Link>
              </td>
              <td className="px-4 py-3 text-text">{player.runs}</td>
              <td className="px-4 py-3 text-gold">{Math.round(player.bestScore).toLocaleString()}</td>
              <td className="px-4 py-3 text-text">{Math.round(player.averageScore).toLocaleString()}</td>
              <td className="px-4 py-3 text-text">{player.averageAccuracy.toFixed(1)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
    </ScrollArea>
  );
}

function ScenarioRecentPulse({ page }: { page: GetScenarioPageResponse }) {
  const recent = [...page.recentRuns]
    .sort((a, b) => new Date(b.playedAtIso).getTime() - new Date(a.playedAtIso).getTime())
    .slice(0, 10);

  if (recent.length === 0) {
    return <EmptyState title="No recent activity yet" body="This scenario will show momentum once players start uploading runs." />;
  }

  return (
    <div className="grid gap-2">
      {recent.map((run) => (
        <div key={run.runId || run.sessionId} className="rounded-[16px] border border-line bg-white/2 px-3 py-2.5">
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <Link className="truncate text-cyan underline underline-offset-3" to={`/profiles/${run.userHandle || run.userDisplayName}`}>
                {run.userDisplayName || run.userHandle}
              </Link>
              <p className="mt-0.5 truncate text-[11px] text-muted">
                {formatRelativeTime(run.playedAtIso)} · {run.accuracy.toFixed(1)}% acc · {formatDurationMs(run.durationMs)}
              </p>
            </div>
            <div className="shrink-0 text-right">
              <div className="text-sm font-medium text-text">{Math.round(run.score).toLocaleString()}</div>
              <Link className="text-[11px] text-mint underline underline-offset-2" to={`/runs/${run.runId || run.sessionId}`}>
                Open run
              </Link>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

export function ScenarioPage() {
  const { slug = "" } = useParams();
  const [page, setPage] = useState<GetScenarioPageResponse | null>(null);
  const [replays, setReplays] = useState<HubSearchRun[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [sortField, setSortField] = useState<SortField>("date");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");
  const [tab, setTab] = useState<Tab>("recent");
  const [recentVisible, setRecentVisible] = useState(PAGE_SIZE);
  const [topVisible, setTopVisible] = useState(PAGE_SIZE);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setReplays([]);
    setError(null);
    setRecentVisible(PAGE_SIZE);
    setTopVisible(PAGE_SIZE);
    setTab("recent");
    void fetchScenarioPage(slug)
      .then((next) => {
        if (!cancelled) {
          setPage(next);
          void fetchReplayHub({ scenarioName: next.scenarioName, limit: 8 })
            .then((response) => {
              if (!cancelled) setReplays(response.items);
            })
            .catch(() => {});
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load this scenario.");
      });
    return () => {
      cancelled = true;
    };
  }, [slug]);

  const doRefresh = useCallback(() => {
    void fetchScenarioPage(slug)
      .then((next) => setPage(next))
      .catch(() => {});
  }, [slug]);
  useAutoRefresh(doRefresh, 60_000);

  function handleSort(field: string) {
    const f = field as SortField;
    if (f === sortField) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(f);
      setSortDir("desc");
    }
  }

  const metaTitle = page ? `${page.scenarioName} · AimMod Hub` : `${slug} · AimMod Hub`;
  const metaDesc = page
    ? `${page.runCount.toLocaleString()} runs · Best score ${Math.round(page.bestScore).toLocaleString()} · Avg accuracy ${page.averageAccuracy.toFixed(1)}%`
    : "Scenario page on AimMod Hub.";

  if (error) {
    return (
      <PageStack>
        <Helmet><title>{slug} · AimMod Hub</title></Helmet>
        <PageSection>
          <SectionHeader eyebrow="Scenario" title="Could not load this scenario" />
          <EmptyState title="Scenario not found" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!page) {
    return (
      <PageStack>
        <PageSection>
          <Skeleton className="mb-3 h-3 w-20" />
          <Skeleton className="mb-3 h-10 w-64" />
          <Skeleton className="mb-5 h-4 w-80" />
          <Grid className="grid-cols-[repeat(auto-fit,minmax(220px,1fr))]">
            {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-[100px]" />)}
          </Grid>
        </PageSection>
      </PageStack>
    );
  }

  const hasDistribution = page.scoreDistribution.length > 0 || page.recentRuns.length >= 3;
  const scoreRange =
    page.bestScore > 0
      ? `${Math.round(page.averageScore).toLocaleString()} avg · ${Math.round(page.bestScore).toLocaleString()} best`
      : null;

  const sortedRuns = [...page.recentRuns].sort((a, b) => {
    let diff = 0;
    if (sortField === "score") diff = a.score - b.score;
    else if (sortField === "accuracy") diff = a.accuracy - b.accuracy;
    else diff = new Date(a.playedAtIso).getTime() - new Date(b.playedAtIso).getTime();
    return sortDir === "asc" ? diff : -diff;
  });

  const visibleRecent = sortedRuns.slice(0, recentVisible);
  const hasMoreRecent = sortedRuns.length > recentVisible;
  const visibleTop = page.topRuns.slice(0, topVisible);
  const hasMoreTop = page.topRuns.length > topVisible;

  return (
    <PageStack>
      <Helmet>
        <title>{metaTitle}</title>
        <meta name="description" content={metaDesc} />
        <meta property="og:title" content={metaTitle} />
        <meta property="og:description" content={metaDesc} />
      </Helmet>
      <PageSection>
        <Breadcrumb crumbs={[{ label: "Community", to: "/community" }, { label: page.scenarioName }]} />
        <SectionHeader
          eyebrow="Scenario"
          title={page.scenarioName}
          body="A shared view of how players are performing on this scenario."
          aside={<ScenarioTypeBadge type={page.scenarioType} />}
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(150px,1fr))]">
          <StatCard label="Runs" value={page.runCount.toLocaleString()} detail="Saved runs for this scenario" />
          <StatCard
            label="Best score"
            value={Math.round(page.bestScore).toLocaleString()}
            detail="Best recorded result so far"
            accent="cyan"
          />
          <StatCard
            label="Average score"
            value={Math.round(page.averageScore).toLocaleString()}
            detail="Average result across all runs"
            accent="gold"
          />
          <StatCard
            label="Average accuracy"
            value={`${page.averageAccuracy.toFixed(1)}%`}
            detail={`Typical duration ${formatDurationMs(page.averageDurationMs)}`}
            accent="violet"
          />
        </Grid>
      </PageSection>

      {hasDistribution && (
        <PageSection>
          <SectionHeader
            eyebrow="Score distribution"
            title="How scores are spread"
            body={
              scoreRange
                ? `${scoreRange}. Highlighted bar shows where the average falls.`
                : "Score spread across all recorded runs. Highlighted bar shows where the average falls."
            }
          />
          {page.scoreDistribution.length > 0
            ? <ScoreDistributionChart bins={page.scoreDistribution} />
            : <ScoreDistributionChart runs={page.recentRuns} />
          }
        </PageSection>
      )}

      <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
        <PageSection>
          {/* Tab bar */}
          <div className="mb-[18px] flex items-center gap-1 border-b border-line pb-px">
            {(["recent", "leaderboard"] as Tab[]).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className={[
                  "rounded-t-[10px] px-4 py-2 text-[11px] uppercase tracking-[0.08em] transition-colors",
                  tab === t
                    ? "border-b-2 border-mint -mb-px text-mint"
                    : "text-muted hover:text-text",
                ].join(" ")}
              >
                {t === "recent" ? "Recent runs" : `Leaderboard${page.topRuns.length > 0 ? ` · ${page.topRuns.length}` : ""}`}
              </button>
            ))}
          </div>

          {tab === "recent" ? (
            page.recentRuns.length > 0 ? (
              <>
                <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
                  <table className="min-w-full text-left text-sm">
                    <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                      <tr>
                        <th className="px-4 py-3">Player</th>
                        <SortableTh label="Score" field="score" sortField={sortField} sortDir={sortDir} onSort={handleSort} />
                        <SortableTh label="Acc" field="accuracy" sortField={sortField} sortDir={sortDir} onSort={handleSort} />
                        <SortableTh label="When" field="date" sortField={sortField} sortDir={sortDir} onSort={handleSort} />
                        <th className="px-4 py-3">Run</th>
                        <th className="px-4 py-3">History</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleRecent.map((run) => {
                        const isBest = Math.round(run.score) === Math.round(page.bestScore);
                        const handle = run.userHandle || run.userDisplayName;
                        return (
                          <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                            <td className="px-4 py-3 text-text">
                              <Link className="text-cyan underline underline-offset-3" to={`/profiles/${handle}`}>
                                {run.userDisplayName || run.userHandle}
                              </Link>
                            </td>
                            <td className="px-4 py-3">
                              <span className={isBest ? "text-mint font-medium" : "text-text"}>
                                {Math.round(run.score).toLocaleString()}
                              </span>
                              {isBest && <span className="ml-2 text-[10px] text-mint/70 uppercase tracking-wider">best</span>}
                            </td>
                            <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                            <td className="px-4 py-3 text-muted" title={new Date(run.playedAtIso).toLocaleString()}>
                              {formatRelativeTime(run.playedAtIso)}
                            </td>
                            <td className="px-4 py-3 text-text">
                              <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.runId || run.sessionId}`}>
                                Open
                              </Link>
                            </td>
                            <td className="px-4 py-3">
                              <Link className="text-violet underline underline-offset-3 text-[12px]" to={`/profiles/${handle}/scenarios/${slug}`}>
                                History
                              </Link>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </ScrollArea>
                {hasMoreRecent && (
                  <button
                    onClick={() => setRecentVisible((n) => n + PAGE_SIZE)}
                    className="mt-3 w-full rounded-[14px] border border-line py-2.5 text-sm text-muted transition-colors hover:border-cyan/30 hover:text-text"
                  >
                    Load {Math.min(PAGE_SIZE, sortedRuns.length - recentVisible)} more runs
                  </button>
                )}
              </>
            ) : (
              <EmptyState title="No runs yet" body="This scenario has not received any uploaded runs yet." />
            )
          ) : (
            page.topRuns.length > 0 ? (
              <>
                <ScrollArea className="max-h-[min(64vh,820px)] overflow-auto rounded-[18px] border border-line bg-white/2">
                  <table className="min-w-full text-left text-sm">
                    <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
                      <tr>
                        <th className="px-4 py-3">#</th>
                        <th className="px-4 py-3">Player</th>
                        <th className="px-4 py-3">Score</th>
                        <th className="px-4 py-3">Acc</th>
                        <th className="px-4 py-3">When</th>
                        <th className="px-4 py-3">Run</th>
                        <th className="px-4 py-3">History</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleTop.map((run, idx) => {
                        const rank = idx + 1;
                        const rankColor = rank === 1 ? "text-gold" : rank <= 3 ? "text-cyan" : "text-muted-2";
                        const handle = run.userHandle || run.userDisplayName;
                        return (
                          <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                            <td className={`px-4 py-3 font-medium tabular-nums ${rankColor}`}>{rank}</td>
                            <td className="px-4 py-3 text-text">
                              <Link className="text-cyan underline underline-offset-3" to={`/profiles/${handle}`}>
                                {run.userDisplayName || run.userHandle}
                              </Link>
                            </td>
                            <td className={`px-4 py-3 font-medium ${rank === 1 ? "text-gold" : "text-text"}`}>
                              {Math.round(run.score).toLocaleString()}
                            </td>
                            <td className="px-4 py-3 text-text">{run.accuracy.toFixed(1)}%</td>
                            <td className="px-4 py-3 text-muted" title={new Date(run.playedAtIso).toLocaleString()}>
                              {formatRelativeTime(run.playedAtIso)}
                            </td>
                            <td className="px-4 py-3 text-text">
                              <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.runId || run.sessionId}`}>
                                Open
                              </Link>
                            </td>
                            <td className="px-4 py-3">
                              <Link className="text-violet underline underline-offset-3 text-[12px]" to={`/profiles/${handle}/scenarios/${slug}`}>
                                History
                              </Link>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </ScrollArea>
                {hasMoreTop && (
                  <button
                    onClick={() => setTopVisible((n) => n + PAGE_SIZE)}
                    className="mt-3 w-full rounded-[14px] border border-line py-2.5 text-sm text-muted transition-colors hover:border-cyan/30 hover:text-text"
                  >
                    Load {Math.min(PAGE_SIZE, page.topRuns.length - topVisible)} more
                  </button>
                )}
              </>
            ) : (
              <EmptyState title="No leaderboard yet" body="Scores will appear here once runs are uploaded." />
            )
          )}
        </PageSection>

        <PageSection>
          <SectionHeader eyebrow="Scenario intelligence" title="At a glance" />
          <ScenarioIntelligence page={page} />
        </PageSection>
      </Grid>

      <Grid className="grid-cols-2 max-[1180px]:grid-cols-1">
        <PageSection>
          <SectionHeader
            eyebrow="Top players"
            title="Who currently owns this scenario"
            body="A compact player view built from the best and most recent uploaded runs."
          />
          <ScenarioPlayerLeaders page={page} />
        </PageSection>

        <PageSection>
          <SectionHeader
            eyebrow="Recent pulse"
            title="What just happened here"
            body="The newest uploaded runs on this scenario, sorted by when they were played."
          />
          <ScenarioRecentPulse page={page} />
        </PageSection>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="Replay library"
          title="Replay-ready runs on this scenario"
          body="Open watchable runs and mouse-path captures without hunting through every result one by one."
        />
        {replays.length > 0 ? (
          <ScrollArea className="max-h-[min(68vh,860px)] pr-2">
            <div className="grid gap-3">
              {replays.map((run) => (
                <ReplayResultCard key={`scenario-replay:${run.publicRunID || run.sessionID}`} run={run} />
              ))}
            </div>
          </ScrollArea>
        ) : (
          <EmptyState title="No replay-ready runs yet" body="Replay-enabled runs for this scenario will show up here once they are uploaded." />
        )}
      </PageSection>
    </PageStack>
  );
}
