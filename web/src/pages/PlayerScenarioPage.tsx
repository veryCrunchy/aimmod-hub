import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetPlayerScenarioHistoryResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { GetPlayerScenarioHistoryRequest } from "../gen/aimmod/hub/v1/hub_pb";
import { ProgressChart } from "../components/charts/ProgressChart";
import { ScenarioBenchmarkRankList } from "../components/BenchmarkCards";
import { SectionHeader } from "../components/SectionHeader";
import { StatCard } from "../components/StatCard";
import { Breadcrumb } from "../components/ui/Breadcrumb";
import { Button } from "../components/ui/Button";
import { PageHeader } from "../components/ui/PageHeader";
import { EmptyState } from "../components/ui/EmptyState";
import { PageSection } from "../components/ui/PageSection";
import { ScrollArea } from "../components/ui/ScrollArea";
import { PageSkeleton } from "../components/ui/Skeleton";
import { SortableTh } from "../components/ui/SortableTh";
import { Grid, PageStack } from "../components/ui/Stack";
import { Helmet } from "../lib/helmet";
import { displayScenarioType, hubClient, formatDurationMs, formatRelativeTime } from "../lib/api";
import { formatAccuracy, formatChange, formatScore, scoreTrend } from "../lib/kovaaksStats";

type SortField = "score" | "accuracy" | "date";

function fetchHistory(handle: string, slug: string) {
  return hubClient.getPlayerScenarioHistory(
    new GetPlayerScenarioHistoryRequest({ handle, scenarioSlug: slug })
  );
}

