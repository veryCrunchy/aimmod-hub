import { useCallback, useEffect, useMemo, useState } from "react";
import { Helmet } from "../lib/helmet";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { filterChoice, updateFilterQuery } from "../lib/savedPageFilters";
import type { GetScenarioPageResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { ReplayResultCard } from "../components/ReplayResultCard";
import { ScoreDistributionChart } from "../components/charts/ScoreDistributionChart";
import { RunTable } from "../components/RunTable";
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
import { displayScenarioType, fetchReplayHub, fetchScenarioPage, formatDurationMs, type HubSearchRun } from "../lib/api";
import { bestPerPlayer, formatAccuracy, formatScore, newestFirst } from "../lib/kovaaksStats";

type Tab = "leaderboard" | "top" | "recent";

export function ScenarioPage() {
  const { slug = "" } = useParams();
  const auth = useAuth();
  const [page, setPage] = useState<GetScenarioPageResponse | null>(null);
  const [replays, setReplays] = useState<HubSearchRun[]>([]);
  const [error, setError] = useState(false);
  const [params, setParams] = useSearchParams();
  const tab = filterChoice<Tab>(params.get("tab"), ["leaderboard", "top", "recent"], "leaderboard");
  const setTab = (tab: Tab) => setParams(current => updateFilterQuery(current, { tab }), { replace: true });

  useEffect(() => {
    let cancelled = false;
    setPage(null);
    setReplays([]);
    setError(false);
    void fetchScenarioPage(slug)
      .then((next) => {
        if (cancelled) return;
        setPage(next);
        void fetchReplayHub({ scenarioName: next.scenarioName, limit: 6 }).then((response) => { if (!cancelled) setReplays(response.items); }).catch(() => {});
      })
      .catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [slug]);

  const doRefresh = useCallback(() => {
    void fetchScenarioPage(slug).then(setPage).catch(() => {});
  }, [slug]);
  useAutoRefresh(doRefresh, 60_000);

  const players = useMemo(() => bestPerPlayer([...(page?.topRuns ?? []), ...(page?.recentRuns ?? [])]), [page]);
  const recent = useMemo(() => newestFirst(page?.recentRuns ?? []), [page]);

  if (error) {
    return (
      <PageStack>
        <Helmet><title>{slug} · AimMod Hub</title></Helmet>
        <PageHeader title="Scenario not found" />
        <EmptyState title="No runs have been shared for this scenario.">
          <Button to="/community">Browse scenarios</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!page) return <PageStack><PageSkeleton label="Loading scenario" /></PageStack>;

  const type = displayScenarioType(page.scenarioType);
  const leader = page.topRuns[0];
  const ownHandle = auth.user?.profileHandle;
  const ownBest = ownHandle ? players.findIndex((p) => p.handle.toLowerCase() === ownHandle.toLowerCase()) : -1;
  const metaTitle = `${page.scenarioName} · AimMod Hub`;
  const metaDesc = `${page.runCount.toLocaleString()} runs · Record ${formatScore(page.bestScore)} · Average accuracy ${page.averageAccuracy.toFixed(1)}%`;
  const playerCount = page.playerCount || players.length;

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
        meta={[type, `${page.runCount.toLocaleString()} runs`, `${playerCount.toLocaleString()} ${playerCount === 1 ? "player" : "players"}`, `${page.runsLast7Days.toLocaleString()} this week`].filter(Boolean).join(" · ")}
        actions={ownHandle ? <Button to={`/profiles/${ownHandle}/scenarios/${page.scenarioSlug || slug}`}>My progress</Button> : null}
      />

      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label="Record" value={formatScore(page.bestScore)} accent="gold"
          detail={leader ? <Link to={`/profiles/${leader.userHandle}`} className="hover:text-cyan">{leader.userDisplayName || leader.userHandle}</Link> : undefined} />
        <StatCard label="Average score" value={formatScore(page.averageScore)} detail={page.bestScore > 0 ? `${Math.round((page.averageScore / page.bestScore) * 100)}% of the record` : undefined} />
        <StatCard label="Average accuracy" value={formatAccuracy(page.averageAccuracy)} />
        <StatCard label="Run length" value={formatDurationMs(page.averageDurationMs)} detail="Average" />
      </div>

      {ownBest >= 0 && (
        <p className="rounded-md border border-line bg-panel px-4 py-2.5 text-sm">
          Your best is <strong className="tabular-nums">{formatScore(players[ownBest].best.score)}</strong>, rank <strong>{ownBest + 1}</strong> of {players.length} players here.
        </p>
      )}

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <div className="min-w-0">
          <Tabs label="Scenario runs" value={tab} onChange={setTab} className="mb-3"
            tabs={[["leaderboard", "Best per player"], ["top", "Top runs"], ["recent", "Recent runs"]]} />
          {tab === "leaderboard" ? (
            players.length ? (
              <RunTable runs={players.map((p) => p.best)} scenario={false} ranked caption="Best score per player"
                playerHref={(run) => `/profiles/${run.userHandle || run.userDisplayName}/scenarios/${page.scenarioSlug || slug}`} />
            ) : <EmptyState title="No runs yet." />
          ) : tab === "top" ? (
            page.topRuns.length ? <RunTable runs={page.topRuns} scenario={false} ranked caption="Top runs" /> : <EmptyState title="No runs yet." />
          ) : (
            recent.length ? <RunTable runs={recent} scenario={false} bestScore={page.bestScore} caption="Recent runs" /> : <EmptyState title="No runs yet." />
          )}
          {tab === "leaderboard" && players.length > 0 && <p className="mt-2 text-xs text-muted-2">Built from the top and most recent {page.topRuns.length + page.recentRuns.length} runs.</p>}
        </div>

        {(page.scoreDistribution.length > 0 || page.recentRuns.length >= 3) && (
          <Section title="Score spread" aside="Highlighted bar: average">
            {page.scoreDistribution.length > 0
              ? <ScoreDistributionChart bins={page.scoreDistribution} />
              : <ScoreDistributionChart runs={page.recentRuns} />}
          </Section>
        )}
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
