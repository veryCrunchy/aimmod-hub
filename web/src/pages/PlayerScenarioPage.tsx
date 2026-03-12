import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { GetPlayerScenarioHistoryResponse } from "../gen/aimmod/hub/v1/hub_pb";
import { GetPlayerScenarioHistoryRequest } from "../gen/aimmod/hub/v1/hub_pb";
import { ProgressChart } from "../components/charts/ProgressChart";
import { ScenarioBenchmarkRankList } from "../components/BenchmarkCards";
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
import { hubClient, formatDurationMs, formatRelativeTime } from "../lib/api";

type SortField = "score" | "accuracy" | "date";

function fetchHistory(handle: string, slug: string) {
  return hubClient.getPlayerScenarioHistory(
    new GetPlayerScenarioHistoryRequest({ handle, scenarioSlug: slug })
  );
}

function improvementRate(scores: number[]): number | null {
  if (scores.length < 4) return null;
  const half = Math.floor(scores.length / 2);
  const firstHalfAvg = scores.slice(0, half).reduce((a, b) => a + b, 0) / half;
  const secondHalfAvg = scores.slice(-half).reduce((a, b) => a + b, 0) / half;
  return firstHalfAvg > 0 ? ((secondHalfAvg - firstHalfAvg) / firstHalfAvg) * 100 : null;
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
        <PageSection>
          <SectionHeader eyebrow="History" title="Could not load history" />
          <EmptyState title="Not found" body={error} />
        </PageSection>
      </PageStack>
    );
  }

  if (!history) {
    return (
      <PageStack>
        <PageSection>
          <Skeleton className="mb-3 h-3 w-24" />
          <Skeleton className="mb-3 h-10 w-64" />
          <Skeleton className="mb-5 h-4 w-80" />
          <Grid className="grid-cols-[repeat(auto-fit,minmax(200px,1fr))]">
            {[0, 1, 2, 3].map((i) => <Skeleton key={i} className="h-[100px]" />)}
          </Grid>
        </PageSection>
      </PageStack>
    );
  }

  if (history.runCount === 0) {
    return (
      <PageStack>
        <PageSection>
          <Breadcrumb crumbs={[
            { label: `@${handle}`, to: `/profiles/${handle}` },
            { label: history.scenarioName || slug },
          ]} />
          <SectionHeader eyebrow="History" title="No runs found" />
          <EmptyState title="No runs" body={`@${handle} has not played this scenario yet.`} />
        </PageSection>
      </PageStack>
    );
  }

  const scores = history.runs.map((r) => r.score);
  const improvement = improvementRate(scores);

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

  return (
    <PageStack>
      <PageSection>
        <Breadcrumb crumbs={[
          { label: `@${handle}`, to: `/profiles/${handle}` },
          { label: history.scenarioName, to: `/scenarios/${slug}` },
        ]} />
        <SectionHeader
          eyebrow="Player history"
          title={history.scenarioName}
          body={`All ${history.runCount.toLocaleString()} runs @${handle} has played on this scenario.`}
          aside={<ScenarioTypeBadge type={history.scenarioType} />}
        />
        <Grid className="grid-cols-[repeat(auto-fit,minmax(200px,1fr))]">
          <StatCard label="Runs" value={history.runCount.toLocaleString()} detail="Total sessions played" />
          <StatCard
            label="Best score"
            value={Math.round(history.bestScore).toLocaleString()}
            detail={`First run ${formatRelativeTime(firstRun.playedAtIso)}`}
            accent="gold"
          />
          <StatCard
            label="Average score"
            value={Math.round(history.averageScore).toLocaleString()}
            detail="Mean across all runs"
            accent="cyan"
          />
          <StatCard
            label="Best accuracy"
            value={`${history.bestAccuracy.toFixed(1)}%`}
            detail={`Avg ${history.averageAccuracy.toFixed(1)}% accuracy`}
            accent="violet"
          />
        </Grid>
      </PageSection>

      {history.benchmarkRanks.length > 0 && (
        <PageSection>
          <SectionHeader
            eyebrow="Benchmark ranks"
            title="This player's rank on this scenario"
            body="These benchmark systems already include this scenario and score."
          />
          <ScenarioBenchmarkRankList title="Ranks" ranks={history.benchmarkRanks} handle={handle} />
        </PageSection>
      )}

      {chartRuns.length >= 2 && (
        <Grid className="grid-cols-2 max-[1100px]:grid-cols-1">
          <PageSection>
            <SectionHeader
              eyebrow="Score trend"
              title="Score over time"
              body="Each dot is one run, oldest to newest."
            />
            <ProgressChart runs={chartRuns} showScore showAccuracy={false} />
            {improvement !== null && (
              <p className="mt-3 text-[11px] text-muted">
                {improvement > 0
                  ? `Trending up +${improvement.toFixed(1)}% — second half avg above first half.`
                  : `Trending down ${improvement.toFixed(1)}% — second half avg below first half.`}
              </p>
            )}
          </PageSection>

          <PageSection>
            <SectionHeader
              eyebrow="Accuracy trend"
              title="Accuracy over time"
              body="Accuracy % across all runs, oldest to newest."
            />
            <ProgressChart runs={chartRuns} showScore={false} showAccuracy />
          </PageSection>
        </Grid>
      )}

      {/* milestone row: personal best, latest */}
      <Grid className="grid-cols-2 max-[900px]:grid-cols-1">
        <PageSection>
          <SectionHeader eyebrow="Personal best" title="Top run" />
          <Link
            to={`/runs/${bestRun.runId || bestRun.sessionId}`}
            className="block rounded-[18px] border border-gold/20 bg-[rgba(255,215,0,0.03)] p-[18px] transition-colors hover:border-gold/40"
          >
            <p className="text-[10px] uppercase tracking-[0.1em] text-gold/70">Best score</p>
            <p className="mt-2 text-3xl font-medium text-gold">{Math.round(bestRun.score).toLocaleString()}</p>
            <div className="mt-2 flex items-center gap-3 text-sm text-muted">
              <span>{bestRun.accuracy.toFixed(1)}% acc</span>
              <span>·</span>
              <span>{formatDurationMs(bestRun.durationMs)}</span>
              <span>·</span>
              <span>{formatRelativeTime(bestRun.playedAtIso)}</span>
            </div>
            <p className="mt-3 text-[11px] text-cyan underline underline-offset-3">Open run →</p>
          </Link>
        </PageSection>

        <PageSection>
          <SectionHeader eyebrow="Most recent" title="Latest run" />
          <Link
            to={`/runs/${latestRun.runId || latestRun.sessionId}`}
            className="block rounded-[18px] border border-cyan/18 bg-white/2 p-[18px] transition-colors hover:border-cyan/30"
          >
            <p className="text-[10px] uppercase tracking-[0.1em] text-cyan/70">Latest</p>
            <p className="mt-2 text-3xl font-medium text-text">{Math.round(latestRun.score).toLocaleString()}</p>
            <div className="mt-2 flex items-center gap-3 text-sm text-muted">
              <span>{latestRun.accuracy.toFixed(1)}% acc</span>
              <span>·</span>
              <span>{formatDurationMs(latestRun.durationMs)}</span>
              <span>·</span>
              <span>{formatRelativeTime(latestRun.playedAtIso)}</span>
            </div>
            <p className="mt-3 text-[11px] text-cyan underline underline-offset-3">Open run →</p>
          </Link>
        </PageSection>
      </Grid>

      <PageSection>
        <SectionHeader
          eyebrow="All runs"
          title={`${history.runCount.toLocaleString()} runs`}
          body={`Complete history from ${new Date(firstRun.playedAtIso).toLocaleDateString()} to ${new Date(latestRun.playedAtIso).toLocaleDateString()}.`}
        />
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