export function PlayerScenarioPage() {
  const { handle = "", slug = "" } = useParams();
  const [history, setHistory] = useState<GetPlayerScenarioHistoryResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sortField, setSortField] = useState<SortField>("date");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  useEffect(() => {
    let cancelled = false;
    setHistory(null);
    setError(null);
    void fetchHistory(handle, slug)
      .then((next) => { if (!cancelled) setHistory(next); })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Could not load history.");
      });
    return () => { cancelled = true; };
  }, [handle, slug]);

  function handleSort(field: string) {
    const f = field as SortField;
    if (f === sortField) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(f);
      setSortDir(f === "date" ? "desc" : "desc");
    }
  }

  if (error) {
    return (
      <PageStack>
        <PageHeader title="History not found" />
        <EmptyState title="This player or scenario could not be found.">
          <Button to={`/profiles/${handle}`}>Open profile</Button>
        </EmptyState>
      </PageStack>
    );
  }

  if (!history) {
    return <PageStack><PageSkeleton label="Loading history" /></PageStack>;
  }

  if (history.runCount === 0) {
    return (
      <PageStack>
        <PageSection>
          <Breadcrumb crumbs={[
            { label: `@${handle}`, to: `/profiles/${handle}` },
            { label: history.scenarioName || slug },
          ]} />
          <PageHeader title={history.scenarioName || slug} />
          <EmptyState title={`@${handle} has no runs on this scenario yet.`}>
            <Button to={`/scenarios/${slug}`}>Scenario leaderboard</Button>
          </EmptyState>
        </PageSection>
      </PageStack>
    );
  }

  const trend = scoreTrend(history.runs);
  const playerName = history.runs[0]?.userDisplayName || handle;

  const sortedRuns = [...history.runs].sort((a, b) => {
    let diff = 0;
    if (sortField === "score") diff = a.score - b.score;
    else if (sortField === "accuracy") diff = a.accuracy - b.accuracy;
    else diff = new Date(a.playedAtIso).getTime() - new Date(b.playedAtIso).getTime();
    return sortDir === "asc" ? diff : -diff;
  });

  // chronological for chart (runs are already oldest-first from API)
  const chartRuns = [...history.runs].sort(
    (a, b) => new Date(a.playedAtIso).getTime() - new Date(b.playedAtIso).getTime()
  );

  const firstRun = history.runs.reduce((a, b) =>
    new Date(a.playedAtIso) < new Date(b.playedAtIso) ? a : b
  );
  const latestRun = history.runs.reduce((a, b) =>
    new Date(a.playedAtIso) > new Date(b.playedAtIso) ? a : b
  );
  const bestRun = history.runs.reduce((a, b) => (a.score > b.score ? a : b));
  const type = displayScenarioType(history.scenarioType);

  return (
    <PageStack>
      <Helmet><title>{`${history.scenarioName} · ${playerName} · AimMod Hub`}</title></Helmet>
      <PageHeader
        before={<Breadcrumb crumbs={[{ label: playerName, to: `/profiles/${handle}` }, { label: history.scenarioName }]} />}
        title={history.scenarioName}
        meta={[`${playerName}'s runs`, type, `${history.runCount.toLocaleString()} runs since ${new Date(firstRun.playedAtIso).toLocaleDateString()}`].filter(Boolean).join(" · ")}
        actions={<Button to={`/scenarios/${slug}`}>Scenario leaderboard</Button>}
      />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label="Best score" value={<Link to={`/runs/${bestRun.runId || bestRun.sessionId}`} className="hover:text-cyan">{formatScore(history.bestScore)}</Link>} accent="gold" detail={`Set ${formatRelativeTime(bestRun.playedAtIso)}`} />
        <StatCard label="Recent average" value={formatScore(trend ? trend.recent : history.averageScore)} detail={trend ? `${formatChange(trend.changePct)} vs the runs before` : "All runs"} accent={trend && trend.changePct > 0.5 ? "mint" : "text"} />
        <StatCard label="Average accuracy" value={formatAccuracy(history.averageAccuracy)} detail={`Best ${formatAccuracy(history.bestAccuracy)}`} />
        <StatCard label="Latest run" value={<Link to={`/runs/${latestRun.runId || latestRun.sessionId}`} className="hover:text-cyan">{formatScore(latestRun.score)}</Link>} detail={formatRelativeTime(latestRun.playedAtIso)} />
      </div>

      {history.benchmarkRanks.length > 0 && (
        <PageSection>
          <SectionHeader title="Benchmark rank" />
          <ScenarioBenchmarkRankList title="Ranks" ranks={history.benchmarkRanks} handle={handle} />
        </PageSection>
      )}

      {chartRuns.length >= 2 && (
        <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
          <PageSection>
            <SectionHeader title="Score" />
            <ProgressChart runs={chartRuns} showScore showAccuracy={false} />
          </PageSection>

          <PageSection>
            <SectionHeader title="Accuracy" />
            <ProgressChart runs={chartRuns} showScore={false} showAccuracy />
          </PageSection>
        </Grid>
      )}

      <PageSection>
        <SectionHeader title="All runs" />
        <ScrollArea className="max-h-[min(72vh,900px)] overflow-auto rounded-[18px] border border-line bg-white/2">
          <table className="min-w-full text-left text-sm">
            <thead className="sticky top-0 z-10 border-b border-line bg-[rgba(4,12,9,0.97)] text-[11px] uppercase tracking-[0.08em] text-muted">
              <tr>
                <th className="px-4 py-3">#</th>
                <SortableTh label="Score" field="score" sortField={sortField} sortDir={sortDir} onSort={handleSort} />
                <SortableTh label="Acc" field="accuracy" sortField={sortField} sortDir={sortDir} onSort={handleSort} />
                <th className="px-4 py-3">Duration</th>
                <SortableTh label="When" field="date" sortField={sortField} sortDir={sortDir} onSort={handleSort} />
                <th className="px-4 py-3">Run</th>
              </tr>
            </thead>
            <tbody>
              {sortedRuns.map((run, idx) => {
                const isBest = Math.round(run.score) === Math.round(history.bestScore);
                return (
                  <tr key={run.runId || run.sessionId} className="border-b border-white/6 last:border-b-0 hover:bg-white/[0.015] transition-colors">
                    <td className="px-4 py-3 tabular-nums text-muted-2">{idx + 1}</td>
                    <td className="px-4 py-3 font-medium tabular-nums">
                      <span className={isBest ? "text-gold" : "text-text"}>
                        {Math.round(run.score).toLocaleString()}
                      </span>
                      {isBest && <span className="ml-2 text-[10px] text-gold/60 uppercase tracking-wider">best</span>}
                    </td>
                    <td className="px-4 py-3 text-text tabular-nums">{run.accuracy.toFixed(1)}%</td>
                    <td className="px-4 py-3 text-muted">{formatDurationMs(run.durationMs)}</td>
                    <td className="px-4 py-3 text-muted" title={new Date(run.playedAtIso).toLocaleString()}>
                      {formatRelativeTime(run.playedAtIso)}
                    </td>
                    <td className="px-4 py-3">
                      <Link className="text-cyan underline underline-offset-3" to={`/runs/${run.runId || run.sessionId}`}>
                        Open
                      </Link>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </ScrollArea>
      </PageSection>
    </PageStack>
  );
}
