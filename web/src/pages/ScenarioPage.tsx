import { useCallback, useEffect, useMemo, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useParams } from "react-router-dom";
import type { GetScenarioPageResponse, ScenarioBenchmarkMembership } from "../gen/aimmod/hub/v1/hub_pb";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { ScoreDistributionChart } from "../components/charts/ScoreDistributionChart";
import { RankBadge } from "../components/RankBadge";
import { RunTable } from "../components/RunTable";
import { ScenarioLeaderboardPanel, leaderboardChoices, leaderboardDefaults } from "../components/ScenarioLeaderboardPanel";
import { StatCard } from "../components/StatCard";
import { Button } from "../components/ui/Button";
import { EmptyState } from "../components/ui/EmptyState";
import { PageHeader } from "../components/ui/PageHeader";
import { Section } from "../components/ui/Section";
import { PageSkeleton } from "../components/ui/Skeleton";
import { Tabs } from "../components/ui/Tabs";
import { PageStack } from "../components/ui/Stack";
import { useAutoRefresh } from "../hooks/useAutoRefresh";
import { useAuth } from "../lib/AuthContext";
import { displayScenarioType, fetchReplayHub, fetchScenarioPage, type HubSearchRun } from "../lib/api";
import { rankColor, rankProgress, compactScore } from "../lib/benchmarkSheet";
import { formatScore, newestFirst } from "../lib/kovaaksStats";
import { useUrlState } from "../lib/urlState";

const tabs = ["aimmod", "kovaaks", "top", "recent"] as const;
type Tab = (typeof tabs)[number];

const pageDefaults = { ...leaderboardDefaults, tab: "aimmod", name: "", player: "" };
const pageChoices = { ...leaderboardChoices, tab: [...tabs, "leaderboard"] };

/** Where a best score sits among AimMod players' bests. */
function PercentileStrip({ page }: { page: GetScenarioPageResponse }) {
  const points = page.percentiles;
  if (points.length < 2) return null;
  const lo = points[0].score;
  const hi = points[points.length - 1].score;
  const span = Math.max(1, hi - lo);
  const pos = (score: number) => `${Math.max(0, Math.min(100, ((score - lo) / span) * 100))}%`;
  const viewer = page.viewer;
  return (
    <div className="rounded-md border border-line bg-panel px-4 py-3">
      <div className="flex flex-wrap items-baseline justify-between gap-2 text-sm">
        <span className="font-medium">Where players land</span>
        <span className="text-xs text-muted-2">Best score per AimMod player</span>
      </div>
      <div className="relative mt-6 h-2 rounded-full bg-gradient-to-r from-white/5 via-cyan/30 to-mint/60">
        {points.map((p) => (
          <span key={p.percentile} className="absolute top-1/2 h-3 w-px -translate-y-1/2 bg-white/40" style={{ left: pos(p.score) }} />
        ))}
        {viewer ? (
          <span className="absolute top-1/2 -translate-x-1/2 -translate-y-1/2" style={{ left: pos(viewer.bestScore) }}>
            <span className="block h-4 w-4 rounded-full border-2 border-bg bg-gold" />
            <span className="absolute bottom-5 left-1/2 -translate-x-1/2 whitespace-nowrap rounded bg-gold px-1.5 text-[11px] font-semibold text-black">You</span>
          </span>
        ) : null}
      </div>
      <ol className="mt-2 grid grid-cols-3 gap-y-1 text-xs sm:grid-cols-6">
        {points.map((p) => (
          <li key={p.percentile} className="tabular-nums">
            <span className="text-muted-2">p{p.percentile} </span>
            <span>{compactScore(Math.round(p.score))}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}

function BenchmarkMemberships({ items, best }: { items: readonly ScenarioBenchmarkMembership[]; best?: number }) {
  if (!items.length) return null;
  return (
    <Section title="In benchmarks" aside={`${items.length} ${items.length === 1 ? "benchmark" : "benchmarks"}`}>
      <ul className="grid gap-2">
        {items.map((item) => {
          const reached = best ? rankProgress(best, item.thresholds) : null;
          return (
            <li key={item.benchmarkId} className="rounded-md border border-line bg-panel px-3 py-2.5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <Link to={`/benchmarks/${item.benchmarkId}`} className="min-w-0 truncate text-sm font-medium hover:text-cyan">{item.benchmarkName}</Link>
                {reached ? <RankBadge rankName={reached.current?.rankName} iconUrl={reached.current?.iconUrl} color={reached.current?.color} rankIndex={reached.current?.rankIndex} emptyLabel="Below first rank" /> : null}
              </div>
              <div className="text-xs text-muted-2">{item.categoryName}</div>
              <ul className="mt-2 flex flex-wrap gap-1" aria-label="Rank thresholds">
                {item.thresholds.map((t) => {
                  const met = best !== undefined && best >= t.score;
                  const color = rankColor(t.color, t.rankIndex);
                  return (
                    <li key={t.rankIndex} className="rounded border px-1.5 py-0.5 text-[11px] tabular-nums" style={{ borderColor: `${color}66`, background: met ? `${color}33` : undefined }}>
                      <span style={{ color }}>{t.rankName}</span> {compactScore(t.score)}
                    </li>
                  );
                })}
              </ul>
            </li>
          );
        })}
      </ul>
    </Section>
  );
}

export function ScenarioPage() {
  const { slug = "" } = useParams();
  const auth = useAuth();
  const viewerHandle = auth.user?.profileHandle || undefined;
  const [page, setPage] = useState<GetScenarioPageResponse | null>(null);
  const [replays, setReplays] = useState<HubSearchRun[]>([]);
  const [missing, setMissing] = useState(false);
  const [state, setState] = useUrlState(pageDefaults, pageChoices);
  const tab: Tab = state.tab === "leaderboard" ? "aimmod" : (state.tab as Tab);

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setReplays([]);
    setMissing(false);
    void fetchScenarioPage(slug, viewerHandle ?? "")
      .then((next) => {
        if (cancelled) return;
        setPage(next);
        void fetchReplayHub({ scenarioName: next.scenarioName, limit: 6 }).then((response) => { if (!cancelled) setReplays(response.items); }).catch(() => {});
      })
      .catch(() => { if (!cancelled) setMissing(true); });
    return () => { cancelled = true; };
  }, [slug, viewerHandle]);

  const doRefresh = useCallback(() => {
    void fetchScenarioPage(slug, viewerHandle ?? "").then(setPage).catch(() => {});
  }, [slug, viewerHandle]);
  useAutoRefresh(doRefresh, 60_000);
  const recent = useMemo(() => newestFirst(page?.recentRuns ?? []), [page]);

  // Scenarios nobody uploaded to AimMod still have KovaaK's global board.
  if (missing) {
    if (!state.name) {
      return (
        <PageStack>
          <Helmet><title>{`${slug} · AimMod Hub`}</title></Helmet>
          <PageHeader title="Scenario not found" />
          <EmptyState title="No runs have been shared for this scenario.">
            <Button to="/community">Browse scenarios</Button>
          </EmptyState>
        </PageStack>
      );
    }
    return (
      <PageStack>
        <Helmet><title>{`${state.name} · KovaaK's leaderboard · AimMod Hub`}</title></Helmet>
        <PageHeader title={state.name} meta="KovaaK's scenario · nobody has uploaded it through AimMod yet" />
        <Section title="KovaaK's global leaderboard">
          <ScenarioLeaderboardPanel source="kovaaks" scenarioName={state.name} state={state} setState={setState} viewerHandle={viewerHandle} viewerSteamId={state.player || undefined} />
        </Section>
      </PageStack>
    );
  }

  if (!page) return <PageStack><PageSkeleton label="Loading scenario" /></PageStack>;

  const type = displayScenarioType(page.scenarioType);
  const leader = page.topRuns[0];
  const median = page.percentiles.find((p) => p.percentile === 50);
  const metaTitle = `${page.scenarioName} · AimMod Hub`;
  const metaDesc = `${page.runCount.toLocaleString()} runs · Record ${formatScore(page.bestScore)} · Average accuracy ${page.averageAccuracy.toFixed(1)}%`;
  const viewer = page.viewer;
  const tabList: (readonly [Tab, string])[] = [["aimmod", "AimMod players"], ...(page.kovaaksLeaderboardId ? [["kovaaks", "KovaaK's global"] as const] : []), ["top", "Top runs"], ["recent", "Recent runs"]];

  return (
    <PageStack>
      <Helmet>
        <title>{metaTitle}</title>
        <meta name="description" content={metaDesc} />
        <meta property="og:title" content={metaTitle} />
        <meta property="og:description" content={metaDesc} />
      </Helmet>

      <PageHeader
        title={page.scenarioName}
        meta={[type, `${page.runCount.toLocaleString()} runs`, `${page.playerCount.toLocaleString()} ${page.playerCount === 1 ? "player" : "players"}`, `${page.runsLast7Days.toLocaleString()} this week`].filter(Boolean).join(" · ")}
        actions={viewerHandle && viewer ? <Button to={`/profiles/${viewerHandle}/scenarios/${page.scenarioSlug || slug}`}>My progress</Button> : null}
      />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label="AimMod record" value={formatScore(page.bestScore)} accent="gold"
          detail={leader ? <Link to={`/profiles/${leader.userHandle}`} className="hover:text-cyan">{leader.userDisplayName || leader.userHandle}</Link> : undefined} />
        <StatCard label="Median best" value={median ? formatScore(median.score) : formatScore(page.averageScore)} detail={median ? "Half of players score above this" : "Average score"} />
        <StatCard label="Your standing" value={viewer ? `#${viewer.rank}` : "–"}
          detail={viewer ? `Best ${formatScore(viewer.bestScore)} · better than ${viewer.percentile}% of ${page.playerCount} players` : viewerHandle ? "No runs on this scenario yet" : "Sign in to see where you rank"} accent={viewer ? "mint" : "text"} />
        <StatCard label="On KovaaK's" value={page.kovaaksEntries ? Number(page.kovaaksEntries).toLocaleString() : "–"} detail={page.kovaaksEntries ? `players · ${Number(page.kovaaksPlays).toLocaleString()} plays` : "No public board found"} />
      </div>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <div className="min-w-0">
          <Tabs label="Scenario leaderboards" value={tab} onChange={(next) => setState({ tab: next, page: "1" })} className="mb-3" tabs={tabList} />
          {tab === "aimmod" || tab === "kovaaks" ? (
            <ScenarioLeaderboardPanel key={tab} source={tab} scenarioSlug={page.scenarioSlug || slug} scenarioName={page.scenarioName}
              leaderboardId={tab === "kovaaks" ? page.kovaaksLeaderboardId : undefined}
              state={state} setState={setState} viewerHandle={viewerHandle} viewerSteamId={state.player || undefined} />
          ) : tab === "top" ? (
            page.topRuns.length ? <RunTable runs={page.topRuns} scenario={false} ranked caption="Top runs" /> : <EmptyState title="No runs yet." />
          ) : (
            recent.length ? <RunTable runs={recent} scenario={false} bestScore={page.bestScore} caption="Recent runs" /> : <EmptyState title="No runs yet." />
          )}
        </div>

        <div className="grid min-w-0 gap-6">
          <PercentileStrip page={page} />
          {(page.scoreDistribution.length > 0 || page.recentRuns.length >= 3) && (
            <Section title="Score spread" aside="All AimMod runs">
              {page.scoreDistribution.length > 0
                ? <ScoreDistributionChart bins={page.scoreDistribution} />
                : <ScoreDistributionChart runs={page.recentRuns} />}
            </Section>
          )}
          <BenchmarkMemberships items={page.benchmarks} best={viewer?.bestScore} />
        </div>
      </div>

      {replays.length > 0 && (
        <Section title="Replays" aside={<Link to={`/replays?q=${encodeURIComponent(page.scenarioName)}`} className="text-cyan hover:underline">All replays</Link>}>
          <div className="grid gap-3 lg:grid-cols-2">
            {replays.map((run) => <ReplayResultCard key={`scenario-replay:${run.publicRunID || run.sessionID}`} run={run} />)}
          </div>
        </Section>
      )}
    </PageStack>
  );
}
